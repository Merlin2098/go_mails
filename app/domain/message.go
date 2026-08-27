package domain

// Message is the outbound email the domain hands to a MailSender. It knows
// nothing about Outlook, COM, or any other transport.
type Message struct {
	To             string
	Subject        string
	Body           string
	AttachmentPath string
}
