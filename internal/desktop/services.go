// Package desktop is the layer between the Wails runtime and the use cases:
// each exported struct here is bound to the frontend, and every exported
// method on it becomes a JavaScript function (wailsjs/go/desktop/...).
//
// It lives outside package main so it can be imported and tested; main.go
// only embeds the frontend and starts Wails.
package desktop

import (
	"context"
	"log/slog"
)

// Services wires the bound services together. It is deliberately NOT bound
// itself: Wails exposes every exported method of a bound struct to
// JavaScript, so lifecycle hooks like OnStartup must live on a type the
// frontend can never call.
type Services struct {
	App       *App
	Providers *ProviderService
	Campaign  *CampaignService
	Rename    *RenameService
}

// New builds every service. Dependencies are passed by hand — four
// constructors do not need a DI framework.
func New(log *slog.Logger) *Services {
	providers := NewProviderService(log)
	campaign := NewCampaignService(log, providers)
	return &Services{
		App:       NewApp(log),
		Providers: providers,
		Campaign:  campaign,
		Rename:    NewRenameService(log, campaign),
	}
}

// OnStartup receives the Wails runtime context, which services need to
// open native dialogs and emit events.
func (s *Services) OnStartup(ctx context.Context) {
	s.App.startup(ctx)
	s.Campaign.startup(ctx)
}

// Bindings lists the structs whose methods are exposed to the frontend.
func (s *Services) Bindings() []interface{} {
	return []interface{}{s.App, s.Providers, s.Campaign, s.Rename}
}
