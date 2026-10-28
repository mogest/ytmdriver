package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
)

type theme struct {
	name                                 string
	accent, title, sub, faint, alert, on lipgloss.Style
}

func fg(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

var themes = []theme{
	{"terminal", fg("5"), fg("6"), fg("4"), lipgloss.NewStyle().Faint(true), fg("1"), fg("2")},
	{"youtube", fg("#ff0033"), fg("#ffffff"), fg("#ff8a9b"), fg("#717171"), fg("#ff4e45"), fg("#3ea6ff")},
	{"nord", fg("#88c0d0"), fg("#ebcb8b"), fg("#81a1c1"), fg("#616e88"), fg("#bf616a"), fg("#a3be8c")},
	{"gruvbox", fg("#fe8019"), fg("#fabd2f"), fg("#83a598"), fg("#928374"), fg("#fb4934"), fg("#b8bb26")},
	{"dracula", fg("#ff79c6"), fg("#8be9fd"), fg("#bd93f9"), fg("#6272a4"), fg("#ff5555"), fg("#50fa7b")},
}

var style = themes[0]

func themePath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "ytc", "theme")
}

func themeIndex(name string) int {
	return slices.IndexFunc(themes, func(t theme) bool { return t.name == name })
}

func loadTheme() {
	name, _ := os.ReadFile(themePath())
	if i := themeIndex(strings.TrimSpace(string(name))); i >= 0 {
		style = themes[i]
	}
}

func nextTheme() {
	style = themes[(themeIndex(style.name)+1)%len(themes)]
	os.MkdirAll(filepath.Dir(themePath()), 0o755)
	os.WriteFile(themePath(), []byte(style.name+"\n"), 0o644)
}

// help renders key/description pairs.
func help(pairs ...string) string {
	var parts []string
	for i := 0; i < len(pairs); i += 2 {
		parts = append(parts, style.sub.Render(pairs[i])+" "+style.faint.Render(pairs[i+1]))
	}
	return " " + strings.Join(parts, style.faint.Render(" · "))
}
