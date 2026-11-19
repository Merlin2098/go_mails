// Package logging is the only place that writes to the log file. It never
// receives document contents or full email bodies — only
// operational metadata (context.md §23-24): batch_id, filename, operation,
// status, technical error text.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// logDir is "logs" next to the running executable, in both dev and
// production. This is a deliberate deviation from context.md §17
// (%LOCALAPPDATA%\Despacho\logs\): keeping logs beside the binary makes
// them easy to find without touching mutable data inside Program Files,
// which only matters if the .exe is installed there — this MVP is
// distributed as a loose .exe the user runs from its own folder, not
// through an installer, so that risk does not apply here.
func logDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "logs"), nil
}

// Init creates logs/app.log next to the executable, truncating it if it
// already exists — one file per run, no daily rotation (see
// docs/specs/docs_enrutamiento.md §0.2 for why history is not retained
// across runs). Returns the configured logger and a closer the caller must
// defer.
func Init() (*slog.Logger, io.Closer, error) {
	dir, err := logDir()
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}

	file, err := os.OpenFile(filepath.Join(dir, "app.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, nil, err
	}

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), file, nil
}
