package domain

import (
	"reflect"
	"testing"
)

func TestVariables_NormalizedAndUnique(t *testing.T) {
	got := Variables("Hola {{ Nombre }}, {{curso}} — {{NOMBRE}}")
	want := []string{"nombre", "curso"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Variables = %v, want %v", got, want)
	}
}

func TestRender_PlainText(t *testing.T) {
	tpl := Template{Subject: "Tu certificado — {{curso}}", Body: "Hola {{nombre}}"}
	r := Recipient{
		Email:  "ana@example.com",
		CC:     "jefe@example.com",
		Fields: map[string]string{"nombre": "Ana", "curso": "Go"},
	}

	msg := tpl.Render(r, []string{"a.pdf"})

	if msg.Subject != "Tu certificado — Go" || msg.Body != "Hola Ana" {
		t.Errorf("unexpected render: %+v", msg)
	}
	if msg.To != "ana@example.com" || msg.CC != "jefe@example.com" || len(msg.Attachments) != 1 {
		t.Errorf("unexpected envelope: %+v", msg)
	}
}

func TestRender_HTMLEscapesValuesButNotSubject(t *testing.T) {
	tpl := Template{Subject: "Para {{nombre}}", Body: "<b>Hola {{nombre}}</b>\nchao", HTML: true}
	r := Recipient{Fields: map[string]string{"nombre": "<b>Eve</b>"}}

	msg := tpl.Render(r, nil)

	if msg.Body != "<b>Hola &lt;b&gt;Eve&lt;/b&gt;</b><br>\nchao" {
		t.Errorf("body = %q", msg.Body)
	}
	if msg.Subject != "Para <b>Eve</b>" {
		t.Errorf("subject = %q", msg.Subject)
	}
}

// A designed email is indented across lines; those newlines are source
// formatting and must not become <br> (they would add gaps between tables).
func TestRender_StructuredHTMLKeepsNewlines(t *testing.T) {
	tpl := Template{Body: "<table>\n  <tr><td>{{nombre}}</td></tr>\n</table>", HTML: true}

	msg := tpl.Render(Recipient{Fields: map[string]string{"nombre": "Ana"}}, nil)

	if msg.Body != "<table>\n  <tr><td>Ana</td></tr>\n</table>" {
		t.Errorf("body = %q", msg.Body)
	}
}

func TestIsMarkup(t *testing.T) {
	cases := map[string]bool{
		"Hola <b>Ana</b>":           false,
		"texto\ncon saltos":         false,
		"<div style='x'>Hola</div>": true,
		"<P>párrafo</P>":            true,
		"<table width=600>":         true,
		"<pre>no cuenta</pre>":      false,
	}
	for in, want := range cases {
		if got := IsMarkup(in); got != want {
			t.Errorf("IsMarkup(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFill_UnknownVariableBecomesEmpty(t *testing.T) {
	if got := Fill("x{{nope}}y", map[string]string{}, false); got != "xy" {
		t.Errorf("Fill = %q", got)
	}
}
