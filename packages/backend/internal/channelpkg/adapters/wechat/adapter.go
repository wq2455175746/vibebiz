package wechat

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/html"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/channelpkg/guides"
)

const (
	adapterVersion = "1.0.0"

	adapterFile = "adapter/index.js"
	bundleFile  = "game/game.bundle.js"
)

var (
	ErrExternalScript    = errors.New("wechat: external script is not allowed")
	ErrUnsafeScript      = errors.New("wechat: script path escapes the source directory")
	ErrUnsupportedSource = errors.New("wechat: source is not supported")
)

//go:embed adapter.js
var adapterJS []byte

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Channel() string { return core.ChannelWeChat }

func (a *Adapter) Validate(source core.SourceInfo) core.Report {
	report := core.Report{
		Level:    source.Report.Level,
		Warnings: append([]string{}, source.Report.Warnings...),
		Blockers: append([]string{}, source.Report.Blockers...),
	}
	if report.Level == core.LevelC && !contains(report.Blockers, "dom_dependency") {
		report.Blockers = append(report.Blockers, "dom_dependency")
	}
	return report
}

func (a *Adapter) Build(ctx context.Context, source core.SourceInfo, out io.Writer) (manifest core.Manifest, err error) {
	report := a.Validate(source)
	if len(report.Blockers) > 0 {
		return core.Manifest{}, ErrUnsupportedSource
	}
	if source.Dir == "" {
		return core.Manifest{}, ErrUnsupportedSource
	}

	zw := zip.NewWriter(out)
	defer func() {
		if closeErr := zw.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	manifest = core.Manifest{
		Channel:        core.ChannelWeChat,
		SourceVersion:  source.Version,
		AdapterVersion: adapterVersion,
		Entry:          filepath.ToSlash(source.Entry),
		GeneratedAt:    time.Now().UTC(),
		Level:          report.Level,
		Warnings:       append([]string{}, report.Warnings...),
	}
	written := func(name string, data []byte) error {
		if writeErr := core.WriteFile(zw, name, data); writeErr != nil {
			return writeErr
		}
		manifest.FileCount++
		manifest.TotalBytes += int64(len(data))
		return nil
	}

	if isLevelA(source) {
		err = buildLevelA(ctx, zw, source, written, &manifest)
	} else {
		err = buildLevelB(ctx, zw, source, written, &manifest)
	}
	if err != nil {
		return core.Manifest{}, err
	}

	if err = core.WriteJSON(zw, "package-manifest.json", manifest); err != nil {
		return core.Manifest{}, err
	}
	return manifest, nil
}

func buildLevelA(ctx context.Context, zw *zip.Writer, source core.SourceInfo, written func(string, []byte) error, manifest *core.Manifest) error {
	count, total, err := core.CopyDirToZip(ctx, zw, source.Dir, "", nil)
	if err != nil {
		return err
	}
	manifest.FileCount += count
	manifest.TotalBytes += total
	manifest.Entry = "game.js"

	if !fileExists(source.Dir, "project.config.json") {
		data, err := marshalJSON(projectConfig())
		if err != nil {
			return err
		}
		if err := written("project.config.json", data); err != nil {
			return err
		}
	}
	return writeGuide(written)
}

func buildLevelB(ctx context.Context, zw *zip.Writer, source core.SourceInfo, written func(string, []byte) error, manifest *core.Manifest) error {
	count, total, err := core.CopyDirToZip(ctx, zw, source.Dir, "game", nil)
	if err != nil {
		return err
	}
	manifest.FileCount += count
	manifest.TotalBytes += total

	entry, err := resolveEntryName(source)
	if err != nil {
		return err
	}
	manifest.Entry = entry

	bundle, warnings, err := extractScripts(source.Dir, entry)
	if err != nil {
		return err
	}
	manifest.Warnings = append(manifest.Warnings, warnings...)

	if err := written(bundleFile, bundle); err != nil {
		return err
	}
	if err := written(adapterFile, adapterJS); err != nil {
		return err
	}
	if err := written("game.js", []byte("require('./adapter/index.js');\nrequire('./game/game.bundle.js');\n")); err != nil {
		return err
	}
	if err := written("game.json", []byte(`{"deviceOrientation":"portrait"}`)); err != nil {
		return err
	}
	data, err := marshalJSON(projectConfig())
	if err != nil {
		return err
	}
	if err := written("project.config.json", data); err != nil {
		return err
	}
	return writeGuide(written)
}

func isLevelA(source core.SourceInfo) bool {
	if source.Report.Level == core.LevelA {
		return true
	}
	return fileExists(source.Dir, "game.js") && fileExists(source.Dir, "game.json")
}

func resolveEntryName(source core.SourceInfo) (string, error) {
	if source.Entry != "" {
		if _, err := safeSourceFile(source.Dir, source.Entry); err != nil {
			return "", err
		}
		return filepath.ToSlash(source.Entry), nil
	}
	for _, name := range []string{"index.html", "app.html"} {
		if fileExists(source.Dir, name) {
			return name, nil
		}
	}
	return "", ErrUnsupportedSource
}

func extractScripts(dir, entry string) ([]byte, []string, error) {
	entryPath, err := safeSourceFile(dir, entry)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(entryPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	doc, err := html.Parse(f)
	if err != nil {
		return nil, nil, err
	}

	var buf bytes.Buffer
	var warnings []string
	var walk func(*html.Node) error
	walk = func(node *html.Node) error {
		if node.Type == html.ElementNode && node.Data == "script" {
			return appendScript(&buf, dir, filepath.Dir(entryPath), node, &warnings)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), warnings, nil
}

func appendScript(buf *bytes.Buffer, root, base string, node *html.Node, warnings *[]string) error {
	kind := strings.ToLower(strings.TrimSpace(attrValue(node, "type")))
	if kind != "" && !isJavaScriptType(kind) {
		*warnings = append(*warnings, "skipped_script_type:"+kind)
		return nil
	}

	if src := strings.TrimSpace(attrValue(node, "src")); src != "" {
		target, err := resolveScript(root, base, src)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("wechat: read script %q: %w", src, err)
		}
		appendChunk(buf, data)
		return nil
	}

	var inline strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			inline.WriteString(child.Data)
		}
	}
	text := inline.String()
	if strings.TrimSpace(text) != "" {
		appendChunk(buf, []byte(text))
	}
	return nil
}

