package demodata_test

import (
	"context"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"despacho/app/application"
	"despacho/app/domain"
	"despacho/app/infrastructure/demomail"
	"despacho/app/infrastructure/filesystem"
	"despacho/app/infrastructure/spreadsheet"
	"despacho/internal/demodata"
)

// TestDemoFlow runs the whole demo script the way the UI does, minus the UI:
// generate the dataset, rename the scans, validate, and "send" through the
// demo adapter. It is the safety net for the TikTok demo itself.
func TestDemoFlow(t *testing.T) {
	dir := t.TempDir()
	gen, err := demodata.Generate(dir, demodata.Options{Count: 12})
	if err != nil {
		t.Fatal(err)
	}

	table, err := spreadsheet.ReadTable(gen.Workbook)
	if err != nil {
		t.Fatal(err)
	}
	tpl, found, err := spreadsheet.ReadMessageTemplate(gen.Workbook)
	if err != nil || !found {
		t.Fatalf("message sheet: found=%v err=%v", found, err)
	}

	// Step 4a: rename scan_XXXX.pdf -> CERT-<id>_<nombre>.pdf
	ops, err := application.PlanRename(table, "archivo_origen", "CERT-{{documento_id}}_{{nombre}}{{ext}}", gen.ScansDir)
	if err != nil {
		t.Fatal(err)
	}
	applied := application.ApplyRename(ops)
	renamed := 0
	for _, op := range applied {
		if op.Status == application.RenameDone {
			renamed++
		}
	}
	if renamed != 11 {
		t.Fatalf("renamed = %d, want 11 (12 rows, one scan missing on purpose)", renamed)
	}
	if _, err := os.Stat(filepath.Join(gen.ScansDir, "CERT-1004_Diego Ramírez.pdf")); err != nil {
		t.Errorf("expected accented file name: %v", err)
	}

	// Step 4b + validation.
	idx, err := filesystem.BuildIndex(gen.ScansDir, false)
	if err != nil {
		t.Fatal(err)
	}
	rule := domain.AttachmentRule{Mode: domain.AttachPattern, Folder: gen.ScansDir, Pattern: "CERT-{{documento_id}}_*.pdf"}
	mapping := application.ColumnMapping{Email: "correo", Name: "nombre", Key: "documento_id", CC: "cc"}
	batch, err := application.BuildBatch(table, mapping, tpl, rule, idx)
	if err != nil {
		t.Fatal(err)
	}
	s := batch.Summary()
	if s.Total != 12 || s.Ready != 10 || s.Invalid != 2 {
		t.Fatalf("summary = %+v, want 12 total / 10 ready / 2 invalid", s)
	}
	if got := batch.Items[10].ValidationField; got != "correo" {
		t.Errorf("karen row field = %q", got)
	}
	if got := batch.Items[11].ValidationField; got != "adjunto" {
		t.Errorf("luis row field = %q", got)
	}

	// Step 5: send through the demo adapter.
	out := filepath.Join(dir, "eml")
	sender, err := demomail.New(out, mail.Address{Address: "demo@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	res := application.RunBatch(context.Background(), &batch, sender, application.RunOptions{})
	if res.Sent != 10 || res.Failed != 0 {
		t.Fatalf("result = %+v", res)
	}

	files, _ := os.ReadDir(out)
	if len(files) != 10 {
		t.Fatalf("eml files = %d, want 10", len(files))
	}
	if !tpl.HTML {
		t.Error("the demo message sheet should load as an HTML template")
	}
	raw, _ := os.ReadFile(filepath.Join(out, files[2].Name())) // Carla: has CC
	text := string(raw)
	for _, want := range []string{"To: carla.mendoza@example.com", "Cc: coordinacion@example.com", "CERT-1003_Carla Mendoza.pdf"} {
		if !strings.Contains(text, want) {
			t.Errorf("eml missing %q", want)
		}
	}

	// Undo restores the scanner names.
	application.UndoRename(applied)
	if _, err := os.Stat(filepath.Join(gen.ScansDir, "scan_0001.pdf")); err != nil {
		t.Errorf("undo: %v", err)
	}
}

// With Inbox set, every valid address becomes a sub-address of that inbox,
// and the seeded invalid one stays invalid (the demo still needs its error).
func TestGenerate_InboxRedirect(t *testing.T) {
	gen, err := demodata.Generate(t.TempDir(), demodata.Options{Count: 12, Inbox: "yo@gmail.com"})
	if err != nil {
		t.Fatal(err)
	}
	table, err := spreadsheet.ReadTable(gen.Workbook)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[int][2]string{ // row index -> {correo, cc}
		0:  {"yo+ana.torres@gmail.com", ""},
		2:  {"yo+carla.mendoza@gmail.com", "yo+coordinacion@gmail.com"},
		10: {"karen.luna@example", ""},
	}
	for i, want := range cases {
		row := table.Rows[i]
		if row[0] != want[0] || cell(row, 6) != want[1] {
			t.Errorf("row %d: correo=%q cc=%q, want %q %q", i, row[0], cell(row, 6), want[0], want[1])
		}
	}
	if application.IsValidEmail(table.Rows[10][0]) {
		t.Error("the seeded invalid address must stay invalid")
	}
}

func TestGenerate_RejectsBadInbox(t *testing.T) {
	for _, inbox := range []string{"sin-arroba", "yo+tag@gmail.com", "yo@localhost"} {
		if _, err := demodata.Generate(t.TempDir(), demodata.Options{Count: 12, Inbox: inbox}); err == nil {
			t.Errorf("Generate accepted inbox %q", inbox)
		}
	}
}

func cell(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}

func TestGenerate_ExtraRowsAreValidAndUnique(t *testing.T) {
	gen, err := demodata.Generate(t.TempDir(), demodata.Options{Count: 60})
	if err != nil {
		t.Fatal(err)
	}
	if gen.Recipients != 60 || gen.Scans != 59 {
		t.Fatalf("result = %+v", gen)
	}
	table, err := spreadsheet.ReadTable(gen.Workbook)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range table.Rows[12:] {
		if !application.IsValidEmail(row[0]) || seen[row[0]] {
			t.Errorf("bad or duplicate generated email %q", row[0])
		}
		seen[row[0]] = true
	}
}
