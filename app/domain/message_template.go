package domain

import "strings"

// MessageTemplate is the subject/body pair read from the "Mensaje" sheet of
// the input spreadsheet. It applies to the whole batch, not per-item — unlike
// BatchItem, there is exactly one MessageTemplate per send run.
type MessageTemplate struct {
	Subject string
	Body    string
}

// Render fills the {NOMBRE} placeholder with the employee's name and returns
// the Message ready to hand to a MailSender. Only {NOMBRE} is supported for
// the MVP; extend this if a real need for more placeholders appears.
func (t MessageTemplate) Render(employee Employee, attachmentPath string) Message {
	body := strings.ReplaceAll(t.Body, "{NOMBRE}", employee.Name)
	return Message{
		To:             employee.Email,
		Subject:        t.Subject,
		Body:           body,
		AttachmentPath: attachmentPath,
	}
}
