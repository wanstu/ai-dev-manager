package gateway

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-dev-manager-v2/internal/app"
	"github.com/fsnotify/fsnotify"
)

const maxAutoIndexWatchedDirs = 3000

func ignoreAutoIndexDir(name string) bool {
	switch strings.ToLower(name) {
	case ".adm", ".git", ".svn", "node_modules", "vendor", "dist", "build", ".next", ".idea", ".vscode", "coverage", "target", "tmp", ".cache", "__pycache__":
		return true
	}
	return false
}
func indexedSourceEvent(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".php", ".js", ".jsx", ".ts", ".tsx":
		return true
	}
	switch strings.ToLower(filepath.Base(path)) {
	case "go.mod", "go.sum", "composer.json", "package.json":
		return true
	}
	return false
}

func (o *runtimeOwner) restoreAutoIndexWatches() {
	if o == nil || o.service == nil {
		return
	}
	ids, err := o.service.AutoIndexIDs()
	if err != nil {
		return
	}
	for _, id := range ids {
		select {
		case <-o.ctx.Done():
			return
		default:
		}
		_ = o.startAutoIndexWatch(id)
	}
}
func (o *runtimeOwner) AutoIndexStatus(id string) (app.IndexAutoStatus, error) {
	status, err := o.service.IndexAutoStatus(id)
	if err != nil {
		return status, err
	}
	if !status.Enabled {
		return status, nil
	}
	o.mu.Lock()
	_, running := o.indexWatchers[id]
	o.mu.Unlock()
	if running {
		status.State = "watching"
		status.Message = "监控文件修改，短暂延迟后自动增量更新"
	} else {
		status.State = "not_running"
		status.Message = "自动更新已配置但当前未监控；请检查目录权限或重启 Gateway"
	}
	return status, nil
}
func (o *runtimeOwner) SetAutoIndex(id string, enabled bool) (app.IndexAutoStatus, error) {
	if o == nil || o.service == nil {
		return app.IndexAutoStatus{}, errors.New("index watcher unavailable")
	}
	if _, err := o.service.Environments.Get(id); err != nil {
		return app.IndexAutoStatus{}, err
	}
	if enabled {
		if err := o.startAutoIndexWatch(id); err != nil {
			return app.IndexAutoStatus{}, err
		}
		if _, err := o.service.SetIndexAuto(id, true); err != nil {
			o.stopAutoIndexWatch(id)
			return app.IndexAutoStatus{}, err
		}
	} else {
		if _, err := o.service.SetIndexAuto(id, false); err != nil {
			return app.IndexAutoStatus{}, err
		}
		o.stopAutoIndexWatch(id)
	}
	return o.AutoIndexStatus(id)
}
func (o *runtimeOwner) stopAutoIndexWatch(id string) {
	o.mu.Lock()
	cancel := o.indexWatchers[id]
	delete(o.indexWatchers, id)
	o.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (o *runtimeOwner) startAutoIndexWatch(id string) error {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return errors.New("gateway is shutting down")
	}
	if _, ok := o.indexWatchers[id]; ok {
		o.mu.Unlock()
		return nil
	}
	o.mu.Unlock()
	rt, _, err := o.service.Runtime(id)
	if err != nil {
		return err
	}
	root := rt.Root()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	count := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && ignoreAutoIndexDir(d.Name()) {
			return filepath.SkipDir
		}
		if count >= maxAutoIndexWatchedDirs {
			return fmt.Errorf("too many directories for automatic index watch (%d); select a narrower Environment root", maxAutoIndexWatchedDirs)
		}
		if err := watcher.Add(path); err != nil {
			return err
		}
		count++
		return nil
	})
	if err != nil {
		watcher.Close()
		return fmt.Errorf("start auto index watcher: %w", err)
	}
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		watcher.Close()
		return errors.New("gateway is shutting down")
	}
	if _, exists := o.indexWatchers[id]; exists {
		o.mu.Unlock()
		watcher.Close()
		return nil
	}
	ctx, cancel := context.WithCancel(o.ctx)
	o.indexWatchers[id] = cancel
	o.mu.Unlock()
	go o.runAutoIndexWatch(ctx, id, root, watcher)
	return nil
}
func (o *runtimeOwner) runAutoIndexWatch(ctx context.Context, id, root string, watcher *fsnotify.Watcher) {
	defer watcher.Close()
	timer := time.NewTimer(700 * time.Millisecond)
	defer timer.Stop()
	queued := true
	dirty := false
	schedule := func(delay time.Duration) {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(delay)
		queued = true
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			rel, err := filepath.Rel(root, event.Name)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			skip := false
			for _, part := range strings.Split(rel, string(filepath.Separator)) {
				if ignoreAutoIndexDir(part) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			if event.Op&fsnotify.Create != 0 {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					_ = filepath.WalkDir(event.Name, func(p string, d fs.DirEntry, walkErr error) error {
						if walkErr != nil {
							return nil
						}
						if d.IsDir() {
							if ignoreAutoIndexDir(d.Name()) {
								return filepath.SkipDir
							}
							_ = watcher.Add(p)
						}
						return nil
					})
					dirty = true
					schedule(1600 * time.Millisecond)
					continue
				}
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 && indexedSourceEvent(event.Name) {
				dirty = true
				schedule(1600 * time.Millisecond)
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
			// A lost watch can be corrected by a later configuration refresh.
		case <-timer.C:
			if !queued {
				continue
			}
			queued = false
			if !dirty {
				status, err := o.service.ProjectIndexStatus(id, 1)
				if err == nil && status.State == "fresh" {
					continue
				}
			}
			// Index refresh operates on immutable source reads and a
			// private index lock, not the Environment writer lease.
			_, analyzeErr := o.service.AnalyzeProject(id, "", 0, 0)
			if analyzeErr != nil {
				schedule(12 * time.Second)
			} else {
				dirty = false
			}
		}
	}
}
