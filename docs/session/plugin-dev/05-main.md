# 05 — Main: Connect, Register, Serve

Two files: `main.go` (lifecycle) and `dispatch.go` (NATS → tool routing).

---

## main.go

Wires everything together. Connect → register → subscribe → heartbeat → wait for signal → deregister.

```go
func main() {
    token := os.Getenv("GITHUB_TOKEN")
    if token == "" {
        log.Fatal("[github-plugin] GITHUB_TOKEN is required")
    }
    natsURL := os.Getenv("NATS_URL")
    if natsURL == "" {
        natsURL = "nats://127.0.0.1:4222"
    }

    nc, err := nats.Connect(natsURL,
        nats.RetryOnFailedConnect(true),
        nats.MaxReconnects(-1),
        nats.ReconnectWait(2*time.Second),
    )
    if err != nil {
        log.Fatalf("[github-plugin] nats connect: %v", err)
    }
    defer nc.Close()
    log.Printf("[github-plugin] connected to %s", natsURL)

    gh := newGitHubClient(token)

    if err := register(nc); err != nil {
        log.Fatalf("[github-plugin] registration failed: %v", err)
    }

    // Queue group = plugin name — only one instance handles each call.
    nc.QueueSubscribe("tool.call.github.*", "github", func(msg *nats.Msg) {
        go dispatch(gh, msg)
    })

    go heartbeat(nc)

    log.Printf("[github-plugin] ready")

    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig

    nc.Publish(plugin.SubjectDeregister, []byte(`{"name":"github"}`))
    nc.Flush()
    log.Printf("[github-plugin] deregistered, exiting")
}

func register(nc *nats.Conn) error {
    payload, _ := json.Marshal(pluginManifest)
    msg, err := nc.Request(plugin.SubjectRegister, payload, 10*time.Second)
    if err != nil {
        return fmt.Errorf("nats request: %w", err)
    }
    var reply plugin.RegisterResponse
    if err := json.Unmarshal(msg.Data, &reply); err != nil {
        return fmt.Errorf("parse reply: %w", err)
    }
    if reply.Status != "ok" {
        return fmt.Errorf("bizzy rejected: %s", reply.Error)
    }
    log.Printf("[github-plugin] registered — %d tools (reloaded=%v)", reply.ToolsRegistered, reply.Reloaded)
    return nil
}

func heartbeat(nc *nats.Conn) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        nc.Publish(plugin.SubjectHealthPrefix+"github", []byte(`{"status":"ok"}`))
    }
}
```

---

## dispatch.go

Routes incoming NATS tool call messages to the right `GitHubClient` method.

```go
// dispatch handles a tool.call.github.<name> NATS message.
func dispatch(gh *GitHubClient, msg *nats.Msg) {
    var req plugin.ToolCallRequest
    if err := json.Unmarshal(msg.Data, &req); err != nil {
        replyErr(msg, "invalid request payload")
        return
    }

    // Subject format: tool.call.github.<toolname>
    toolName := msg.Subject[len("tool.call.github."):]

    result, err := callTool(gh, toolName, req.Params)
    if err != nil {
        replyErr(msg, err.Error())
        return
    }

    payload, _ := json.Marshal(plugin.ToolCallResponse{Result: result})
    msg.Respond(payload)
}

func callTool(gh *GitHubClient, name string, params map[string]any) (any, error) {
    str := func(k string) string { v, _ := params[k].(string); return v }
    num := func(k string) int {
        switch v := params[k].(type) {
        case float64:
            return int(v)
        case int:
            return v
        }
        return 0
    }

    switch name {
    case "list_prs":
        return gh.ListPRs(str("owner"), str("repo"), str("state"))
    case "get_pr":
        return gh.GetPR(str("owner"), str("repo"), num("number"))
    case "get_diff":
        return gh.GetDiff(str("owner"), str("repo"), num("number"))
    case "list_commits":
        return gh.ListCommits(str("owner"), str("repo"), str("branch"), str("since"), num("limit"))
    case "list_issues":
        return gh.ListIssues(str("owner"), str("repo"), str("state"), str("label"), str("assignee"))
    default:
        return nil, fmt.Errorf("unknown tool: %s", name)
    }
}

func replyErr(msg *nats.Msg, errMsg string) {
    payload, _ := json.Marshal(plugin.ToolCallResponse{Error: errMsg})
    msg.Respond(payload)
}
```
