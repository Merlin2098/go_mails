package domain

import "strings"

// SplitAddresses splits a list like "a@x.com; b@y.com, c@z.com" into its
// trimmed, non-empty addresses. Spreadsheet users mix both separators.
func SplitAddresses(list string) []string {
	fields := strings.FieldsFunc(list, func(r rune) bool { return r == ';' || r == ',' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// Message is the outbound email the domain hands to a MailSender. It knows
// nothing about Outlook, SMTP or any other transport.
type Message struct {
	To          string
	CC          string // may hold several addresses separated by ";" or ","
	Subject     string
	Body        string
	HTML        bool
	Attachments []string // absolute paths, already resolved during validation
}
