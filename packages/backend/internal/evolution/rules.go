package evolution

import "strings"

const (
	KindBug     = "bug"
	KindVisual  = "visual"
	KindFeature = "feature"
)

const (
	ActionPatch    = "patch"
	ActionPreview  = "preview"
	ActionPushback = "pushback"
)

var bugWords = []string{
	"bug", "报错", "错误", "崩溃", "闪退", "白屏", "空白", "失效", "没反应", "没有反应",
	"无法", "不能", "异常", "卡住", "错别字", "打不开", "加载失败",
}

var featureWords = []string{
	"新增", "增加", "支持", "接入", "功能", "模块", "重构", "架构", "数据库",
	"接口", "权限", "登录", "支付", "协作", "多用户",
}

var visualWords = []string{
	"颜色", "布局", "间距", "字体", "按钮", "样式", "优化", "交互", "体验", "文案",
	"动画", "紧凑", "卡片", "导航", "排版", "预览", "首屏", "输入框",
}

type Decision struct {
	Kind     string
	Action   string
	Scenario string
	Summary  string
	Reply    string
}

func containsAny(content string, words []string) bool {
	text := strings.ToLower(content)
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func Classify(content string) string {
	if containsAny(content, bugWords) {
		return KindBug
	}
	if containsAny(content, featureWords) {
		return KindFeature
	}
	if containsAny(content, visualWords) {
		return KindVisual
	}
	return KindFeature
}

func pickScenario(content, kind string) string {
	switch kind {
	case KindBug:
		return "bugfix"
	case KindFeature:
		return "focus-composer"
	}
	if containsAny(content, []string{"紧凑", "密度", "间距", "导航", "布局"}) {
		return "compact-sidebar"
	}
	if containsAny(content, []string{"卡片", "项目", "模板"}) {
		return "soft-cards"
	}
	if containsAny(content, []string{"首屏", "输入框", "聚焦", "创作"}) {
		return "focus-composer"
	}
	return "soft-cards"
}

func Decide(content string) Decision {
	kind := Classify(content)
	scenario := pickScenario(content, kind)
	switch kind {
	case KindBug:
		return Decision{
			Kind:     kind,
			Action:   ActionPatch,
			Scenario: scenario,
			Summary:  "修复当前用户版本中的确定性页面缺陷。",
			Reply:    "已定位到这个问题属于当前页面的确定性缺陷。基于现有交互约定，我会在你的版本副本中直接修复并渲染，官方 origin_version 保持不变。请在预览中确认修复结果。",
		}
	case KindVisual:
		return Decision{
			Kind:     kind,
			Action:   ActionPreview,
			Scenario: scenario,
			Summary:  "调整当前页面的视觉与交互呈现，不改动平台架构。",
			Reply:    "这个建议属于视觉 / 交互优化，方向可行。我的判断是：只调整当前页面的呈现与操作路径，不改动平台架构和数据契约。我已生成候选版本，请预览后确认是否采纳。",
		}
	default:
		return Decision{
			Kind:     kind,
			Action:   ActionPushback,
			Scenario: scenario,
			Summary:  "功能或架构提议，等待补充场景与验收标准。",
			Reply:    "这个提议涉及功能或架构改动，我不会直接生成。理由：一是会扩大当前版本范围；二是需要先确认与平台现有边界、数据契约不冲突；三是目前缺少可验证的验收标准。建议先补充使用场景、影响范围和成功标准，再评估是否纳入后续版本。",
		}
	}
}
