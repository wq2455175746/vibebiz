package guides

import (
	"strings"
	"testing"
)

func TestEveryChannelHasGuide(t *testing.T) {
	for _, channel := range []string{"wechat_minigame", "douyin_minigame", "steam_desktop"} {
		if strings.TrimSpace(Markdown(channel)) == "" {
			t.Errorf("missing guide for %s", channel)
		}
	}
}

func TestListHasStepsAndRequirements(t *testing.T) {
	items := List()
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	for _, item := range items {
		if len(item.Steps) == 0 || len(item.Requirements) == 0 || item.UpdatedAt == "" {
			t.Errorf("incomplete guide: %+v", item)
		}
	}
}
