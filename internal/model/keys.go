package model

import "charm.land/bubbles/v2/key"

type KeyMap struct {
	Quit         key.Binding
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Top          key.Binding
	Bottom       key.Binding
	Search       key.Binding
	TagFilter    key.Binding
	PkgFilter    key.Binding
	PidFilter    key.Binding
	Pause        key.Binding
	Clear        key.Binding
	DevicePicker key.Binding
	Export       key.Binding
	Bookmark     key.Binding
	NextBookmark key.Binding
	PrevBookmark key.Binding
	WrapToggle   key.Binding
	AutoScroll   key.Binding
	Help         key.Binding
	Confirm      key.Binding
	Cancel       key.Binding
	LevelV       key.Binding
	LevelD       key.Binding
	LevelI       key.Binding
	LevelW       key.Binding
	LevelE       key.Binding
	LevelF       key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "退出"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "向上"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "向下"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("PgUp", "上翻页"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("PgDn", "下翻页"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("Ctrl+u", "上半页"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("Ctrl+d", "下半页"),
		),
		Top: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g/Home", "顶部"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/End", "底部"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "搜索"),
		),
		TagFilter: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "Tag过滤"),
		),
		PkgFilter: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "包名过滤"),
		),
		PidFilter: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "PID过滤"),
		),
		Pause: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("Space", "暂停/恢复"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "清除日志"),
		),
		DevicePicker: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "选择设备"),
		),
		Export: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "导出日志"),
		),
		Bookmark: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "书签"),
		),
		NextBookmark: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "下一书签"),
		),
		PrevBookmark: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "上一书签"),
		),
		WrapToggle: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "换行切换"),
		),
		AutoScroll: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "自动滚动"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "帮助"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "确认"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "取消"),
		),
		LevelV: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "Verbose"),
		),
		LevelD: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "Debug"),
		),
		LevelI: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "Info"),
		),
		LevelW: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "Warn"),
		),
		LevelE: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "Error"),
		),
		LevelF: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "Fatal"),
		),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Search, k.Pause, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom},
		{k.Search, k.TagFilter, k.PkgFilter, k.PidFilter},
		{k.LevelV, k.LevelD, k.LevelI, k.LevelW, k.LevelE, k.LevelF},
		{k.Pause, k.Clear, k.DevicePicker, k.Export},
		{k.Bookmark, k.NextBookmark, k.PrevBookmark},
		{k.WrapToggle, k.AutoScroll, k.Help, k.Quit},
	}
}
