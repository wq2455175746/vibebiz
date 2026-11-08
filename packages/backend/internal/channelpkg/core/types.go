package core

import (
	"context"
	"io"
	"time"
)

const (
	ChannelWeChat = "wechat_minigame"
	ChannelDouyin = "douyin_minigame"
	ChannelSteam  = "steam_desktop"

	SourceHTML5          = "html5"
	SourceEngineArtifact = "engine_artifact"

	LevelA = "A"
	LevelB = "B"
	LevelC = "C"
)

type Report struct {
	Level    string   `json:"level"`
	Warnings []string `json:"warnings"`
	Blockers []string `json:"blockers"`
}

type SourceInfo struct {
	Dir        string `json:"-"`
	Entry      string `json:"entry"`
	Version    string `json:"version"`
	FileCount  int    `json:"fileCount"`
	TotalBytes int64  `json:"totalBytes"`
	Report     Report `json:"report"`
}

type Manifest struct {
	ProjectID      string    `json:"projectId"`
	Channel        string    `json:"channel"`
	SourceType     string    `json:"sourceType"`
	SourceVersion  string    `json:"sourceVersion"`
	AdapterVersion string    `json:"adapterVersion"`
	Entry          string    `json:"entry"`
	FileCount      int       `json:"fileCount"`
	TotalBytes     int64     `json:"totalBytes"`
	GeneratedAt    time.Time `json:"generatedAt"`
	Level          string    `json:"level"`
	Warnings       []string  `json:"warnings"`
}

type Adapter interface {
	Channel() string
	Validate(source SourceInfo) Report
	Build(ctx context.Context, source SourceInfo, out io.Writer) (Manifest, error)
}
