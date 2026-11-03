package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/walkmiao/nodeswarm/internal/desktopcore"
)

//go:embed all:frontend/dist
var assets embed.FS

// App is the Wails native bridge. Methods are invoked within the desktop app's
// WebView2 origin; it does not start a web server or expose an HTTP admin API.
type App struct{ core *desktopcore.Service }

func (a *App) GetState() desktopcore.State               { return a.core.GetState() }
func (a *App) SaveSettings(s desktopcore.Settings) error { return a.core.SaveSettings(s) }
func (a *App) Start(enrollKey string) error              { return a.core.Start(enrollKey) }
func (a *App) Stop() error                               { return a.core.Stop() }
func (a *App) Refresh() error                            { return a.core.Refresh() }
func (a *App) Assign(pid int, startTime, nodeID string) error {
	return a.core.Assign(pid, startTime, nodeID)
}
func (a *App) Unassign(pid int, startTime string) error { return a.core.Unassign(pid, startTime) }
func (a *App) AutoFill() (int, error)                   { return a.core.AutoFill() }
func (a *App) ClearLogs()                              { a.core.ClearLogs() }

// No method exposes the device token, enrollment key or SOCKS5 passwords.

func main() {
	release, lockErr := acquireAppLock()
	if lockErr != nil {
		log.Fatal(lockErr)
	}
	defer release()
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	core, err := desktopcore.New(filepath.Join(dir, "NodeSwarmDesktop"))
	if err != nil {
		log.Fatal(err)
	}
	app := &App{core: core}
	err = wails.Run(&options.App{
		Title: "NodeSwarm Desktop",
		Width: 1280, Height: 820, MinWidth: 1080, MinHeight: 680,
		BackgroundColour: &options.RGBA{R: 246, G: 248, B: 250, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			state := core.GetState()
			if state.Settings.AutoConnect && state.CredentialReady {
				_ = core.Start("")
			}
		},
		OnShutdown: func(ctx context.Context) { _ = core.Stop() },
		Bind:       []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
