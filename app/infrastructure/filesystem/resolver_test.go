package filesystem

import (
	"path/filepath"
	"testing"
)

func boletasDir(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "data", "Boletas"))
	if err != nil {
		t.Fatalf("resolving boletas dir: %v", err)
	}
	return path
}

func TestBuildIndexAndResolve_RealFolder(t *testing.T) {
	idx, err := BuildIndex(boletasDir(t))
	if err != nil {
		t.Fatalf("BuildIndex returned error: %v", err)
	}

	path, err := idx.Resolve("12345678")
	if err != nil {
		t.Fatalf("Resolve(12345678) returned error: %v", err)
	}
	if filepath.Base(path) != "12345678.pdf" {
		t.Errorf("expected 12345678.pdf, got %q", path)
	}

	path, err = idx.Resolve("44315604")
	if err != nil {
		t.Fatalf("Resolve(44315604) returned error: %v", err)
	}
	if filepath.Base(path) != "44315604.pdf" {
		t.Errorf("expected 44315604.pdf, got %q", path)
	}
}

func TestResolve_NotFound(t *testing.T) {
	idx, err := BuildIndex(boletasDir(t))
	if err != nil {
		t.Fatalf("BuildIndex returned error: %v", err)
	}

	_, err = idx.Resolve("99999999")
	if err == nil {
		t.Fatal("expected error for missing DNI, got nil")
	}
}

func TestResolve_Ambiguous(t *testing.T) {
	idx := Index{byStem: map[string][]string{
		"12345678": {"C:\\Boletas\\12345678.pdf", "C:\\Boletas\\12345678_2.pdf"},
	}}

	_, err := idx.Resolve("12345678")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	resolveErr, ok := err.(*ResolveError)
	if !ok {
		t.Fatalf("expected *ResolveError, got %T", err)
	}
	if len(resolveErr.Matches) != 2 {
		t.Errorf("expected 2 matches in error, got %d", len(resolveErr.Matches))
	}
}
