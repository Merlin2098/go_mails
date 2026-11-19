package application

import (
	"path/filepath"
	"strings"
	"testing"

	"despacho/app/domain"
)

// fakeIndex is an in-memory DocumentIndex: base name -> absolute path.
type fakeIndex map[string]string

func (f fakeIndex) Glob(pattern string) ([]string, error) {
	var out []string
	for name, path := range f {
		ok, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(name))
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, path)
		}
	}
	return out, nil
}

func (f fakeIndex) Resolve(name string) (string, bool) {
	p, ok := f[name]
	return p, ok
}

var demoTable = domain.Table{
	Headers: []string{"correo", "Nombre", "documento_id", "curso", "cc"},
	Rows: [][]string{
		{"ana@example.com", "Ana", "1001", "Go", ""},
		{"karen@example", "Karen", "1002", "Go", ""},
		{"luis@example.com", "Luis", "1003", "Go", ""},
		{"bruno@example.com", "Bruno", "1001", "Go", ""},
		{"eva@example.com", "Eva", "1004", "Go", "jefe@example.com; mal"},
	},
}

var demoMapping = ColumnMapping{Email: "correo", Name: "nombre", Key: "documento_id", CC: "cc"}
var demoTemplate = domain.Template{Subject: "Certificado {{curso}}", Body: "Hola {{nombre}}"}

func TestBuildBatch_RowLevelValidation(t *testing.T) {
	docs := fakeIndex{
		"CERT-1001_Ana.pdf": `C:\d\CERT-1001_Ana.pdf`,
		"CERT-1002_K.pdf":   `C:\d\CERT-1002_K.pdf`,
		"CERT-1004_Eva.pdf": `C:\d\CERT-1004_Eva.pdf`,
	}
	rule := domain.AttachmentRule{Mode: domain.AttachPattern, Folder: `C:\d`, Pattern: "CERT-{{documento_id}}_*.pdf"}

	batch, err := BuildBatch(demoTable, demoMapping, demoTemplate, rule, docs)
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		status domain.BatchItemStatus
		field  string
	}{
		{domain.StatusReady, ""},
		{domain.StatusInvalid, "correo"},  // no dot in domain
		{domain.StatusInvalid, "adjunto"}, // CERT-1003 missing
		{domain.StatusInvalid, "clave"},   // 1001 duplicated
		{domain.StatusInvalid, "cc"},
	}
	for i, w := range want {
		got := batch.Items[i]
		if got.Status != w.status || got.ValidationField != w.field {
			t.Errorf("row %d: got %s/%s (%s), want %s/%s", i, got.Status, got.ValidationField, got.ValidationError, w.status, w.field)
		}
	}
	if batch.Items[0].Recipient.Row != 2 || len(batch.Items[0].Attachments) != 1 {
		t.Errorf("unexpected first item: %+v", batch.Items[0])
	}
	if s := batch.Summary(); s.Ready != 1 || s.Invalid != 4 {
		t.Errorf("summary = %+v", s)
	}
}

func TestBuildBatch_CampaignLevelErrors(t *testing.T) {
	cases := map[string]struct {
		m    ColumnMapping
		tpl  domain.Template
		rule domain.AttachmentRule
		docs DocumentIndex
		want string
	}{
		"no email column":  {ColumnMapping{}, demoTemplate, domain.AttachmentRule{}, nil, "correo"},
		"unknown column":   {ColumnMapping{Email: "mail"}, demoTemplate, domain.AttachmentRule{}, nil, "no existe"},
		"empty subject":    {demoMapping, domain.Template{Body: "x"}, domain.AttachmentRule{}, nil, "asunto"},
		"unknown variable": {demoMapping, domain.Template{Subject: "{{apellido}}"}, domain.AttachmentRule{}, nil, "{{apellido}}"},
		"pattern var":      {demoMapping, demoTemplate, domain.AttachmentRule{Mode: domain.AttachPattern, Pattern: "{{dni}}.pdf"}, fakeIndex{}, "{{dni}}"},
		"no folder":        {demoMapping, demoTemplate, domain.AttachmentRule{Mode: domain.AttachPattern, Pattern: "*.pdf"}, nil, "carpeta"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := BuildBatch(demoTable, c.m, c.tpl, c.rule, c.docs)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want containing %q", err, c.want)
			}
		})
	}
}

func TestBuildBatch_ColumnMode(t *testing.T) {
	table := domain.Table{
		Headers: []string{"correo", "archivos"},
		Rows: [][]string{
			{"a@example.com", "uno.pdf; dos.pdf"},
			{"b@example.com", "falta.pdf"},
			{"c@example.com", ""},
		},
	}
	docs := fakeIndex{"uno.pdf": `C:\d\uno.pdf`, "dos.pdf": `C:\d\dos.pdf`}
	rule := domain.AttachmentRule{Mode: domain.AttachColumn, Column: "archivos"}

	batch, err := BuildBatch(table, ColumnMapping{Email: "correo"}, domain.Template{Subject: "s"}, rule, docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Items[0].Attachments) != 2 || batch.Items[0].Status != domain.StatusReady {
		t.Errorf("item 0 = %+v", batch.Items[0])
	}
	if batch.Items[1].Status != domain.StatusInvalid || batch.Items[2].Status != domain.StatusInvalid {
		t.Errorf("items 1,2 should be invalid: %+v", batch.Items[1:])
	}
}

func TestIsValidEmail(t *testing.T) {
	for addr, want := range map[string]bool{
		"ana@example.com":       true,
		"a.b+tag@sub.dom.co":    true,
		"karen.luna@example":    false,
		"Ana <ana@example.com>": false,
		"x@example.":            false,
		"":                      false,
	} {
		if got := IsValidEmail(addr); got != want {
			t.Errorf("IsValidEmail(%q) = %v, want %v", addr, got, want)
		}
	}
}
