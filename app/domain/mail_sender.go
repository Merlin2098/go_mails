package domain

import "context"

// MailSender is the transport boundary between the domain and whatever
// actually delivers email: Outlook COM, SMTP, or the demo adapter that only
// writes .eml files. Batch logic depends on this interface alone, which is
// also what makes it testable with a fake.
type MailSender interface {
	Send(ctx context.Context, message Message) error
}
