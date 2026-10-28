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

const usage = `usage: ytc                 open the player
       ytc play|pause|toggle|next|previous|restart|shuffle|autoplay|state|watch|install|help
       ytc seek <seconds>
       ytc volume <0-100>
       ytc playlists|albums
       ytc playlist|album <name>`

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
		fmt.Fprintln(os.Stderr, "ytc:", err)
		os.Exit(1)
	}
}

func socketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, fmt.Sprintf("ytc-%d.sock", os.Getuid()))
}

func run(args []string) error {
	if len(args) == 0 {
		return runTUI()
	}
	cmd := args[0]
	switch cmd {
	case "help":
		fmt.Println(usage)
		return nil
	case "install":
		return install()
	case "watch":
		return watch()
	case "playlists", "albums":
		if len(args) != 1 {
			return errors.New(usage)
		}
		return listLibrary(cmd)
	case "playlist", "album":
		if len(args) < 2 {
			return errors.New(usage)
		}
		return playFromLibrary(cmd+"s", strings.Join(args[1:], " "))
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
	case "play", "pause", "toggle", "next", "previous", "restart", "shuffle", "autoplay", "state":
		if len(args) == 0 {
			return nil, nil
		}
	case "seek", "volume":
		if len(args) == 1 {
			if n, err := strconv.ParseFloat(args[0], 64); err == nil {
				return map[string]any{map[string]string{"seek": "seconds", "volume": "level"}[cmd]: n}, nil
			}
		}
	}
	return nil, errors.New(usage)
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
			fmt.Fprintln(os.Stderr, "ytc:", r.Error)
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
