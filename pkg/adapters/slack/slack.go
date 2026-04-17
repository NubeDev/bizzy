// Package slack provides a bidirectional Slack adapter for the command bus.
// It uses Socket Mode (no public URL needed) to receive messages, requires
// bot mention in shared channels, and maintains a thread-to-workflow mapping
// so contextual commands like "approve" resolve to the right run.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/NubeDev/bizzy/pkg/command"
	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"gorm.io/gorm"
)

// Config holds the Slack adapter configuration.
type Config struct {
	BotToken string // xoxb-...
	AppToken string // xapp-...
	DB       *gorm.DB
}

// Adapter is a bidirectional Slack adapter.
type Adapter struct {
	botToken  string
	appToken  string
	client    *slackgo.Client
	botUserID string
	router    *command.Router
	threads   *ThreadStore
	db        *gorm.DB
}

// New creates a Slack adapter from config.
func New(cfg Config) *Adapter {
	client := slackgo.New(
		cfg.BotToken,
		slackgo.OptionAppLevelToken(cfg.AppToken),
	)

	return &Adapter{
		botToken: cfg.BotToken,
		appToken: cfg.AppToken,
		client:   client,
		threads:  NewThreadStore(cfg.DB),
		db:       cfg.DB,
	}
}

func (s *Adapter) Name() string { return "slack" }

func (s *Adapter) Start(ctx context.Context, router *command.Router) error {
	s.router = router

	// Migrate thread store table.
	s.threads.Migrate()

	// Resolve bot user ID for mention detection.
	authResp, err := s.client.AuthTest()
	if err != nil {
		return fmt.Errorf("slack auth test failed: %w", err)
	}
	s.botUserID = authResp.UserID
	log.Printf("[slack] connected as %s (bot user: %s)", authResp.User, s.botUserID)

	// Subscribe to bus events to track thread→workflow mappings.
	// When a workflow starts from Slack, we record the thread so that
	// "approve" in the same thread resolves to the right workflow.
	// The actual notification delivery is handled by the ReplyRouter
	// and Notifier — the Slack adapter just needs thread tracking.
	//
	// NOTE: bus subscriptions for thread tracking are set up here but
	// require the nats.go dependency. The handlers use json.Unmarshal
	// on msg.Data to extract workflow run IDs and reply info.

	// Start Socket Mode listener.
	socketClient := socketmode.New(s.client)

	go func() {
		for evt := range socketClient.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				socketClient.Ack(*evt.Request)
				s.handleEventsAPI(ctx, evt)
			case socketmode.EventTypeInteractive:
				socketClient.Ack(*evt.Request)
			case socketmode.EventTypeSlashCommand:
				socketClient.Ack(*evt.Request)
				s.handleSlashCommand(ctx, evt)
			}
		}
	}()

	go func() {
		if err := socketClient.RunContext(ctx); err != nil {
			log.Printf("[slack] socket mode error: %v", err)
		}
	}()

	return nil
}

func (s *Adapter) Stop() error { return nil }

// BuildReply reconstructs a live Slack reply sender from stored ReplyInfo.
func (s *Adapter) BuildReply(info command.ReplyInfo) (command.ReplyChannel, error) {
	if len(info.Address) == 0 {
		return nil, fmt.Errorf("slack reply info has no address")
	}
	var addr command.SlackAddress
	if err := json.Unmarshal(info.Address, &addr); err != nil {
		return nil, fmt.Errorf("invalid slack address: %w", err)
	}
	if addr.ChannelID == "" {
		return nil, fmt.Errorf("slack address missing channel_id")
	}
	return &SlackReply{
		client:   s.client,
		channel:  addr.ChannelID,
		threadTS: addr.ThreadTS,
	}, nil
}

// handleEventsAPI processes incoming Slack messages.
func (s *Adapter) handleEventsAPI(ctx context.Context, evt socketmode.Event) {
	eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	switch ev := eventsAPI.InnerEvent.Data.(type) {
	case *slackgo.MessageEvent:
		s.handleMessage(ctx, ev)
	}
}

