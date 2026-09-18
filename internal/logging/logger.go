package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

type Logger struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	maxFiles int
	now      func() time.Time
}

type Entry struct {
	Time   time.Time         `json:"time"`
	Level  string            `json:"level"`
	Event  string            `json:"event"`
	Fields map[string]string `json:"fields,omitempty"`
}

type Status struct {
	Directory string `json:"directory"`
	MaxBytes  int64  `json:"max_bytes"`
	MaxFiles  int    `json:"max_files_per_level"`
}

func New(dir string, maxBytes int64, maxFiles int) *Logger {
	if maxBytes <= 0 {
		maxBytes = 5 * 1024 * 1024
	}
	if maxFiles <= 0 {
		maxFiles = 7
	}
	return &Logger{dir: filepath.Clean(dir), maxBytes: maxBytes, maxFiles: maxFiles, now: time.Now}
}

func (l *Logger) Status() Status {
	if l == nil {
		return Status{}
	}
	return Status{Directory: l.dir, MaxBytes: l.maxBytes, MaxFiles: l.maxFiles}
}

func (l *Logger) Log(level, event string, fields map[string]string) error {
	if l == nil {
		return nil
	}
	level = normalizeLevel(level)
	event = strings.TrimSpace(event)
	if event == "" {
		event = "event"
	}
	safeFields := make(map[string]string, len(fields))
	for key, value := range fields {
		key = strings.TrimSpace(key)
		if key == "" || isSensitiveField(key) {
			continue
		}
		safeFields[key] = strings.TrimSpace(value)
	}
	entry := Entry{Time: l.now().UTC(), Level: level, Event: event, Fields: safeFields}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(l.dir, level+".log")
	if err := l.rotateIfNeeded(path, level, entry.Time, int64(len(data))); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (l *Logger) rotateIfNeeded(path, level string, now time.Time, incoming int64) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sameDay := sameUTCDate(info.ModTime().UTC(), now.UTC())
	if sameDay && info.Size()+incoming <= l.maxBytes {
		return nil
	}
	stamp := info.ModTime().UTC().Format("20060102-150405")
	rotated := filepath.Join(l.dir, fmt.Sprintf("%s-%s.log", level, stamp))
	for index := 1; ; index++ {
		if _, err := os.Stat(rotated); os.IsNotExist(err) {
			break
		}
		rotated = filepath.Join(l.dir, fmt.Sprintf("%s-%s-%d.log", level, stamp, index))
	}
	if err := os.Rename(path, rotated); err != nil {
		return err
	}
	return l.cleanup(level)
}

func (l *Logger) cleanup(level string) error {
	matches, err := filepath.Glob(filepath.Join(l.dir, level+"-*.log"))
	if err != nil {
		return err
	}
	type item struct {
		path string
		time time.Time
	}
	items := make([]item, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		items = append(items, item{path: path, time: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].time.After(items[j].time) })
	for i := l.maxFiles; i < len(items); i++ {
		_ = os.Remove(items[i].path)
	}
	return nil
}

func normalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case LevelDebug:
		return LevelDebug
	case LevelWarn, "warning":
		return LevelWarn
	case LevelError:
		return LevelError
	default:
		return LevelInfo
	}
}

func isSensitiveField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, token := range []string{"secret", "token", "password", "api_key", "apikey", "authorization", "cookie"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return false
}

func sameUTCDate(left, right time.Time) bool {
	ly, lm, ld := left.Date()
	ry, rm, rd := right.Date()
	return ly == ry && lm == rm && ld == rd
}
