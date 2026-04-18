package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/NubeDev/bizzy/pkg/adapters"
	"github.com/NubeDev/bizzy/pkg/adapters/cron"
	gmailadapter "github.com/NubeDev/bizzy/pkg/adapters/gmail"
	slackadapter "github.com/NubeDev/bizzy/pkg/adapters/slack"
	webhookadapter "github.com/NubeDev/bizzy/pkg/adapters/webhook"
	"github.com/NubeDev/bizzy/pkg/api"
	"github.com/NubeDev/bizzy/pkg/bus"
	"github.com/NubeDev/bizzy/pkg/command"
	"github.com/NubeDev/bizzy/pkg/notify"
	"github.com/NubeDev/bizzy/pkg/plugin"
	"github.com/NubeDev/bizzy/pkg/services"
	"gorm.io/gorm"
)

// setupCommandBus initialises the NATS event bus, command router, and all
// adapters (cron, webhook, Slack, Gmail). It returns a cleanup function that
// should be deferred by the caller, or an error if the bus cannot start.
func setupCommandBus(ctx context.Context, a *api.API, agentSvc *services.AgentService, toolSvc *services.ToolService, db *gorm.DB, dataDir string) (cleanup func(), err error) {
	eventBus, err := bus.New(dataDir)
	if err != nil {
		return nil, err
	}

	a.Workflows.SetBus(eventBus)
	agentSvc.Jobs.SetBus(eventBus)

	// Wire event bus into flow engine.
	if a.FlowEngine != nil {
		a.FlowEngine.SetBus(eventBus)
		a.FlowEngine.RecoverRuns()
	}

	// --- Plugin System ---
	pluginReg := plugin.NewRegistry(plugin.RegistryConfig{
		NC: eventBus.Conn(),
		DB: db,
	})
	if err := pluginReg.Start(); err != nil {
		log.Printf("[plugins] failed to start registry: %v", err)
	} else {
		bridge := plugin.NewMCPBridge(pluginReg)
		a.PluginRegistry = pluginReg
		a.MCPFactory.SetPluginSource(bridge)
		a.MCPFactory.SetPluginQuery(bridge)
		toolSvc.PluginQuery = bridge
		fmt.Fprintf(os.Stderr, "[nube-server] plugin system: enabled\n")
	}

	adapterRegistry := adapters.NewRegistry()
	cmdParser := command.NewParser()

	cmdRouter := command.NewRouter(command.RouterConfig{
		Parser:    cmdParser,
		Workflows: a.Workflows,
		Tools:     a.ToolSvc,
		Agents: &api.CommandAgentBridge{
			AgentSvc: agentSvc,
			Jobs:     agentSvc.Jobs,
		},
		Lister:   &api.CommandToolLister{ToolSvc: toolSvc},
		Bus:      eventBus,
		Adapters: adapterRegistry,
	})

	a.CmdRouter = cmdRouter

	replyRouter := notify.NewReplyRouter(eventBus, adapterRegistry)
	if err := replyRouter.Start(); err != nil {
		log.Printf("[command-bus] reply router failed: %v", err)
	}

	notifier := notify.NewNotifier(eventBus, db, adapterRegistry)
	if err := notifier.Start(); err != nil {
		log.Printf("[command-bus] notifier failed: %v", err)
	}

	// Cron adapter (DB-driven scheduled commands).
	cronAdapter := cron.New(db)
	adapterRegistry.Register("cron", cronAdapter)
	go cronAdapter.Start(ctx, cmdRouter)

	// Webhook adapter (HTTP handler mounted on gin router).
	webhookSecret := os.Getenv("NUBE_WEBHOOK_SECRET")
	whAdapter := webhookadapter.New(webhookSecret, db)
	adapterRegistry.Register("webhook", whAdapter)
	whAdapter.Start(ctx, cmdRouter)
	a.WebhookHandler = whAdapter.Handler()

	// Slack adapter (optional).
	if slackBot := os.Getenv("NUBE_SLACK_BOT_TOKEN"); slackBot != "" {
		slackApp := os.Getenv("NUBE_SLACK_APP_TOKEN")
		sa := slackadapter.New(slackadapter.Config{
			BotToken: slackBot,
			AppToken: slackApp,
			DB:       db,
		})
		adapterRegistry.Register("slack", sa)
		go func() {
			if err := sa.Start(ctx, cmdRouter); err != nil {
				log.Printf("[slack] failed to start: %v", err)
			}
		}()
		fmt.Fprintf(os.Stderr, "[nube-server] slack adapter: enabled\n")
	}

	// Gmail adapter (optional).
	if os.Getenv("NUBE_GMAIL_ENABLED") == "true" {
		smtpPort := 587
		ga := gmailadapter.New(gmailadapter.Config{
			SMTPHost:       os.Getenv("NUBE_SMTP_HOST"),
			SMTPPort:       smtpPort,
			SMTPUser:       os.Getenv("NUBE_SMTP_USER"),
			SMTPPassword:   os.Getenv("NUBE_SMTP_PASSWORD"),
			FromAddress:    os.Getenv("NUBE_SMTP_FROM"),
			PollInterval:   2 * time.Minute,
			Query:          "is:unread label:bizzy",
			MarkRead:       true,
			AllowedDomains: strings.Split(os.Getenv("NUBE_GMAIL_ALLOWED_DOMAINS"), ","),
			DB:             db,
		})
		adapterRegistry.Register("email", ga)
		go func() {
			if err := ga.Start(ctx, cmdRouter); err != nil {
				log.Printf("[gmail] not started: %v", err)
			}
		}()
		fmt.Fprintf(os.Stderr, "[nube-server] gmail adapter: enabled\n")
	}

	fmt.Fprintf(os.Stderr, "[nube-server] command bus: enabled (NATS embedded)\n")
	return func() {
		if pluginReg != nil {
			pluginReg.Stop()
		}
		eventBus.Close()
	}, nil
}
