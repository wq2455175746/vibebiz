package usertrace

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var traceFileMu sync.Mutex

type Recorder struct {
	dir    string
	plane  string
	logger *slog.Logger
	now    func() time.Time
	newID  func() string
	mu     sync.Mutex
}

func NewRecorder(dir, plane string, logger *slog.Logger) *Recorder {
	return &Recorder{
		dir:    dir,
		plane:  plane,
		logger: logger,
		now:    time.Now,
		newID:  uuid.NewString,
	}
}

func (r *Recorder) Append(event Event) {
	if r == nil {
		return
	}
	if event.EventAt.IsZero() {
		event.EventAt = r.now()
	}
	if event.EventID == "" {
		event.EventID = r.newID()
	}
	event = clampEvent(event)
	if event.DeviceType == "" || event.OS == "" || event.Browser == "" {
		device := ParseUserAgent(event.UserAgent)
		if event.DeviceType == "" {
			event.DeviceType = device.DeviceType
		}
		if event.OS == "" {
			event.OS = device.OS
		}
		if event.Browser == "" {
			event.Browser = device.Browser
		}
	}

	line, err := json.Marshal(event)
	if err != nil {
		r.warn("marshal user trace failed", err)
		return
	}
	path := r.path(event)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.warn("create user trace dir failed", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	traceFileMu.Lock()
	defer traceFileMu.Unlock()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		r.warn("open user trace file failed", err)
		return
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		r.warn("write user trace file failed", err)
	}
}

func (r *Recorder) path(event Event) string {
	name := "anonymous"
	if event.UserID > 0 {
		name = fmt.Sprintf("%d", event.UserID)
	}
	return filepath.Join(r.dir, r.plane, event.EventAt.Format("2006-01-02"), name+".log")
}

func (r *Recorder) warn(msg string, err error) {
	if r.logger != nil {
		r.logger.Warn(msg, "error", err)
	}
}

func TrimPlane(plane string) string {
	plane = strings.ToLower(strings.TrimSpace(plane))
	if plane != "dp" {
		return "cp"
	}
	return plane
}
