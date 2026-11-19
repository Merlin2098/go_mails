package domain

import (
	"html"
	"regexp"
	"strings"
)

// Template is the subject/body pair applied to every recipient of a batch.
// Placeholders use the {{column}} syntax, where column is any header of the
// input table (see NormalizeKey for how names are matched).
//
// Why not text/template? Its syntax is {{.Column}} and it fails on headers
// with spaces or accents ("{{.Fecha de emisión}}" is a parse error). The
// people writing these templates are not Go developers: a tiny, forgiving
// placeholder language is the right tool here, and it fits in a regexp.
type Template struct {
	Subject string
	Body    string
	HTML    bool
}

var placeholder = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// blockTag matches the tags that only appear in an email laid out by hand.
var blockTag = regexp.MustCompile(`(?i)<\s*(html|body|table|div|p|h[1-6]|ul|ol)[\s>/]`)

// IsMarkup reports whether s is already structured HTML (as opposed to text
// with a couple of inline tags like <b>). Such bodies keep their newlines as
// source formatting instead of turning them into <br>.
func IsMarkup(s string) bool {
	return blockTag.MatchString(s)
}

// Variables returns the distinct normalized variable names used in s, in
// order of first appearance.
func Variables(s string) []string {
	var names []string
	seen := map[string]bool{}
	for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
		key := NormalizeKey(m[1])
		if !seen[key] {
			seen[key] = true
			names = append(names, key)
		}
	}
	return names
}

// Variables returns every variable referenced by the subject or the body.
func (t Template) Variables() []string {
	return Variables(t.Subject + "\n" + t.Body)
}

// Fill replaces every {{variable}} in s with its value from fields. Unknown
// variables become "" — callers are expected to have rejected them earlier
// via Variables (validation happens before any side effect, never during a
// send). When escapeHTML is true the inserted values are HTML-escaped, so a
// recipient named "<script>" cannot inject markup into an HTML body.
func Fill(s string, fields map[string]string, escapeHTML bool) string {
	return placeholder.ReplaceAllStringFunc(s, func(match string) string {
		key := NormalizeKey(placeholder.FindStringSubmatch(match)[1])
		value := fields[key]
		if escapeHTML {
			return html.EscapeString(value)
		}
		return value
	})
}

// Render builds the outbound Message for one recipient. The subject is
// always plain text, so it is never HTML-escaped.
func (t Template) Render(r Recipient, attachments []string) Message {
	body := Fill(t.Body, r.Fields, t.HTML)
	if t.HTML && !IsMarkup(t.Body) {
		// Plain line breaks typed in the editor should survive in HTML mode
		// without forcing the user to write <br> by hand. A body that is
		// already laid out with block tags (a designed email) is left alone:
		// there the newlines only indent the source.
		body = strings.ReplaceAll(body, "\n", "<br>\n")
	}
	return Message{
		To:          r.Email,
		CC:          r.CC,
		Subject:     Fill(t.Subject, r.Fields, false),
		Body:        body,
		HTML:        t.HTML,
		Attachments: attachments,
	}
}
