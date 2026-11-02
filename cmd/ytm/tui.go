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
	Title     string      `json:"title"`
	Artist    string      `json:"artist"`
	Album     string      `json:"album"`
	State     string      `json:"state"`
	Position  float64     `json:"position"`
	Duration  float64     `json:"duration"`
	At        int64       `json:"at"`
	Volume    int         `json:"volume"`
	Liked     bool        `json:"liked"`
	Shuffle   bool        `json:"shuffle"`
	Autoplay  bool        `json:"autoplay"`
	StopAfter bool        `json:"stopAfter"`
	Queue     []queueItem `json:"queue"`
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

	dragging int // the line of the bar being dragged, or 0

	browsing     bool
	kind         string
	items        map[string][]item
	loading      map[int]string // request id → library kind
	cursor       int
	filter       string
	filtering    bool
	searchFilter string
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
				if kind == "save" {
					m.selectRecent()
				}
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

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft && m.showingPlayer() {
			m.click(msg.X, msg.Y)
		}

	case tea.MouseMotionMsg:
		if msg.Button == tea.MouseLeft && m.dragging != 0 && m.showingPlayer() {
			m.drag(msg.X)
		}

	case tea.MouseReleaseMsg:
		m.dragging = 0
	}
	return m, nil
}

func (m *model) click(x, y int) {
	m.dragging = 0
	if y == seekLine || y == volumeLine {
		if start, width := m.bar(y); x >= start && x < start+width {
			m.dragging = y
			m.drag(x)
			return
		}
	}
	if _, at := m.settingsView(); y == volumeLine && x < len(at) && at[x] != "" {
		m.send(at[x], nil)
		return
	}
	top := len(m.playerView())
	_, at := m.queueView(top)
	if row := y - top; row >= 0 && row < len(at) && at[row] >= 0 {
		m.send("play_queue_item", map[string]any{"index": at[row], "title": m.state.Queue[at[row]].Title})
	}
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
		return m.libraryKey(key, msg.Text)
	}

	switch key {
	case "space":
		m.send("toggle", nil)
	case "j", "n", "down":
		m.send("next", nil)
	case "k", "up":
		m.send("previous", nil)
	case "r":
		m.seek(0)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.seek(m.state.Duration * float64(key[0]-'0') / 10)
	case "h", "l", "left", "right":
		m.seek(m.position() + map[string]float64{"h": -10, "left": -10, "l": 10, "right": 10}[key])
	case "-", ",", "+", "=", ".":
		m.setVolume(m.state.Volume + map[string]int{"-": -5, ",": -5, "+": 5, "=": 5, ".": 5}[key])
	case "s":
		m.send("shuffle", nil)
	case "a":
		m.send("autoplay", nil)
	case "x":
		m.send("stop-after", nil)
	case "b", ";":
		m.browsing = true
		if m.kind == "save" {
			m.kind, m.filter, m.cursor = "playlists", "", 0
		}
		m.load()
	case "/":
		m.browsing, m.kind, m.filtering, m.cursor = true, "search", true, 0
	case "p":
		m.browsing, m.kind, m.filter, m.filtering, m.cursor = true, "save", "", false, 0
		m.selectRecent()
		m.load()
	}
	return nil
}

func (m *model) seek(seconds float64) {
	m.state.Position, m.state.At = max(seconds, 0), time.Now().UnixMilli()
	m.send("seek", map[string]any{"seconds": m.state.Position})
}

// drag sets the dragged bar from x, which may have strayed off either end.
func (m *model) drag(x int) {
	start, width := m.bar(m.dragging)
	if width < 1 {
		return
	}
	if m.dragging == seekLine {
		m.seek(m.state.Duration * float64(min(max(x-start, 0), width-1)) / float64(width))
	} else {
		m.setVolume((x - start + 1) * 100 / width)
	}
}

