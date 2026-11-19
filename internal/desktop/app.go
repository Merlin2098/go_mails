package desktop

import (
	"context"
	"log/slog"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App exposes the OS-level helpers the UI needs (native dialogs, opening
// links). The actual features live in ProviderService, CampaignService and
// RenameService: each is bound to Wails separately, so the frontend gets
// one generated module per concern instead of a 40-method god object.
type App struct {
	ctx context.Context
	log *slog.Logger
}

func NewApp(log *slog.Logger) *App {
	return &App{log: log}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SelectDataFile opens a native picker for the recipients table. Returns ""
// if the user cancels.
func (a *App) SelectDataFile() (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Seleccionar archivo de destinatarios",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Excel o CSV (*.xlsx, *.csv)", Pattern: "*.xlsx;*.xlsm;*.csv"},
		},
	})
}

// SelectFolder opens a native folder picker. Returns "" if the user cancels.
func (a *App) SelectFolder(title string) (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: title})
}

// OpenURL opens an https link in the system browser. Only https is allowed
// so the UI can never be used to launch arbitrary protocol handlers.
func (a *App) OpenURL(url string) {
	if strings.HasPrefix(url, "https://") {
		wailsruntime.BrowserOpenURL(a.ctx, url)
	}
}
