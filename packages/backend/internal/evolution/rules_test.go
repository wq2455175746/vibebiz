package evolution

import (
	"testing"
)

func TestClassifyProposal(t *testing.T) {
	cases := []struct {
		content string
		want    string
	}{
		{"删除按钮点击没有反应", KindBug},
		{"页面报错，打不开", KindBug},
		{"首页卡片太挤了，希望更紧凑", KindVisual},
		{"输入框的布局和间距需要优化", KindVisual},
		{"希望支持多人协作", KindFeature},
		{"在官方版本左侧菜单加一个言出法随的菜单", KindVisual},
		{"接入一个新的数据库", KindFeature},
		{"我想到了一个全新的玩法", KindFeature},
	}
	for _, item := range cases {
		if got := Classify(item.content); got != item.want {
			t.Errorf("Classify(%q) = %q, want %q", item.content, got, item.want)
		}
	}
}

func TestDecideActions(t *testing.T) {
	if got := Decide("按钮失效了"); got.Action != ActionPatch || got.Scenario != "bugfix" {
		t.Fatalf("bug decision = %+v", got)
	}
	if got := Decide("首页导航布局更紧凑一些"); got.Action != ActionPreview || got.Scenario != "compact-sidebar" {
		t.Fatalf("visual decision = %+v", got)
	}
	if got := Decide("支持多人同时编辑"); got.Action != ActionPreview || got.Kind != KindFeature {
		t.Fatalf("feature decision = %+v", got)
	}
}
