package guides

import (
	"embed"
)

//go:embed *.md
var files embed.FS

type Item struct {
	Channel      string   `json:"channel"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
	UpdatedAt    string   `json:"updatedAt"`
}

type meta struct {
	title, summary, updated string
	steps, requirements     []string
}

var metas = map[string]meta{
	"wechat_minigame": {
		title: "微信小游戏", summary: "企业主体、软著、类目审核后导入微信开发者工具",
		updated:      "2026-11-07",
		steps:        []string{"生成渠道包", "注册小游戏 AppID", "导入微信开发者工具", "真机调试", "上传提审"},
		requirements: []string{"企业主体", "软件著作权", "对应类目资质"},
	},
	"douyin_minigame": {
		title: "抖音小游戏", summary: "企业主体、软著、类目资质后导入抖音开发者工具",
		updated:      "2026-11-07",
		steps:        []string{"生成渠道包", "注册小游戏 AppID", "导入抖音开发者工具", "真机调试", "上传提审"},
		requirements: []string{"企业主体", "软件著作权", "对应类目资质"},
	},
	"steam_desktop": {
		title: "Steam 桌面", summary: "Steamworks 账号、App 费、商店素材和 SteamPipe 上传",
		updated:      "2026-11-07",
		steps:        []string{"生成渠道包", "注册 Steamworks", "本地构建 Electron 壳", "配置 Depot 和 VDF", "SteamPipe 上传并提审"},
		requirements: []string{"Steamworks 账号", "100 美元 App 费", "商店页素材", "构建产物"},
	},
}

func Markdown(channel string) string {
	data, err := files.ReadFile(channel + ".md")
	if err != nil {
		return ""
	}
	return string(data)
}

func List() []Item {
	order := []string{"wechat_minigame", "douyin_minigame", "steam_desktop"}
	items := make([]Item, 0, len(order))
	for _, channel := range order {
		m := metas[channel]
		items = append(items, Item{
			Channel: channel, Title: m.title, Summary: m.summary,
			Markdown: Markdown(channel), Steps: m.steps,
			Requirements: m.requirements, UpdatedAt: m.updated,
		})
	}
	return items
}
