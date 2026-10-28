package main

import (
	"maps"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
)

var libraryKinds = []string{"playlists", "albums"}

func (m *model) load() {
	m.loading[m.send("library", map[string]any{"kind": m.kind})] = m.kind
}

func (m model) visible() []item {
	var visible []item
	for _, it := range m.items[m.kind] {
		if it.matches(m.filter) {
			visible = append(visible, it)
		}
	}
	return visible
}

func (m *model) libraryKey(key, text string) {
	switch {
	case key == "up" || key == "k" && !m.filtering:
		m.cursor--
	case key == "down" || key == "j" && !m.filtering:
		m.cursor++
	case key == "enter":
		if visible := m.visible(); m.cursor < len(visible) {
			m.send("play_item", map[string]any{"kind": m.kind, "key": visible[m.cursor].Key})
			m.browsing = false
		}
		m.filtering, m.filter, m.cursor = false, "", 0
	case key == "tab" || key == "right" || key == "left" || (key == "l" || key == "h") && !m.filtering:
		step := 1
		if key == "left" || key == "h" {
			step = len(libraryKinds) - 1
		}
		m.kind = libraryKinds[(slices.Index(libraryKinds, m.kind)+step)%len(libraryKinds)]
		m.cursor, m.filter = 0, ""
		m.load()
	case m.filtering:
		switch key {
		case "esc":
			m.filtering, m.filter = false, ""
		case "backspace":
			if runes := []rune(m.filter); len(runes) > 0 {
				m.filter = string(runes[:len(runes)-1])
			} else {
				m.filtering = false
			}
		default:
			m.filter += text
		}
		m.cursor = 0
	case key == "esc" || key == "b":
		m.browsing = false
	case key == "/":
		m.filtering = true
	}
	m.cursor = min(max(m.cursor, 0), max(len(m.visible())-1, 0))
}

func (m model) libraryView() []string {
	var tabs []string
	for _, kind := range libraryKinds {
		name := strings.ToUpper(kind[:1]) + kind[1:]
		if kind == m.kind {
			tabs = append(tabs, bold.Render(name))
		} else {
			tabs = append(tabs, faint.Render(name))
		}
	}
	lines := []string{" " + strings.Join(tabs, faint.Render(" · "))}

	switch {
	case m.filtering:
		lines = append(lines, " /"+m.filter+"█")
	case m.filter != "":
		lines = append(lines, " /"+m.filter)
	default:
		lines = append(lines, "")
	}

	rows := max(m.height-5, 1)
	visible := m.visible()
	if _, loaded := m.items[m.kind]; loaded && len(visible) == 0 {
		lines = append(lines, faint.Render(" Nothing matches"))
	} else if !loaded && slices.Contains(slices.Collect(maps.Values(m.loading)), m.kind) {
		lines = append(lines, faint.Render(" Loading…"))
	} else if !loaded {
		lines = append(lines, faint.Render(" Couldn't load"))
	}
	clip := lipgloss.NewStyle().MaxWidth(m.width)
	start := max(min(m.cursor-rows/2, len(visible)-rows), 0)
	for i := start; i < min(start+rows, len(visible)); i++ {
		prefix := "   "
		if i == m.cursor {
			prefix = " ▸ "
		}
		lines = append(lines, clip.Render(prefix+visible[i].Title+"  "+faint.Render(visible[i].Subtitle)))
	}

	help := " j/k move · h/l playlists/albums · enter play · / filter · esc back"
	if m.filtering {
		help = " ↑/↓ move · enter play · ←/→ playlists/albums · esc clear"
	}
	return append(lines, "", faint.Render(help))
}
