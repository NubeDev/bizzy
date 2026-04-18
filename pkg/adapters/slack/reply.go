package slack

import (
	"context"
	"fmt"

	"github.com/NubeDev/bizzy/pkg/command"
	slackgo "github.com/slack-go/slack"
)

// SlackReply sends messages to a Slack channel/thread.
// Reconstructed from stored ReplyInfo — no live connection needed.
type SlackReply struct {
	client   *slackgo.Client
	channel  string
	threadTS string
}

func (r *SlackReply) Send(ctx context.Context, msg command.ReplyMessage) error {
	if r.client == nil {
		return fmt.Errorf("slack client not initialized")
	}

	opts := []slackgo.MsgOption{
		slackgo.MsgOptionText(msg.Text, false),
	}
	if r.threadTS != "" {
		opts = append(opts, slackgo.MsgOptionTS(r.threadTS))
	}

	_, _, err := r.client.PostMessageContext(ctx, r.channel, opts...)
	return err
}
