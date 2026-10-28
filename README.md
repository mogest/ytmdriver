# ytmdriver

Control YouTube Music, playing in your normal browser, from the terminal.

```
ytm ─unix socket─ ytm host ─native messaging─ extension ─DOM─ music.youtube.com tab
```

The browser does all the playback. The extension operates the player bar's own buttons and sliders, as a person
would, and reads the now-playing state from the page. It doesn't block ads, fetch streams or call YouTube's
internal JavaScript.

Currently Chrome only, on macOS and Linux.

## Setup

Requires Go and Node.

1. Build the extension:
   ```sh
   cd extension && npm install && npm run build
   ```
2. In Chrome, open `chrome://extensions`, turn on Developer mode, click "Load unpacked" and choose
   `extension/dist`. The extension ID is always `bidbhfllonlbjabgdemkdibgdillegmd` (fixed by `key` in the
   manifest).
3. Build `ytm` and register it as the native messaging host:
   ```sh
   go build -o bin/ytm ./cmd/ytm && ./bin/ytm install
   ```
   This writes `com.seriousorange.ytmdriver.json` into Chrome's `NativeMessagingHosts` directory, pointing at the binary's
   absolute path. Re-run it if you move the binary.
4. Open (or reload) a YouTube Music tab. Chrome starts `ytm` in host mode.

## Usage

`ytm` on its own opens the player:

```
 ▶ Child Song                                          ♡
   The Cinematic Orchestra · Ma Fleur

   1:34 ━━━━━━━━━━━━━━━━━━──────────────────────── 5:14
   vol ━━━━━━━━━━━━━━━━━━━━ 100

 space play/pause · j/k next/prev · h/l seek · 0-9 jump · -/+ vol · b library · q quit
```

h/l (or ←/→) seek 10s, -/+ change volume by 5; n/p and ↓/↑ also work for next/previous. It reconnects by itself when
a YouTube Music tab appears; with none open, space opens one.

`t` cycles through colour themes (terminal, youtube, nord, gruvbox, dracula). The choice is saved in `ytm/theme`
under your user config directory.

`b` opens your library: playlists and albums, with h/l to switch between them, j/k to move, enter to play,
/ to filter, esc to go back.

Commands for scripting:

```
ytm play | pause | toggle | next | previous | restart
ytm shuffle | autoplay           # toggle
ytm seek <seconds>
ytm volume <0-100>
ytm state      # current state as JSON
ytm watch      # current state, then one JSON line per change
ytm playlists | albums           # list, tab-separated title and subtitle
ytm playlist | album <name>      # play; any words that pick out one item, e.g. `ytm album djesse 4`
```

Listing or playing albums takes the YouTube Music tab to Library → Albums, since that's the only place albums
are listed. Playback isn't affected. Playlists are read from the sidebar, so the tab stays where it is.

A hidden tab (in the background, or its window minimised or covered) doesn't draw, and YouTube Music only builds
the album grid as it draws. So if the tab gets to Albums while hidden, album commands fail until you've looked
at it once. It stays on Albums afterwards, so after that they work in the background.

With no YouTube Music tab open, `ytm play` opens https://music.youtube.com in your default browser; other
commands fail with `no YouTube Music tab open`.

Chrome won't start media in a tab that hasn't been visible since it loaded, e.g. one restored in the background
after a browser restart. Until you switch to it once, the commands that start playback fail
saying so.

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
```

Commands: `play`, `pause`, `toggle`, `next`, `previous`, `restart`, `shuffle`, `autoplay`, `seek {seconds}`, `volume {level}`,
`state`, `library {kind}`, `play_item {kind, key}`. `kind` is `playlists` or `albums`.

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
state, and only `library` returns items. An item's `key` is opaque: pass it back to `play_item`. Request ids
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
  "queue": [
    {"title": "There, There", "artist": "Radiohead", "duration": 324, "current": false},
    {"title": "Child Song", "artist": "The Cinematic Orchestra", "duration": 314, "current": true}
  ]
}
```

- `state` is one of `playing`, `paused`, `buffering`, `ended`.
- `at` is when `position` was read (epoch ms). While playing, current position is
  `position + (now - at) / 1000`.
- `liked`, `shuffle` and `autoplay` are `null` if their control can't be found. The `shuffle` and `autoplay`
  commands toggle them.
- `queue` is the up next list as YouTube Music shows it, including tracks already played; `current` marks the
  one playing. Autoplay suggestions aren't included.
- There's no track ID: YouTube Music doesn't expose one in the page. Identify tracks by title, artist and album.
