package domain

import "context"

// MailSender is the transport boundary between the domain and whatever
// actually delivers email. The MVP implements it with an Outlook COM
// adapter (app/infrastructure/outlook); tests use a fake implementation.
// A Microsoft Graph adapter can be added later without touching batch
// processing logic.
type MailSender interface {
	Send(ctx context.Context, message Message) error
}
