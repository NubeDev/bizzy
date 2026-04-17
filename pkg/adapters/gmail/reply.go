package gmail

import (
	"context"
	"fmt"

	"github.com/NubeDev/bizzy/pkg/command"
	gomail "github.com/wneessen/go-mail"
)

// EmailReply sends a reply via SMTP.
type EmailReply struct {
	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpPassword string
	from         string
	to           string
	subject      string
}

func (r *EmailReply) Send(ctx context.Context, msg command.ReplyMessage) error {
	if r.smtpHost == "" {
		return fmt.Errorf("SMTP not configured")
	}

	m := gomail.NewMsg()
	if err := m.From(r.from); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := m.To(r.to); err != nil {
		return fmt.Errorf("set to: %w", err)
	}
	m.Subject(r.subject)
	m.SetBodyString(gomail.TypeTextPlain, msg.Text)

	client, err := gomail.NewClient(r.smtpHost,
		gomail.WithPort(r.smtpPort),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
		gomail.WithUsername(r.smtpUser),
		gomail.WithPassword(r.smtpPassword),
		gomail.WithTLSPolicy(gomail.TLSOpportunistic),
	)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}

	return client.DialAndSendWithContext(ctx, m)
}
