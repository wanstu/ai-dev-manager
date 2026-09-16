//go:build !windows

package main

import (
	"context"
	"sync"

	"ai-dev-manager-v2/internal/desktop"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const traySupported = false

type trayManager struct {
	mu  sync.RWMutex
	ctx context.Context
}

func newTrayManager(_ []byte, _ *desktop.Adapter) *trayManager { return &trayManager{} }

func (t *trayManager) Startup(ctx context.Context) {
	t.mu.Lock()
	t.ctx = ctx
	t.mu.Unlock()
}

func (t *trayManager) DomReady(context.Context) {}

func (t *trayManager) Shutdown(context.Context) {}

func (t *trayManager) ShowWindow() {
	t.mu.RLock()
	ctx := t.ctx
	t.mu.RUnlock()
	if ctx == nil {
		return
	}
	wailsruntime.WindowShow(ctx)
	wailsruntime.WindowUnminimise(ctx)
}
