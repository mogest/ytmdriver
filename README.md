# ytmdriver

ytmdriver is a keyboard-first accessibility tool and terminal remote for the YouTube Music tab
you already use. Skip a track, pick an album, or see what's playing without switching to the browser.
Your normal browser session handles the music; `ytm` puts the controls at your fingertips.

Use it two ways:

- **Interactive TUI:** run `ytm` for a keyboard-driven player with library browsing and colour themes.
- **CLI commands:** run `ytm next`, `ytm album djesse 4`, or `ytm state` for quick control,
  shell scripts and shortcuts.

```text
 ▶ Child Song
   The Cinematic Orchestra · Ma Fleur

   1:34 ━━━━━━━━━━━━━━━━━━──────────────────────── 5:14
   vol ━━━━━━━━━━━━━━━━━━━━ 100

 space play/pause · j/k next/prev · h/l seek · 0-9 jump · -/+ vol
 s shuffle · a autoplay · x stop after · b library · / search · t theme · q quit
```

## The website does the playing

**ytmdriver operates the same website controls you would use by hand.** Its Chrome extension
clicks Play, Pause and Next, adjusts the page's seek and volume sliders, and opens albums through
Library → Albums. Track information comes from the page and the browser's media metadata.

The audio stays in YouTube Music. ytmdriver doesn't download tracks, extract stream URLs, block ads,
or call YouTube's private APIs or internal JavaScript. You sign in through the website as usual;
there are no API keys or exported cookies to configure.

The guiding principle is simple: **make ordinary listening more accessible.** You choose what to
play, then ytmdriver translates your commands into interactions with the existing interface. It's an
alternative way to operate your own listening session, with playback, ads and account restrictions
all handled by the official player.

