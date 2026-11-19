package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGlob_CaseInsensitiveTopLevel(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "CERT-1001_Ana.PDF"))
	touch(t, filepath.Join(dir, "CERT-1002_Bruno.pdf"))
	touch(t, filepath.Join(dir, "sub", "CERT-1001_copia.pdf"))

	idx, err := BuildIndex(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := idx.Glob("cert-1001_*.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "CERT-1001_Ana.PDF" {
		t.Errorf("Glob = %v", got)
	}
}

func TestGlob_Recursive(t *testing.T) {
	dir := t.TempDir()
	touch(t, filepath.Join(dir, "a", "b", "doc-7.pdf"))

	idx, err := BuildIndex(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := idx.Glob("doc-7.*")
	if len(got) != 1 {
		t.Errorf("Glob = %v", got)
	}
}

func TestGlob_InvalidPattern(t *testing.T) {
	idx, _ := BuildIndex(t.TempDir(), false)
	if _, err := idx.Glob("[abc"); err == nil {
		t.Error("expected error for malformed pattern")
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "Factura 01.pdf")
	touch(t, abs)
	idx, _ := BuildIndex(dir, false)

	if p, ok := idx.Resolve("factura 01.PDF"); !ok || p != abs {
		t.Errorf("Resolve relative = %q, %v", p, ok)
	}
	if p, ok := idx.Resolve(abs); !ok || p != abs {
		t.Errorf("Resolve absolute = %q, %v", p, ok)
	}
	if _, ok := idx.Resolve("nope.pdf"); ok {
		t.Error("expected missing file")
	}
}
