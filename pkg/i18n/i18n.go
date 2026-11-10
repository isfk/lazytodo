// Package i18n holds every string the user sees, and decides which language to
// show. The rule is deliberately simple: read the locale the shell exports and
// pick Chinese when it is a Chinese one, English otherwise -- the same approach
// lazygit takes, just with far fewer strings.
package i18n

import (
	"os"
	"strings"
)

// Language codes we know about.
const (
	English = "en"
	Chinese = "zh"
)

// TranslationSet is one complete language. Every field is either a plain string
// or a format string for fmt.Sprintf.
type TranslationSet struct {
	// Pane titles.
	TodosTitle  string
	DetailTitle string

	// Status filter tabs.
	FilterAll   string
	FilterTodo  string
	FilterDoing string
	FilterDone  string

	// Status bar.
	DoneCount   string // "%d/%d done"
	DoingCount  string // "%d doing"
	OnlyStatus  string // "only %s"
	TextFilter  string // "filter: %q"
	EmptyDetail string // "(no description)"
	Copied      string // "copied %q"
	ErrorPrefix string // "error: "

	// Actions, shown in the hint bar and used as binding descriptions.
	Cycle       string
	Add         string
	Edit        string
	Delete      string
	Filter      string
	Copy        string
	Back        string
	Quit        string
	Reload      string
	ClearFilter string
	NextFilter  string
	PrevFilter  string

	// Installing the agent integration.
	InstallAgent       string
	ExtInstalled       string // "%s"
	ExtInstalledBackup string // "%s", "%s"
	ExtUpToDate        string // "%s"

	// Why a binding is disabled.
	NoTodoSelected string
	NoFilter       string
	NothingToCopy  string

	// Popups.
	NewTodoTitle    string
	EditTodoTitle   string
	FilterTitle     string
	ConfirmTitle    string
	FieldTitle      string
	FieldDetail     string
	FieldMatch      string
	ConfirmDelete   string // `Delete %q?`
	FormFooter      string
	FormFooterMulti string
	ConfirmFooter   string
}

var english = TranslationSet{
	TodosTitle:  "Todos",
	DetailTitle: "Detail",

	FilterAll:   "All",
	FilterTodo:  "Todo",
	FilterDoing: "Doing",
	FilterDone:  "Done",

	DoneCount:   "%d/%d done",
	DoingCount:  "%d doing",
	OnlyStatus:  "only %s",
	TextFilter:  "filter: %q",
	EmptyDetail: "(no description)",
	Copied:      "copied %q",
	ErrorPrefix: "error: ",

	Cycle:       "Cycle",
	Add:         "Add",
	Edit:        "Edit",
	Delete:      "Delete",
	Filter:      "Filter",
	Copy:        "Copy",
	Back:        "Back",
	Quit:        "Quit",
	Reload:      "Reload",
	ClearFilter: "Clear filters",
	NextFilter:  "Next filter",
	PrevFilter:  "Prev filter",

	InstallAgent:       "Install agent extension",
	ExtInstalled:       "Installed %s",
	ExtInstalledBackup: "Installed %s (old file kept as %s)",
	ExtUpToDate:        "Extension already up to date: %s",

	NoTodoSelected: "No todo selected",
	NoFilter:       "No filter applied",
	NothingToCopy:  "Nothing to copy",

	NewTodoTitle:    "New todo",
	EditTodoTitle:   "Edit todo",
	FilterTitle:     "Filter",
	ConfirmTitle:    "Confirm",
	FieldTitle:      "Title",
	FieldDetail:     "Description",
	FieldMatch:      "Match",
	ConfirmDelete:   "Delete %q?",
	FormFooter:      "[⇥] next field  [⏎] submit  [esc] cancel",
	FormFooterMulti: "[⇥] next field  [⏎] newline  [^s] submit  [esc] cancel",
	ConfirmFooter:   "[y] yes  [n/esc] cancel",
}

var chinese = TranslationSet{
	TodosTitle:  "待办",
	DetailTitle: "详情",

	FilterAll:   "全部",
	FilterTodo:  "待办",
	FilterDoing: "进行中",
	FilterDone:  "完成",

	DoneCount:   "%d/%d 完成",
	DoingCount:  "%d 进行中",
	OnlyStatus:  "只看%s",
	TextFilter:  "过滤 %q",
	EmptyDetail: "（无详情）",
	Copied:      "已复制 %q",
	ErrorPrefix: "错误：",

	Cycle:       "切换状态",
	Add:         "新增",
	Edit:        "编辑",
	Delete:      "删除",
	Filter:      "过滤",
	Copy:        "复制",
	Back:        "返回",
	Quit:        "退出",
	Reload:      "重新载入",
	ClearFilter: "清除过滤",
	NextFilter:  "下一个筛选",
	PrevFilter:  "上一个筛选",

	InstallAgent:       "安装 agent 扩展",
	ExtInstalled:       "已安装到 %s",
	ExtInstalledBackup: "已安装到 %s（旧文件备份为 %s）",
	ExtUpToDate:        "扩展已是最新：%s",

	NoTodoSelected: "没有选中待办",
	NoFilter:       "没有生效的过滤",
	NothingToCopy:  "没有可复制的内容",

	NewTodoTitle:    "新增待办",
	EditTodoTitle:   "编辑待办",
	FilterTitle:     "过滤",
	ConfirmTitle:    "确认",
	FieldTitle:      "标题",
	FieldDetail:     "详情",
	FieldMatch:      "匹配",
	ConfirmDelete:   "确定删除 %q？",
	FormFooter:      "[⇥] 切换字段  [⏎] 确认  [esc] 取消",
	FormFooterMulti: "[⇥] 切换字段  [⏎] 换行  [^s] 确认  [esc] 取消",
	ConfirmFooter:   "[y] 是  [n/esc] 否",
}

// EnglishSet is the fallback language.
func EnglishSet() *TranslationSet { return &english }

// ChineseSet is used for any zh_* locale.
func ChineseSet() *TranslationSet { return &chinese }

// For returns the set for a language code. An empty code means "detect".
func For(language string) *TranslationSet {
	if Normalize(language) == Chinese {
		return ChineseSet()
	}
	return EnglishSet()
}

// Detect reads the environment the way libc does: LC_ALL wins over LC_MESSAGES,
// which wins over LANGUAGE, which wins over LANG. An unset or unhelpful locale
// means English.
func Detect() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		if locale := strings.TrimSpace(os.Getenv(key)); locale != "" {
			return Normalize(locale)
		}
	}
	return English
}

// Normalize turns a locale such as "zh_CN.UTF-8" or "zh-Hans-CN" into a bare
// language code, and anything that isn't Chinese into English.
func Normalize(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if i := strings.IndexAny(locale, "._@-"); i >= 0 {
		locale = locale[:i]
	}

	if locale == Chinese {
		return Chinese
	}
	return English
}
