package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type playerState struct {
	Title    string      `json:"title"`
	Artist   string      `json:"artist"`
	Album    string      `json:"album"`
	State    string      `json:"state"`
	Position float64     `json:"position"`
	Duration float64     `json:"duration"`
	At       int64       `json:"at"`
	Volume   int         `json:"volume"`
	Liked    bool        `json:"liked"`
	Shuffle  bool        `json:"shuffle"`
	Autoplay bool        `json:"autoplay"`
	Queue    []queueItem `json:"queue"`
}

type queueItem struct {
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Duration float64 `json:"duration"`
	Current  bool    `json:"current"`
}

type connectedMsg struct{ conn net.Conn }
type disconnectedMsg struct{}
type replyMsg reply
type tickMsg struct{}
type clearStatusMsg struct{ seq int }

type model struct {
	conn      net.Conn
	scanner   *bufio.Scanner
	state     *playerState
	width     int
	height    int
	nextID    int
	status    string
	statusSeq int

	browsing  bool
	kind      string
	items     map[string][]item
	loading   map[int]string // request id → library kind
	cursor    int
	filter    string
	filtering bool
}

func runTUI() error {
	loadTheme()
	_, err := tea.NewProgram(model{
		width:   60,
		height:  20,
		kind:    "playlists",
		items:   map[string][]item{},
		loading: map[int]string{},
	}).Run()
	return err
}

func connect() tea.Msg {
	conn, err := net.Dial("unix", socketPath())
	if err != nil {
		return disconnectedMsg{}
	}
	return connectedMsg{conn}
}

func read(scanner *bufio.Scanner) tea.Cmd {
	return func() tea.Msg {
		for scanner.Scan() {
			var r reply
			if json.Unmarshal(scanner.Bytes(), &r) == nil {
				return replyMsg(r)
			}
		}
		return disconnectedMsg{}
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) Init() tea.Cmd {
	return tea.Batch(connect, tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case connectedMsg:
		m.conn = msg.conn
		m.scanner = bufio.NewScanner(msg.conn)
		m.send("state", nil)
		return m, read(m.scanner)

	case disconnectedMsg:
		if m.conn != nil {
			m.conn.Close()
		}
		m.conn, m.state, m.browsing = nil, nil, false
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return connect() })

	case replyMsg:
		if kind, ok := m.loading[msg.ID]; ok {
			delete(m.loading, msg.ID)
			if msg.Error == "" {
				m.items[kind] = msg.Items
			}
		}
		if msg.State != nil {
			m.state = &playerState{}
			json.Unmarshal(msg.State, m.state)
		}
		if msg.Error != "" {
			return m, tea.Batch(read(m.scanner), m.setStatus(msg.Error))
		}
		return m, read(m.scanner)

	case tickMsg:
		return m, tick()

	case clearStatusMsg:
		if msg.seq == m.statusSeq {
			m.status = ""
		}

	case tea.KeyPressMsg:
		return m, m.key(msg)
	}
	return m, nil
}

func (m *model) key(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" || key == "q" && !m.filtering {
		return tea.Quit
	}
	if key == "t" && !m.filtering {
		nextTheme()
		return nil
	}
	if m.state == nil {
		if key == "space" {
			return func() tea.Msg {
				openYouTubeMusic()
				return nil
			}
		}
		return nil
	}
	if m.browsing {
		m.libraryKey(key, msg.Text)
		return nil
	}

	switch key {
	case "space":
		m.send("toggle", nil)
	case "j", "n", "down":
		m.send("next", nil)
	case "k", "p", "up":
		m.send("previous", nil)
	case "r":
		m.seek(0)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.seek(m.state.Duration * float64(key[0]-'0') / 10)
	case "h", "l", "left", "right":
		m.seek(m.position() + map[string]float64{"h": -10, "left": -10, "l": 10, "right": 10}[key])
	case "-", "+", "=":
		m.state.Volume = min(max(m.state.Volume+map[string]int{"-": -5, "+": 5, "=": 5}[key], 0), 100)
		m.send("volume", map[string]any{"level": m.state.Volume})
	case "s":
		m.send("shuffle", nil)
	case "a":
		m.send("autoplay", nil)
	case "b":
		m.browsing = true
		m.load()
	}
	return nil
}

func (m *model) seek(seconds float64) {
	m.state.Position, m.state.At = max(seconds, 0), time.Now().UnixMilli()
	m.send("seek", map[string]any{"seconds": m.state.Position})
}