This approach is designed to respect YouTube's service and the listening experience it provides.
Use remains subject to [YouTube's Terms of Service](https://www.youtube.com/static?template=terms).
ytmdriver is an independent project and isn't endorsed by YouTube.

## A little remote that fits your workflow

- **Stay on the keyboard.** Play, pause, skip, seek and adjust volume from an interactive terminal player.
- **Pick something from your library.** Browse and filter saved playlists and albums, or play one by name.
- **Find anything else.** Search YouTube Music from the player or the command line.
- **Make it yours.** Choose terminal, YouTube, Nord, Gruvbox or Dracula colours.
- **Connect your tools.** Use individual commands for shortcuts, JSON state for status bars, and a live
  stream of updates for scripts.
- **Finish the song.** Toggle stop-after to pause near the end of the current track.

Currently supports **Google Chrome on macOS and Linux**.

## Get started

You'll need Chrome, Go (the version required by [go.mod](go.mod) or newer), and Node.js with npm.
Run these commands from the repository root.

1. **Build the extension.**

   ```sh
   npm --prefix extension install
   npm --prefix extension run build
   ```

2. **Load it in Chrome.** Open `chrome://extensions`, enable **Developer mode**, select
   **Load unpacked**, and choose this repository's `extension/dist` directory.

3. **Build and register the terminal remote.**

   ```sh
   go build -o bin/ytm ./cmd/ytm
   ./bin/ytm install
   ```

4. **Open YouTube Music.** Visit [music.youtube.com](https://music.youtube.com) in Chrome, sign in
   as usual, and start some music. Reload the tab if it was already open when you loaded the extension.

5. **Take control.**

   ```sh
   ./bin/ytm
   ```

Add the repository's `bin` directory to your `PATH` to use `ytm` from anywhere. The examples below
assume you've done that; otherwise, use `./bin/ytm` from the repository root.

Chrome starts the native host automatically when a YouTube Music tab opens. The `install` command
writes `com.seriousorange.ytmdriver.json` into Chrome's `NativeMessagingHosts` directory, pointing to
the binary's absolute path. Run it again if you move the binary. The extension's manifest fixes its
ID as `bidbhfllonlbjabgdemkdibgdillegmd`.

## Play from the terminal

Run `ytm` with no arguments to open the interactive player.

| Key | Action |
| --- | --- |
| Space | Play / pause |
| `j` / `k` or ↓ / ↑ | Next / previous track (`n` / `p` also work) |
| `h` / `l` or ← / → | Seek back / forward 10 seconds |
| `0`–`9` | Jump to 0%–90% of the track |
| `r` | Restart the track |
| `-` / `+` | Lower / raise volume by 5 |
| `s` | Toggle shuffle |
| `a` | Toggle autoplay |
| `x` | Toggle stop-after |
| `b` | Open your library |
| `/` | Search YouTube Music |
| `t` | Cycle colour themes |
| `q` | Quit the terminal player |

In the library, use `h` / `l` to switch between playlists, albums and search, `j` / `k` to move,
Enter to play, `/` to filter, and Esc to go back. On the search tab, type your search and press Enter;
`/` edits the search. Ctrl-S, Ctrl-A, Ctrl-V and Ctrl-P show only songs, albums, videos or playlists,
also while typing; pressing the same one again shows all results. Your theme choice is saved in `ytm/theme`
under your user config directory.

The player reconnects automatically when a YouTube Music tab appears. With no tab open,
Space opens the site in your default browser; use Chrome for the extension to connect.

## Commands for shortcuts and scripts

```sh
ytm toggle                 # play or pause
ytm next                   # skip this track
ytm seek 90                # jump to 1:30
ytm volume 60              # set volume to 60%
ytm shuffle on             # enable shuffle (or use off to disable)
ytm autoplay off           # disable suggested tracks after the queue ends
ytm album djesse 4         # play a saved album by name
ytm playlist favourites    # play a saved playlist by name
ytm search nick drake      # list numbered search results
ytm play-result 4          # play the 4th of them
ytm play pink moon         # play the first search result
ytm search --albums ok computer   # only albums (or --songs, --videos, --playlists)
ytm stop-after             # toggle pausing at the end of this track
ytm state                  # current player state as JSON
ytm watch                  # initial state, then one JSON object per update
```

| Command | What it does |
| --- | --- |
| `play`, `pause`, `toggle` | Control playback |
| `play [--filter] <words>` | Search and play the first result |
| `next`, `previous`, `restart` | Change tracks or restart the current one |
| `seek <seconds>` | Seek to an absolute position in the track |
| `volume <0-100>` | Set volume |
| `shuffle <on\|off>`, `autoplay <on\|off>` | Set the corresponding setting; repeating a command leaves it in that state |
| `stop-after` | Toggle pausing at the end of the current track |
| `playlists`, `albums` | List saved items as tab-separated title and subtitle |
| `playlist <words>`, `album <words>` | Play a matching library item |
| `search [--filter] <words>` | List search results as tab-separated number, title and subtitle; `--songs`, `--albums`, `--videos` or `--playlists` shows only those |
| `play-result <n>` | Play a result of the last search, while the tab still shows them |
| `state`, `watch` | Read player state once or follow updates |
| `help` | Show command help |

Library names match without regard to case. An exact title wins, followed by titles containing every
supplied word. Only if neither matches does the search include subtitles (such as artist and year).
If several items match at the best matching level, `ytm` lists those candidates.

Playback commands exit silently after operating the control; the resulting state change may arrive
a moment later. Errors go to stderr with exit status 1.

## A few browser details

Because ytmdriver works through the website, it follows the website's behaviour:

- **Keep a YouTube Music tab open.** With no tab, `ytm play` opens the site in your default browser;
  other playback commands report `no YouTube Music tab open`.
- **Show a newly loaded tab once.** Chrome can prevent playback in a tab that has never been visible,
  such as one restored in the background. Switch to it once before starting playback remotely.
- **Albums need the library page.** Album commands navigate to Library → Albums without interrupting
  playback. If the tab is hidden, switch to it once so YouTube Music can draw the album grid.
  It stays on Albums afterwards, allowing subsequent commands to work in the background.
  Playlists are read from the sidebar and don't need navigation.
- **Searches use the search box.** They type into YouTube Music's search box and read the results page,
  leaving the tab there. Results only include rows such as songs, albums and videos, not the top
  result's card itself.
- **Page changes can affect controls.** The extension depends on YouTube Music's page structure and
  English control labels. A missing-element error identifies the selector it couldn't find.

## How it fits together

- **Extension** (`extension/src`)
  - `page.ts` holds everything specific to YouTube Music's page: selectors, how each control is operated, how
    state is read. When YouTube changes the page, this is the file to fix; errors name the missing selector.
  - `content.ts` runs in each YouTube Music tab, executes commands and sends state events.
  - `background.ts` routes commands to the tab that most recently started playing (else the most recently
    opened) and connects to the native host while any YouTube Music tab is open. From the service worker
    console, `ytm('next')` or `ytm('seek', {seconds: 60})` sends commands directly.
- **`ytm host`**: launched by Chrome when the first YouTube Music tab opens, exits when the last closes.
  Listens on `ytm-<uid>.sock` in `$XDG_RUNTIME_DIR`, else `$TMPDIR`. If another host (another browser or
  profile) owns the socket, it waits and takes over when that one exits.
- **`ytm` CLI**: connects to the socket.

## Protocol

Newline-delimited JSON on the socket. The same objects pass between the host and the extension.

Request:
```json
{"id": 1, "cmd": "seek", "args": {"seconds": 42}}
{"id": 2, "cmd": "volume", "args": {"level": 60}}
{"id": 3, "cmd": "library", "args": {"kind": "albums"}}
{"id": 4, "cmd": "play_item", "args": {"kind": "albums", "key": "/browse/MPREb_VoRIcL5VxmY"}}
{"id": 5, "cmd": "search", "args": {"query": "nick drake", "filter": "albums"}}
```

Commands: `play`, `pause`, `toggle`, `next`, `previous`, `restart`, `shuffle {enabled}`, `autoplay {enabled}`, `stop-after`, `seek {seconds}`, `volume {level}`,
`state`, `library {kind}`, `search {query, filter}`, `play_item {kind, key}`. `kind` is `playlists`, `albums` or
`search`; `search` results are those on the results page, so play them before searching again. A search's
optional `filter` is `songs`, `albums`, `videos` or `playlists`, selecting that chip on the results page.

For `shuffle` and `autoplay`, `enabled` is a boolean. Omitting it toggles the setting, as used by the
interactive player; the CLI requires `on` or `off`.

`previous` always goes to the previous track. YouTube Music's own Previous button restarts the track when
you're 5s or more in, so in that case it's pressed twice.

Response:
```json
{"id": 1, "ok": true}
{"id": 5, "ok": true, "state": {...}}
{"id": 3, "ok": true, "items": [{"key": "/browse/MPREb_VoRIcL5VxmY", "title": "Ma Fleur", "subtitle": "Album • The Cinematic Orchestra • 2007"}]}
{"id": 6, "ok": false, "error": "YouTube Music element not found: ytmusic-miniplayer button[aria-label=\"Next\"]"}
```

`ok: true` means the control was operated; the resulting change arrives as an event. Only `state` returns
state, and only `library` and `search` return items. An item's `key` is opaque: pass it back to `play_item`. Request ids
are per connection; the host keeps them from colliding between clients.

Events go to every connected client, on track, play/pause, volume or like changes, on seek, and every 15s
while playing:
```json
{"event": "state", "state": {...}}
```

State:
```json
{
  "title": "Child Song",
  "artist": "The Cinematic Orchestra",
  "album": "Ma Fleur",
  "artworkUrl": "https://...",
  "state": "playing",
  "position": 94.3,
  "duration": 314,
  "at": 1793150081982,
  "volume": 100,
  "liked": false,
  "shuffle": false,
  "autoplay": true,
  "stopAfter": false,
  "queue": [
    {"title": "There, There", "artist": "Radiohead", "duration": 324, "current": false},
    {"title": "Child Song", "artist": "The Cinematic Orchestra", "duration": 314, "current": true}
  ]
}
```

- `state` is one of `playing`, `paused`, `buffering`, `ended`.
- `at` is when `position` was read (epoch ms). While playing, current position is
  `position + (now - at) / 1000`.
- `liked`, `shuffle` and `autoplay` are `null` if their control can't be found.
- `stopAfter` indicates whether pausing at the end of the current track is enabled.
- `queue` is the up next list as YouTube Music shows it, including tracks already played; `current` marks the
  one playing. Autoplay suggestions aren't included.
- There's no track ID: YouTube Music doesn't expose one in the page. Identify tracks by title, artist and album.