func appendChunk(buf *bytes.Buffer, data []byte) {
	if buf.Len() > 0 {
		buf.WriteString("\n;\n")
	}
	buf.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(";\n")
}

func resolveScript(root, base, src string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(src))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, src)
	}
	if parsed.IsAbs() || parsed.Host != "" || strings.HasPrefix(src, "//") {
		return "", fmt.Errorf("%w: %s", ErrExternalScript, src)
	}
	name := strings.ReplaceAll(parsed.Path, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(filepath.FromSlash(name)) {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, src)
	}
	target := filepath.Join(base, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, src)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("wechat: script %q: %w", src, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, src)
	}
	return target, nil
}

func safeSourceFile(dir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, "\\", "/")))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, name)
	}
	target := filepath.Join(dir, clean)
	rel, err := filepath.Rel(dir, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, name)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("wechat: source file %q: %w", name, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("wechat: source file %q is a directory", name)
	}
	return target, nil
}

func isJavaScriptType(kind string) bool {
	switch kind {
	case "text/javascript", "application/javascript", "text/ecmascript", "application/ecmascript", "module":
		return true
	}
	return false
}

func attrValue(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func projectConfig() map[string]any {
	return map[string]any{
		"appid":       "touristappid",
		"compileType": "game",
		"projectname": "vibebiz-game",
		"description": "Generated by Vibebiz channel package builder",
		"setting": map[string]any{
			"urlCheck": false,
			"es6":      true,
			"postcss":  false,
			"minified": false,
		},
	}
}

func writeGuide(written func(string, []byte) error) error {
	guide := guides.Markdown(core.ChannelWeChat)
	if strings.TrimSpace(guide) == "" {
		return errors.New("wechat: missing channel guide")
	}
	return written("CHANNEL_GUIDE.md", []byte(guide))
}

func marshalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func fileExists(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil && !info.IsDir()
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
