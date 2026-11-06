package usertrace

import "testing"

func TestParseUserAgent(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want DeviceInfo
	}{
		{"chrome mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", DeviceInfo{"desktop", "macOS", "Chrome"}},
		{"safari ios", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", DeviceInfo{"mobile", "iOS", "Safari"}},
		{"edge windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36 Edg/120.0", DeviceInfo{"desktop", "Windows", "Edge"}},
		{"firefox linux", "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", DeviceInfo{"desktop", "Linux", "Firefox"}},
		{"android phone", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", DeviceInfo{"mobile", "Android", "Chrome"}},
		{"ipad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1", DeviceInfo{"tablet", "iOS", "Safari"}},
		{"bot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", DeviceInfo{"bot", "other", "other"}},
		{"empty", "", DeviceInfo{"other", "other", "other"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseUserAgent(tc.ua); got != tc.want {
				t.Fatalf("ParseUserAgent() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
