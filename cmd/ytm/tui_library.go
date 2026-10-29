package main

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
)

var libraryKinds = []string{"playlists", "albums", "search"}

var searchFilterChoices = append([]string{""}, searchFilters...)

var searchFilterKeys = map[string]string{"ctrl+s": "songs", "ctrl+a": "albums", "ctrl+v": "videos", "ctrl+p": "playlists"}

func (m *model) load() {
	if m.kind == "search" {
		if m.filter != "" {
			m.loading[m.send("search", map[string]any{"query": m.filter, "filter": m.searchFilter})] = m.kind
		}
		return
	}
	m.loading[m.send("library", map[string]any{"kind": m.kind})] = m.kind
}

// visible applies the filter, which on the search tab is the query the results are already for.
func (m model) visible() []item {
	if m.kind == "search" {
		return m.items[m.kind]
	}
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
	case key == "pgup" || key == "ctrl+b":
		m.cursor -= m.rows()
	case key == "pgdown" || key == "ctrl+f":
		m.cursor += m.rows()
	case key == "g" && !m.filtering:
		m.cursor = 0
	case key == "G" && !m.filtering:
		m.cursor = len(m.visible())
	case key == "enter" && m.filtering && m.kind == "search":
		m.filtering, m.cursor = false, 0
		m.load()
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
		m.cursor, m.filter, m.filtering = 0, "", m.kind == "search"
		m.load()
	case m.kind == "search" && searchFilterKeys[key] != "":
		filter := searchFilterKeys[key]
		if filter == m.searchFilter {
			filter = ""
		}
		m.searchFilter, m.filtering, m.cursor = filter, false, 0
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

func (m model) rows() int {
	return max(m.height-5, 1)
}

func (m model) libraryView() []string {
	header := " " + choices(libraryKinds, m.kind)
	if m.kind == "search" {
		header += "    " + choices(searchFilterChoices, m.searchFilter)
	}
	lines := []string{header}

	switch {
	case m.filtering:
		lines = append(lines, style.accent.Render(" /")+style.title.Render(m.filter)+style.accent.Render("█"))
	case m.filter != "":
		lines = append(lines, style.accent.Render(" /")+style.title.Render(m.filter))
	default:
		lines = append(lines, "")
	}

	rows := m.rows()
	visible := m.visible()
	if _, loaded := m.items[m.kind]; loaded && len(visible) == 0 {
		lines = append(lines, style.faint.Render(" Nothing matches"))
	} else if !loaded && slices.Contains(slices.Collect(maps.Values(m.loading)), m.kind) {
		lines = append(lines, style.faint.Render(" Loading…"))
	} else if !loaded && m.kind != "search" {
		lines = append(lines, style.faint.Render(" Couldn't load"))
	}
	clip := lipgloss.NewStyle().MaxWidth(m.width)
	start := max(min(m.cursor-rows/2, len(visible)-rows), 0)
	for i := start; i < min(start+rows, len(visible)); i++ {
		prefix, title := "   ", style.title.Render(visible[i].Title)
		if i == m.cursor {
			prefix, title = style.accent.Render(" ▸ "), style.title.Bold(true).Render(visible[i].Title)
		}
		lines = append(lines, clip.Render(prefix+title+"  "+style.sub.Render(visible[i].Subtitle)))
	}

	switch {
	case m.filtering && m.kind == "search":
		return append(lines, "", help("enter", "search", "^s/^a/^v/^p", "songs/albums/videos/playlists", "esc", "clear"))
	case m.filtering:
		return append(lines, "", help("↑/↓", "move", "pgup/pgdn", "page", "enter", "play", "tab", "switch", "esc", "clear"))
	case m.kind == "search":
		return append(lines, "", help("j/k", "move", "^b/^f", "page", "h/l", "switch", "enter", "play", "/", "search", "^s/^a/^v/^p", "filter", "esc", "back"))
	}
	return append(lines, "", help("j/k", "move", "^b/^f", "page", "h/l", "switch", "enter", "play", "/", "filter", "esc", "back"))
}

// choices renders options with the current one highlighted; an empty option is "all".
func choices(options []string, current string) string {
	var rendered []string
	for _, option := range options {
		name := cmp.Or(option, "all")
		name = strings.ToUpper(name[:1]) + name[1:]
		if option == current {
			rendered = append(rendered, style.accent.Bold(true).Render(name))
		} else {
			rendered = append(rendered, style.faint.Render(name))
		}
	}
	return strings.Join(rendered, style.faint.Render(" · "))
}
