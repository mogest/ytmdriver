package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const usage = `ytm controls YouTube Music playing in a Chrome tab, through the ytmdriver extension.

usage: ytm [command [args]]

With no command, opens the interactive player.

Playback
  play               start playback; with no YouTube Music tab open, opens music.youtube.com instead
  play <words>       search YouTube Music and play the first result, e.g. ytm play pyramid song;
                     takes the same filters as search
  pause              pause playback
  toggle             pause if playing, otherwise play
  next               skip to the next track
  previous           go to the previous track, however far into the current one
  restart            go back to the start of the current track
  seek <seconds>     jump to that position in the current track, e.g. ytm seek 90
  volume <0-100>     set the volume
  shuffle <on|off>   turn shuffle on or off
  autoplay <on|off>  turn suggested tracks after the queue runs out on or off
  stop-after         toggle pausing when the current track ends

Library
  playlists         list saved playlists, one per line: title<TAB>subtitle
  albums            list saved albums, one per line: title<TAB>subtitle
  playlist <words>  play a matching playlist (any case): exact title first, then every word in
                    the title, then title and subtitle; lists candidates if the best matches tie
  album <words>     the same, for albums, e.g. ytm album djesse 4
  search <words>    search YouTube Music, listing results one per line: number<TAB>title<TAB>subtitle;
                    --songs, --albums, --videos or --playlists first shows only those, e.g.
                    ytm search --albums ok computer
  play-result <n>   play result n of the last search, while the tab still shows its results

  Album commands take the tab to Library > Albums, and searches to the results page; playback
  carries on.

State
  state             print the current state as JSON
  watch             print the current state, then the new state on each change, one JSON object
                    per line, until interrupted

  title, artist, album, artworkUrl  the current track
  state                             playing, paused, buffering or ended
  position, duration                seconds
  at                                epoch ms when position was read; while playing, the current
                                    position is position + (now - at) / 1000
  volume                            0-100
  liked, shuffle, autoplay          true or false, or null if the control can't be found
  stopAfter                         whether stop-after is on
  queue                             [{title, artist, duration, current}], the up next list
                                    including tracks already played; current marks this track

Setup
  install           register ytm as Chrome's native messaging host; re-run after moving the binary
  help              show this help

Playback commands print nothing and exit once the control has been operated; the change shows up
in state a moment later. Errors go to stderr with exit status 1, e.g. "no YouTube Music tab open".`

var errUsage = errors.New("unrecognised command or arguments; see ytm help")

var errNoTab = errors.New("no YouTube Music tab open")

type reply struct {
	ID    int             `json:"id"`
	Event string          `json:"event"`
	OK    bool            `json:"ok"`
	Error string          `json:"error"`
	State json.RawMessage `json:"state"`
	Items []item          `json:"items"`
}

func main() {
	// Chrome launches native hosts with the caller's origin as the first argument.
	if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "chrome-extension://") {
		runHost()
		return
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ytm:", err)
		os.Exit(1)
	}
}

func socketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, fmt.Sprintf("ytm-%d.sock", os.Getuid()))
}

func run(args []string) error {
	if len(args) == 0 {
		return runTUI()
	}
	cmd := args[0]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Println(usage)
		return nil
	case "install":
		return install()
	case "watch":
		return watch()
	case "playlists", "albums":
		if len(args) != 1 {
			return errUsage
		}
		return printItems(libraryItems(cmd))
	case "search":
		query, filter, err := searchArgs(args[1:])
		if err != nil {
			return err
		}
		return printResults(searchItems(query, filter))
	case "play-result":
		if len(args) != 2 {
			return errUsage
		}
		n, err := strconv.Atoi(args[1])
		if err != nil {
			return errUsage
		}
		return playResult(n)
	case "playlist", "album":
		if len(args) < 2 {
			return errUsage
		}
		return playFromLibrary(cmd+"s", strings.Join(args[1:], " "))
	case "play":
		if len(args) > 1 {
			query, filter, err := searchArgs(args[1:])
			if err != nil {
				return err
			}
			return playSearch(query, filter)
		}
	}

	params, err := commandArgs(cmd, args[1:])
	if err != nil {
		return err
	}
	r, err := request(cmd, params)
	if err == errNoTab && cmd == "play" {
		return openYouTubeMusic()
	}
	if err != nil {
		return err
	}
	if cmd == "state" {
		var out bytes.Buffer
		json.Indent(&out, r.State, "", "  ")
		fmt.Println(out.String())
	}
	return nil
}

func commandArgs(cmd string, args []string) (map[string]any, error) {
	switch cmd {
	case "play", "pause", "toggle", "next", "previous", "restart", "stop-after", "state":
		if len(args) == 0 {
			return nil, nil
		}
	case "shuffle", "autoplay":
		if len(args) == 1 && (args[0] == "on" || args[0] == "off") {
			return map[string]any{"enabled": args[0] == "on"}, nil
		}
	case "seek", "volume":
		if len(args) == 1 {
			if n, err := strconv.ParseFloat(args[0], 64); err == nil {
				return map[string]any{map[string]string{"seek": "seconds", "volume": "level"}[cmd]: n}, nil
			}
		}
	}
	return nil, errUsage
}

// request sends one command and waits for its reply.
func request(cmd string, params map[string]any) (reply, error) {
	conn, err := net.Dial("unix", socketPath())
	if err != nil {
		return reply{}, errNoTab
	}
	defer conn.Close()

	// Long enough for library commands, which navigate the page.
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err := json.NewEncoder(conn).Encode(map[string]any{"id": 1, "cmd": cmd, "args": params}); err != nil {
		return reply{}, err
	}
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var r reply
		if json.Unmarshal(scanner.Bytes(), &r) == nil && r.Event == "" && r.ID == 1 {
			if !r.OK {
				return r, errors.New(r.Error)
			}
			return r, nil
		}
	}
	return reply{}, errors.New("no response from YouTube Music")
}

func watch() error {
	conn, err := net.Dial("unix", socketPath())
	if err != nil {
		return errNoTab
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(map[string]any{"id": 1, "cmd": "state"}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var r reply
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			continue
		}
		if r.Error != "" {
			fmt.Fprintln(os.Stderr, "ytm:", r.Error)
		} else if r.State != nil {
			fmt.Println(string(r.State))
		}
	}
	return scanner.Err()
}

func openYouTubeMusic() error {
	opener := map[string]string{"darwin": "open", "linux": "xdg-open"}[runtime.GOOS]
	if opener == "" {
		return errNoTab
	}
	return exec.Command(opener, "https://music.youtube.com").Run()
}