func (m *model) send(cmd string, args map[string]any) int {
	if m.conn == nil {
		return 0
	}
	m.nextID++
	json.NewEncoder(m.conn).Encode(map[string]any{"id": m.nextID, "cmd": cmd, "args": args})
	return m.nextID
}

func (m *model) setStatus(status string) tea.Cmd {
	m.status = status
	m.statusSeq++
	seq := m.statusSeq
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return clearStatusMsg{seq} })
}

func (m model) position() float64 {
	position := m.state.Position
	if m.state.State == "playing" {
		position += float64(time.Now().UnixMilli()-m.state.At) / 1000
	}
	return min(position, m.state.Duration)
}

func (m model) View() tea.View {
	var lines []string
	if m.state != nil && m.browsing {
		lines = m.libraryView()
	} else if m.state == nil {
		lines = []string{
			"",
			" No YouTube Music tab — space to open",
			"",
			help("space", "open", "q", "quit"),
		}
	} else if m.state.Title == "" {
		lines = []string{
			"",
			" " + style.title.Bold(true).Render("Nothing playing"),
			"   " + style.sub.Render("press ") + style.accent.Render("b") + style.sub.Render(" to pick something from your library"),
			"",
			m.settingsView(),
			"",
			help("b", "library", "-/+", "vol", "s", "shuffle", "a", "autoplay", "t", "theme", "q", "quit"),
		}
	} else {
		s := m.state
		icon := map[string]string{"playing": "▶", "paused": "⏸", "buffering": "⋯", "ended": "■"}[s.State]
		heart := "♡"
		if s.Liked {
			heart = style.accent.Render("♥")
		}
		title := " " + style.accent.Render(icon) + " " + style.title.Bold(true).Render(s.Title)
		gap := max(m.width-lipgloss.Width(title)-2, 1)

		position := m.position()
		times := lipgloss.Width(clock(position)) + lipgloss.Width(clock(s.Duration)) + 5
		fraction := 0.0
		if s.Duration > 0 {
			fraction = position / s.Duration
		}

		lines = []string{
			title + strings.Repeat(" ", gap) + heart,
			"   " + style.sub.Render(s.Artist) + style.faint.Render(" · ") + style.sub.Render(s.Album),
			"",
			"   " + style.sub.Render(clock(position)) + " " + bar(fraction, m.width-times-3) + " " + style.sub.Render(clock(s.Duration)),
			m.settingsView(),
			"",
		}
		lines = append(lines, m.queueView(m.height-len(lines)-3)...)
		lines = append(lines,
			help("space", "play/pause", "j/k", "next/prev", "h/l", "seek", "0-9", "jump", "-/+", "vol"),
			help("s", "shuffle", "a", "autoplay", "b", "library", "t", "theme", "q", "quit"),
		)
	}
	if m.status != "" {
		lines = append(lines, style.alert.Render(" "+m.status))
	}

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

func (m model) settingsView() string {
	s := m.state
	return "   " + style.faint.Render("vol") + " " + bar(float64(s.Volume)/100, 20) + style.sub.Render(fmt.Sprintf(" %-3d", s.Volume)) + "   " + toggle("shuffle", s.Shuffle) + "  " + toggle("autoplay", s.Autoplay)
}

// queueView shows the queue from just before the current track, to fit rows lines.
func (m model) queueView(rows int) []string {
	queue := m.state.Queue
	shown := rows - 2 // heading and blank line
	if len(queue) == 0 || shown < 1 {
		return nil
	}
	current := slices.IndexFunc(queue, func(q queueItem) bool { return q.Current })
	start := max(min(current-2, len(queue)-shown), 0)
	lines := []string{style.accent.Bold(true).Render(" Queue")}
	clip := lipgloss.NewStyle().MaxWidth(m.width)
	for _, q := range queue[start:min(start+shown, len(queue))] {
		prefix, title := "   ", style.title.Render(q.Title)
		if q.Current {
			prefix, title = style.accent.Render(" ▶ "), style.title.Bold(true).Render(q.Title)
		}
		lines = append(lines, clip.Render(prefix+title+"  "+style.sub.Render(q.Artist)+style.faint.Render(" · "+clock(q.Duration))))
	}
	return append(lines, "")
}

func toggle(name string, on bool) string {
	if on {
		return style.on.Render("● " + name + " on")
	}
	return style.faint.Render("○ " + name + " off")
}

func bar(fraction float64, width int) string {
	width = max(width, 1)
	filled := int(min(max(fraction, 0), 1) * float64(width))
	return style.accent.Render(strings.Repeat("━", filled)) + style.faint.Render(strings.Repeat("─", width-filled))
}

func clock(seconds float64) string {
	s := int(seconds)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