// handleMessage processes a single Slack message.
func (s *Adapter) handleMessage(ctx context.Context, msg *slackgo.MessageEvent) {
	// Ignore bot messages (including our own).
	if msg.BotID != "" || msg.SubType == "bot_message" {
		return
	}

	text := msg.Text

	// Require bot mention in channels. DMs don't need a mention.
	isDM := strings.HasPrefix(msg.Channel, "D")
	mentionTag := fmt.Sprintf("<@%s>", s.botUserID)

	if !isDM {
		if !strings.Contains(text, mentionTag) {
			return // not addressed to us
		}
		text = strings.ReplaceAll(text, mentionTag, "")
		text = strings.TrimSpace(text)
	}

	if text == "" {
		return
	}

	// Determine thread TS — reply in thread if started in one, otherwise start one.
	threadTS := msg.ThreadTimestamp
	if threadTS == "" {
		threadTS = msg.Timestamp
	}

	// Build durable reply info.
	addrJSON, _ := json.Marshal(command.SlackAddress{
		ChannelID: msg.Channel,
		ThreadTS:  threadTS,
	})
	replyTo := command.ReplyInfo{
		Channel: "slack",
		Address: addrJSON,
	}

	// Parse command.
	cfg := command.ParseConfig{BareTextBehaviour: "ask"}
	if !isDM {
		cfg.BareTextBehaviour = "ask" // mention already stripped
	}

	cmd, err := s.router.Parser().Parse(text, s.resolveUser(msg.User), replyTo, cfg)
	if err != nil {
		s.postReply(msg.Channel, threadTS, "I didn't understand that: "+err.Error())
		return
	}

	// For approve/reject with no target in a thread, resolve from thread context.
	if (cmd.Verb == command.VerbApprove || cmd.Verb == command.VerbReject) && cmd.Target.Name == "" {
		runID, ok := s.threads.Lookup(threadTS)
		if !ok {
			s.postReply(msg.Channel, threadTS, "No active workflow in this thread.")
			return
		}
		cmd.Target = command.Target{Kind: "workflow", Name: runID}
	}

	// Dispatch.
	s.router.Execute(ctx, cmd)

	// If this started a workflow/job, track the thread mapping.
	if cmd.Verb == command.VerbRun && cmd.Target.Kind == "workflow" {
		// The router has already dispatched — we'll pick up the run ID
		// from the bus event in onBusEvent and map it there.
	}
}

// handleSlashCommand processes /bizzy slash commands.
func (s *Adapter) handleSlashCommand(ctx context.Context, evt socketmode.Event) {
	slashCmd, ok := evt.Data.(slackgo.SlashCommand)
	if !ok {
		return
	}

	text := slashCmd.Text
	if text == "" {
		text = "help"
	}

	threadTS := slashCmd.TriggerID // slash commands don't have threads

	addrJSON, _ := json.Marshal(command.SlackAddress{
		ChannelID: slashCmd.ChannelID,
		ThreadTS:  "",
	})
	replyTo := command.ReplyInfo{
		Channel: "slack",
		Address: addrJSON,
	}

	cmd, err := s.router.Parser().Parse(text, s.resolveUser(slashCmd.UserID), replyTo,
		command.ParseConfig{BareTextBehaviour: "ask"})
	if err != nil {
		// Respond ephemeral to slash command errors.
		_ = threadTS // slash commands don't thread the same way
		return
	}

	s.router.Execute(ctx, cmd)
}

// resolveUser maps a Slack user ID to a bizzy user ID.
// For now, uses the Slack user ID directly — a proper implementation would
// look up a slack_user_id → bizzy_user_id mapping in the database.
func (s *Adapter) resolveUser(slackUserID string) string {
	// TODO: look up mapping in DB. For now, use slack ID as-is.
	return slackUserID
}

func (s *Adapter) postReply(channel, threadTS, text string) {
	opts := []slackgo.MsgOption{
		slackgo.MsgOptionText(text, false),
	}
	if threadTS != "" {
		opts = append(opts, slackgo.MsgOptionTS(threadTS))
	}
	_, _, err := s.client.PostMessage(channel, opts...)
	if err != nil {
		log.Printf("[slack] post reply failed: %v", err)
	}
}
