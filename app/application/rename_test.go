package application

import (
	"os"
	"path/filepath"
	"testing"

	"despacho/app/domain"
)

func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanApplyUndoRename(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "scan_0001.pdf", "scan_0002.pdf", "scan_0004.pdf", "taken.pdf")
	table := domain.Table{
		Headers: []string{"archivo_origen", "id", "nombre"},
		Rows: [][]string{
			{"scan_0001.pdf", "1001", "Ana Torres"},
			{"scan_0002.pdf", "1002", "Bruno: Díaz"}, // ':' is illegal on Windows
			{"scan_0003.pdf", "1003", "Carla"},       // missing
			{"", "1004", "Diego"},                    // no source
			{"scan_0004.pdf", "taken", ""},           // target exists
		},
	}

	ops, err := PlanRename(table, "archivo_origen", "CERT-{{id}}_{{nombre}}{{ext}}", dir)
	if err != nil {
		t.Fatal(err)
	}
	wantStatus := []RenameStatus{RenameReady, RenameReady, RenameMissing, RenameInvalid, RenameReady}
	for i, w := range wantStatus {
		if ops[i].Status != w {
			t.Errorf("op %d status = %s (%s), want %s", i, ops[i].Status, ops[i].Detail, w)
		}
	}
	if got := filepath.Base(ops[1].To); got != "CERT-1002_Bruno_ Díaz.pdf" {
		t.Errorf("sanitized name = %q", got)
	}

	// Rule without {{ext}} and a target that already exists.
	ops2, _ := PlanRename(table, "archivo_origen", "{{id}}", dir)
	if ops2[4].Status != RenameConflict {
		t.Errorf("expected conflict for existing target, got %s", ops2[4].Status)
	}
	if filepath.Base(ops2[0].To) != "1001.pdf" {
		t.Errorf("extension not preserved: %q", ops2[0].To)
	}

	applied := ApplyRename(ops[:2])
	for _, op := range applied {
		if op.Status != RenameDone {
			t.Fatalf("apply: %+v", op)
		}
		if _, err := os.Stat(op.To); err != nil {
			t.Errorf("renamed file missing: %v", err)
		}
	}

	undone := UndoRename(applied)
	if len(undone) != 2 {
		t.Fatalf("undo = %+v", undone)
	}
	if _, err := os.Stat(filepath.Join(dir, "scan_0001.pdf")); err != nil {
		t.Errorf("undo did not restore original: %v", err)
	}
}

func TestPlanRename_DuplicateTargets(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "a.pdf", "b.pdf")
	table := domain.Table{Headers: []string{"f", "curso"}, Rows: [][]string{{"a.pdf", "Go"}, {"b.pdf", "go"}}}

	ops, err := PlanRename(table, "f", "{{curso}}.pdf", dir)
	if err != nil {
		t.Fatal(err)
	}
	if ops[0].Status != RenameConflict || ops[1].Status != RenameConflict {
		t.Errorf("expected both conflicting (case-insensitive): %+v", ops)
	}
}

func TestPlanRename_Errors(t *testing.T) {
	table := domain.Table{Headers: []string{"f"}}
	if _, err := PlanRename(table, "nope", "x", "C:/"); err == nil {
		t.Error("expected unknown column error")
	}
	if _, err := PlanRename(table, "f", "{{zzz}}", "C:/"); err == nil {
		t.Error("expected unknown variable error")
	}
	if _, err := PlanRename(table, "f", "", "C:/"); err == nil {
		t.Error("expected empty rule error")
	}
}