func (m *model) setVolume(level int) {
	m.state.Volume = min(max(level, 0), 100)
	m.send("volume", map[string]any{"level": m.state.Volume})
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
	} else if !m.showingPlayer() {
		settings, _ := m.settingsView()
		lines = []string{
			"",
			" " + style.title.Bold(true).Render("Nothing playing"),
			"   " + style.sub.Render("press ") + style.accent.Render(";") + style.sub.Render(" to pick something from your library"),
			"",
			settings,
			"",
			help(";", "library", "/", "search", ",/.", "vol", "s", "shuffle", "a", "autoplay", "t", "theme", "q", "quit"),
		}
	} else {
		lines = m.playerView()
		queue, _ := m.queueView(len(lines))
		lines = append(lines, queue...)
		lines = append(lines,
			help("space", "play/pause", "j/k", "next/prev", "h/l", "seek", "0-9", "jump", ",/.", "vol"),
			help("s", "shuffle", "a", "autoplay", "x", "stop after", "p", "save", ";", "library", "/", "search", "t", "theme", "q", "quit"),
		)
	}
	if m.status != "" {
		lines = append(lines, style.alert.Render(" "+m.status))
	}

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m model) showingPlayer() bool {
	return m.state != nil && !m.browsing && m.state.Title != ""
}

func (m model) playerView() []string {
	s := m.state
	icon := map[string]string{"playing": "▶", "paused": "⏸", "buffering": "⋯", "ended": "■"}[s.State]
	title := " " + style.accent.Render(icon) + " " + style.title.Bold(true).Render(s.Title)
	if s.Liked {
		title += "  " + style.accent.Render("♥")
	}

	position := m.position()
	_, seekWidth := m.bar(seekLine)
	settings, _ := m.settingsView()
	fraction := 0.0
	if s.Duration > 0 {
		fraction = position / s.Duration
	}

	return []string{
		title,
		"   " + style.sub.Render(s.Artist) + style.faint.Render(" · ") + style.sub.Render(s.Album),
		"",
		"   " + style.sub.Render(clock(position)) + " " + bar(fraction, seekWidth) + " " + style.sub.Render(clock(s.Duration)),
		settings,
		"",
	}
}

// The lines of playerView that hold the seek and volume bars.
const seekLine, volumeLine = 3, 4

const volumeBarWidth = 20

// bar is where the bar on that line sits; the seek bar is between the position and duration.
func (m model) bar(line int) (start, width int) {
	if line == volumeLine {
		return 7, volumeBarWidth
	}
	start = 4 + lipgloss.Width(clock(m.position()))
	return start, m.width - start - 4 - lipgloss.Width(clock(m.state.Duration))
}

// settingsView also returns the command clicking each column sends, or "".
func (m model) settingsView() (line string, at []string) {
	s := m.state
	add := func(text, cmd string) {
		line += text
		at = append(at, slices.Repeat([]string{cmd}, lipgloss.Width(text))...)
	}
	add("   "+style.faint.Render("vol")+" "+bar(float64(s.Volume)/100, volumeBarWidth)+style.sub.Render(fmt.Sprintf(" %-3d", s.Volume))+"   ", "")
	add(toggle("shuffle", s.Shuffle), "shuffle")
	add("  ", "")
	add(toggle("autoplay", s.Autoplay), "autoplay")
	return line, at
}

// queueView shows the queue from just before the current track, to fit below top lines and above the help and
// status lines. at holds the index in the queue of each line's track, or -1.
func (m model) queueView(top int) (lines []string, at []int) {
	queue := m.state.Queue
	shown := m.height - top - 5 // help and status lines, heading and blank line
	if len(queue) == 0 || shown < 1 {
		return nil, nil
	}
	clip := lipgloss.NewStyle().MaxWidth(m.width)
	var items []string
	var indexes []int
	for i, q := range queue {
		prefix, title := "   ", style.title.Render(q.Title)
		if q.Current {
			prefix, title = style.accent.Render(" ▶ "), style.title.Bold(true).Render(q.Title)
		}
		items = append(items, clip.Render(prefix+title+"  "+style.sub.Render(q.Artist)+style.faint.Render(" · "+clock(q.Duration))))
		indexes = append(indexes, i)
		if q.Current && m.state.StopAfter {
			items = append(items, style.alert.Render("   ─── STOP ───"))
			indexes = append(indexes, -1)
		}
	}
	current := slices.IndexFunc(queue, func(q queueItem) bool { return q.Current })
	start := max(min(current-2, len(items)-shown), 0)
	end := min(start+shown, len(items))
	lines = append([]string{style.accent.Bold(true).Render(" Queue")}, items[start:end]...)
	at = append([]int{-1}, indexes[start:end]...)
	return append(lines, ""), at
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
