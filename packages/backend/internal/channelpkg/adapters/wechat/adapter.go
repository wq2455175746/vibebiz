package wechat

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
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

	adapterFile      = "adapter/index.js"
	bundleFile       = "game/game.bundle.js"
	maxManifestFiles = 500
)

var (
	ErrExternalScript    = errors.New("wechat: external script is not allowed")
	ErrUnsafeScript      = errors.New("wechat: script path escapes the source directory")
	ErrUnsupportedSource = errors.New("wechat: source is not supported")
	ErrIncompleteSource  = errors.New("wechat: mini-game source is incomplete")
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

func (a *Adapter) Build(ctx context.Context, source core.SourceInfo, out io.Writer) (core.Manifest, error) {
	report := a.Validate(source)
	if len(report.Blockers) > 0 {
		return core.Manifest{}, ErrUnsupportedSource
	}
	if source.Dir == "" {
		return core.Manifest{}, ErrUnsupportedSource
	}
	levelA := isLevelA(source)
	if levelA {
		if err := validateLevelA(source.Dir); err != nil {
			return core.Manifest{}, err
		}
	}

	summaries, err := core.SummarizeFiles(ctx, source.Dir, skipWorkspacePath)
	if err != nil {
		return core.Manifest{}, err
	}
	prefix := ""
	if !levelA {
		prefix = "game/"
	}

	manifest := core.Manifest{
		ProjectID:      source.ProjectID,
		Channel:        core.ChannelWeChat,
		SourceType:     source.SourceType,
		SourceVersion:  source.Version,
		AdapterVersion: adapterVersion,
		Entry:          filepath.ToSlash(source.Entry),
		GeneratedAt:    time.Now().UTC(),
		Level:          report.Level,
		Warnings:       append([]string{}, report.Warnings...),
		Report:         report,
		Files:          make([]core.FileSummary, 0, len(summaries)),
	}
	for _, summary := range summaries {
		manifest.Files = appendFileSummary(manifest.Files, core.FileSummary{
			Path:   prefix + summary.Path,
			Size:   summary.Size,
			SHA256: summary.SHA256,
		})
	}

	tmp, err := os.CreateTemp("", "vibebiz-wechat-*.zip")
	if err != nil {
		return core.Manifest{}, err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	zw := zip.NewWriter(tmp)

	written := func(name string, data []byte) error {
		if writeErr := core.WriteFile(zw, name, data); writeErr != nil {
			return writeErr
		}
		manifest.FileCount++
		manifest.TotalBytes += int64(len(data))
		manifest.Files = appendFileSummary(manifest.Files, summarizeBytes(name, data))
		return nil
	}

	if levelA {
		err = buildLevelA(ctx, zw, source, written, &manifest)
	} else {
		err = buildLevelB(ctx, zw, source, written, &manifest)
	}
	if err != nil {
		_ = zw.Close()
		return core.Manifest{}, err
	}

	manifest.Report.Warnings = append([]string{}, manifest.Warnings...)
	if err := core.WriteJSON(zw, "package-manifest.json", manifest); err != nil {
		_ = zw.Close()
		return core.Manifest{}, err
	}
	if err := zw.Close(); err != nil {
		return core.Manifest{}, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return core.Manifest{}, err
	}
	if _, err := io.Copy(out, tmp); err != nil {
		return core.Manifest{}, err
	}
	return manifest, nil
}

func buildLevelA(ctx context.Context, zw *zip.Writer, source core.SourceInfo, written func(string, []byte) error, manifest *core.Manifest) error {
	count, total, err := core.CopyDirToZip(ctx, zw, source.Dir, "", skipWorkspacePath)
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
	count, total, err := core.CopyDirToZip(ctx, zw, source.Dir, "game", skipWorkspacePath)
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
	return regularFileExists(source.Dir, "game.js") && regularFileExists(source.Dir, "game.json")
}

func validateLevelA(dir string) error {
	for _, name := range []string{"game.js", "game.json"} {
		if !regularFileExists(dir, name) {
			return fmt.Errorf("%w: %s must be a regular file", ErrIncompleteSource, name)
		}
	}
	return nil
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
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script":
				return appendScript(&buf, dir, filepath.Dir(entryPath), node, &warnings)
			case "link":
				if hasExternalReference(attrValue(node, "href")) {
					addWarning(&warnings, "external_resource:link")
				}
			case "img":
				if hasExternalReference(attrValue(node, "src")) {
					addWarning(&warnings, "external_resource:img")
				}
			case "style":
				if hasExternalCSS(textContent(node)) {
					addWarning(&warnings, "external_css_url")
				}
			}
			if hasExternalCSS(attrValue(node, "style")) {
				addWarning(&warnings, "external_css_url")
			}
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
		addWarning(warnings, "skipped_script_type:"+kind)
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
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "//") {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, src)
	}
	parsed, err := url.Parse(src)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnsafeScript, src)
	}
	if parsed.IsAbs() || parsed.Host != "" {
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
	case "text/javascript", "application/javascript", "text/ecmascript", "application/ecmascript":
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

func skipWorkspacePath(rel string) bool {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	if len(parts) > 0 && (parts[0] == "docs" || parts[0] == "node_modules") {
		return true
	}
	switch parts[len(parts)-1] {
	case "package-manifest.json", "CHANNEL_GUIDE.md":
		return true
	}
	return false
}

func appendFileSummary(files []core.FileSummary, summary core.FileSummary) []core.FileSummary {
	if len(files) >= maxManifestFiles {
		return files
	}
	return append(files, summary)
}

func summarizeBytes(name string, data []byte) core.FileSummary {
	hasher := sha256.Sum256(data)
	return core.FileSummary{
		Path:   filepath.ToSlash(name),
		Size:   int64(len(data)),
		SHA256: hex.EncodeToString(hasher[:])[:12],
	}
}

func regularFileExists(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil && info.Mode().IsRegular()
}

func addWarning(warnings *[]string, warning string) {
	for _, existing := range *warnings {
		if existing == warning {
			return
		}
	}
	*warnings = append(*warnings, warning)
}

func hasExternalReference(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "//")
}

func hasExternalCSS(value string) bool {
	value = strings.ToLower(value)
	if strings.Contains(value, "http://") || strings.Contains(value, "https://") {
		return true
	}
	compact := strings.Join(strings.Fields(value), "")
	return strings.Contains(compact, "url(//") ||
		strings.Contains(compact, `url("//`) ||
		strings.Contains(compact, `url('//`) ||
		strings.Contains(compact, `@import"//`) ||
		strings.Contains(compact, `@import'//`) ||
		strings.Contains(compact, "@import//")
}

func textContent(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}
