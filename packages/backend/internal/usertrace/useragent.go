package usertrace

import "strings"

type DeviceInfo struct {
	DeviceType string
	OS         string
	Browser    string
}

func ParseUserAgent(ua string) DeviceInfo {
	if strings.TrimSpace(ua) == "" {
		return DeviceInfo{DeviceType: "other", OS: "other", Browser: "other"}
	}
	lower := strings.ToLower(ua)
	info := DeviceInfo{DeviceType: "desktop", OS: "other", Browser: "other"}

	switch {
	case strings.Contains(lower, "bot"), strings.Contains(lower, "spider"), strings.Contains(lower, "crawler"):
		info.DeviceType = "bot"
	case strings.Contains(lower, "ipad"), strings.Contains(lower, "tablet"):
		info.DeviceType = "tablet"
	case strings.Contains(lower, "android") && !strings.Contains(lower, "mobile"):
		info.DeviceType = "tablet"
	case strings.Contains(lower, "iphone"), strings.Contains(lower, "android"), strings.Contains(lower, "mobile"):
		info.DeviceType = "mobile"
	}

	switch {
	case strings.Contains(lower, "windows"):
		info.OS = "Windows"
	case strings.Contains(lower, "iphone"), strings.Contains(lower, "ipad"), strings.Contains(lower, "ios"):
		info.OS = "iOS"
	case strings.Contains(lower, "mac os x"), strings.Contains(lower, "macintosh"):
		info.OS = "macOS"
	case strings.Contains(lower, "android"):
		info.OS = "Android"
	case strings.Contains(lower, "linux"):
		info.OS = "Linux"
	}

	switch {
	case strings.Contains(lower, "edg/"):
		info.Browser = "Edge"
	case strings.Contains(lower, "chrome/"), strings.Contains(lower, "crios/"):
		info.Browser = "Chrome"
	case strings.Contains(lower, "firefox/"), strings.Contains(lower, "fxios/"):
		info.Browser = "Firefox"
	case strings.Contains(lower, "safari/"):
		info.Browser = "Safari"
	}
	return info
}
