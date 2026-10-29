// Everything that knows YouTube Music's DOM lives here.

export interface State {
  title: string | null
  artist: string | null
  album: string | null
  artworkUrl: string | null
  state: 'playing' | 'paused' | 'buffering' | 'ended'
  position: number
  duration: number
  at: number
  volume: number
  liked: boolean | null
  shuffle: boolean | null
  autoplay: boolean | null
  stopAfter: boolean
  queue: QueueItem[]
}

export interface QueueItem {
  title: string
  artist: string
  duration: number
  current: boolean
}

export interface LibraryItem {
  key: string
  title: string
  subtitle: string
}

function find<T extends Element>(selector: string): T {
  const el = document.querySelector<T>(selector)
  if (!el) throw new Error(`YouTube Music element not found: ${selector}`)
  return el
}

const button = (label: string) => find<HTMLButtonElement>(`ytmusic-miniplayer button[aria-label="${label}"]`)
const slider = (label: string) => find<HTMLInputElement>(`ytmusic-miniplayer input[aria-label="${label}"]`)

const SHUFFLE = 'ytmusic-miniplayer button[aria-label="Shuffle"]'
// The Autoplay switch is at the top of the queue.
const AUTOPLAY = 'tp-yt-paper-toggle-button#automix'

function pressed(selector: string) {
  const el = document.querySelector(selector)
  return el && el.getAttribute('aria-pressed') === 'true'
}

// An omitted state keeps the interactive player's toggle behaviour.
function setPressed(selector: string, enabled: unknown) {
  if (enabled !== undefined && typeof enabled !== 'boolean') throw new Error('enabled: expected a boolean')
  const el = find<HTMLElement>(selector)
  if (enabled === undefined || pressed(selector) !== enabled) el.click()
}

const playing = () => document.querySelector('ytmusic-miniplayer button[aria-label="Pause"]') !== null
const position = () => Number(slider('Seek slider').value)
const title = () => navigator.mediaSession.metadata?.title ?? null

// The title of the track to pause at the end of.
let stopAfter: string | null = null
// With gapless playback the next track starts without a media event, so the end is caught with timeupdate,
// which fires a few times a second. If the track changes another way, e.g. Next, the new one is paused once
// its Pause button appears.
document.addEventListener(
  'timeupdate',
  () => {
    if (stopAfter && playing() && (title() !== stopAfter || Number(slider('Seek slider').max) - position() < 1)) {
      stopAfter = null
      button('Pause').click()
    }
  },
  true,
)

// From this far into a track, YouTube Music's Previous button restarts the track instead of going back.
const PREVIOUS_RESTARTS_AFTER = 5

function setRange(label: string, value: unknown) {
  if (typeof value !== 'number' || !isFinite(value)) throw new Error(`${label}: expected a number, got ${value}`)
  const input = slider(label)
  input.value = String(value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

async function waitFor(condition: () => boolean, timeout = 2000) {
  for (let waited = 0; waited < timeout; waited += 50) {
    if (condition()) return
    await sleep(50)
  }
  throw new Error('YouTube Music did not respond')
}

// The queue stays in the page when the player page is closed. A wrapped item holds the song and video versions
// of a track; only the primary one is shown.
const queueItems = () =>
  document.querySelectorAll(
    '#queue #contents > ytmusic-player-queue-item, #queue #contents > ytmusic-playlist-panel-video-wrapper-renderer > #primary-renderer > ytmusic-player-queue-item',
  )

const seconds = (clock: string) => clock.split(':').reduce((total, part) => total * 60 + Number(part), 0)

const text = (el: Element, selector: string) => el.querySelector(selector)?.textContent?.trim().replace(/\s+/g, ' ') ?? ''

const guideEntries = () => [...document.querySelectorAll('ytmusic-guide-entry-renderer')]
const chipLink = (label: string) =>
  [...document.querySelectorAll('ytmusic-chip-cloud-chip-renderer')].find(c => text(c, 'a') === label)?.querySelector('a')
const albumCards = () => [...document.querySelectorAll('ytmusic-grid-renderer ytmusic-two-row-item-renderer')]
const albumsLoader = () => document.querySelector('ytmusic-grid-renderer yt-next-continuation')

const onAlbumsPage = () => location.pathname === '/library/albums'

// YouTube Music builds the album grid as the page draws, and a hidden tab doesn't draw, so a grid shown while
// hidden stops part way and has no play buttons. Whatever page the tab is on when its visibility changes gets drawn.
let albumsDrawn = false
document.addEventListener('visibilitychange', () => (albumsDrawn = onAlbumsPage()))

// Albums are only listed on the library page, so the tab is taken there the way a person would: the sidebar's
// Library entry, then the Albums chip. Both navigate within the page, so playback carries on.
async function showAlbums() {
  if (!onAlbumsPage()) {
    albumsDrawn = false
    await navigateToAlbums()
  }
  if (document.visibilityState === 'hidden' && !albumsDrawn) {
    throw new Error('albums only load while the YouTube Music tab is visible; switch to it once')
  }
  // The grid loads the next page of albums when its loader scrolls into view, and removes the loader after the last.
  for (let loader; (loader = albumsLoader()); ) {
    const count = albumCards().length
    loader.scrollIntoView()
    await waitFor(() => albumCards().length > count || !albumsLoader(), 5000)
  }
}

async function navigateToAlbums() {
  if (!location.pathname.startsWith('/library')) {
    const library = guideEntries().find(e => text(e, '.title') === 'Library')?.querySelector<HTMLElement>('tp-yt-paper-item')
    if (!library) throw new Error('YouTube Music element not found: sidebar Library entry')
    library.click()
    await waitFor(() => !!chipLink('Albums'), 5000)
  }
  const albums = chipLink('Albums')
  if (!albums) throw new Error('YouTube Music element not found: Albums chip')
  albums.click()
  await waitFor(() => onAlbumsPage() && albumCards().length > 0, 5000)
}

const SEARCH_RESULTS = 'ytmusic-tabbed-search-results-renderer'
const visible = (selector: string) => [...document.querySelectorAll(selector)].filter(el => el.checkVisibility())

// The results page reuses its elements for a new search, hiding those it doesn't need, e.g. the songs under an
// artist when the top result is a song.
const searchResults = () => visible(`${SEARCH_RESULTS} ytmusic-responsive-list-item-renderer`)

// Search filters and the chips that apply them.
const SEARCH_FILTERS: Record<string, string> = { songs: 'Songs', albums: 'Albums', videos: 'Videos', playlists: 'Community playlists' }
const chips = () => [...document.querySelectorAll(`${SEARCH_RESULTS} ytmusic-chip-cloud-chip-renderer`)]
const selectedChips = () => chips().filter(c => c.hasAttribute('is-selected')).map(c => text(c, 'a'))

async function search(query: unknown, filter: unknown) {
  if (typeof query !== 'string' || !query.trim()) throw new Error('query: expected some words')
  const label = filter ? SEARCH_FILTERS[String(filter)] : undefined
  if (filter && !label) throw new Error(`unknown search filter: ${filter}`)

  const input = find<HTMLInputElement>('ytmusic-search-box input')
  input.value = query.trim()
  input.dispatchEvent(new Event('input', { bubbles: true }))
  input.dispatchEvent(new KeyboardEvent('keypress', { key: 'Enter', keyCode: 13, bubbles: true }))
  // Searching clears the selected chip as the new results arrive.
  await waitFor(
    () => new URLSearchParams(location.search).get('q') === input.value && selectedChips().length === 0 && searchResults().length > 0,
    5000,
  )
  if (!label) return

  const chip = chips().find(c => text(c, 'a') === label)?.querySelector('a')
  if (!chip) throw new Error(`YouTube Music element not found: ${label} chip`)
  chip.click()
  // The chip shows as selected straight away; the filtered results are in once they replace the mixed page's
  // several sections with one.
  await waitFor(() => selectedChips().join() === label && visible(`${SEARCH_RESULTS} ytmusic-section-list-renderer > #contents > *`).length === 1, 5000)
}

const playlistOptions = () => visible('ytmusic-add-to-playlist-renderer ytmusic-playlist-add-to-option-renderer')
const menuOpen = () => visible('ytmusic-menu-popup-renderer').length > 0
// Each notification adds a toast, and old ones linger.
const toasts = () => document.querySelectorAll('tp-yt-paper-toast')
const latestToast = () => toasts()[toasts().length - 1]
const toastText = () => latestToast().textContent!.trim().replace(/\s+/g, ' ')

// A menu stays open behind the dialog it opens, and its items do nothing once used. Escape closes the frontmost
// menu or toast.
async function closeMenu() {
  for (let i = 0; i < 5 && menuOpen(); i++) {
    const focused = document.activeElement ?? document.body
    focused.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', keyCode: 27, bubbles: true, composed: true }))
    await sleep(300)
  }
}

// The player bar's Save to playlist opens a dialog of playlists the first time. After that, for another track, it
// saves straight to the last playlist used and offers Change, which undoes that and opens the dialog.
async function saveToPlaylist(playlist: unknown) {
  if (!title()) throw new Error('nothing playing')
  await closeMenu()
  button('MORE').click()
  const save = () => visible('ytmusic-menu-popup-renderer #items > *').find(i => text(i, 'a') === 'Save to playlist')?.querySelector('a')
  await waitFor(() => !!save())
  const saved = `Saved to ${playlist} Change`
  let count = toasts().length
  save()!.click()
  await waitFor(() => playlistOptions().length > 0 || toasts().length > count, 5000)
  if (!playlistOptions().length) {
    if (toastText() === saved) return closeMenu()
    const change = latestToast().querySelector<HTMLElement>('#action-button button')
    if (change?.textContent!.trim() !== 'Change') throw new Error(toastText())
    // Pressed straight away, Change doesn't undo the save; two seconds later it does.
    await sleep(2000)
    change.click()
    await waitFor(() => playlistOptions().length > 0, 5000)
  }
  const option = playlistOptions().find(o => text(o, '#title') === playlist)
  if (!option) {
    find<HTMLElement>('ytmusic-add-to-playlist-renderer button[aria-label="Dismiss"]').click()
    throw new Error(`can't save to playlist: ${playlist}`)
  }
  count = toasts().length
  option.querySelector<HTMLElement>('button')!.click()
  await waitFor(() => toasts().length > count, 5000)
  if (toastText() !== saved) throw new Error(toastText().replace(/ Add anyway$/, ''))
  await closeMenu()
}

// Playlists are listed in the sidebar on every page, so need no navigation. They have no link, only a name.
const library: Record<string, { show?: () => Promise<void>; items: () => Element[]; key: (el: Element) => string }> = {
  playlists: {
    items: () => guideEntries().filter(e => e.querySelector('ytmusic-play-button-renderer')),
    key: e => text(e, '.title'),
  },
  albums: {
    show: showAlbums,
    items: albumCards,
    key: card => card.querySelector('a')?.pathname ?? '',
  },
  search: {
    items: searchResults,
    key: result => result.querySelector('a')?.getAttribute('href') ?? '',
  },
}

async function libraryElements(kind: unknown) {
  const source = library[String(kind)]
  if (!source) throw new Error(`unknown library kind: ${kind}`)
  await source.show?.()
  return { source, elements: source.items() }
}

async function libraryElement(kind: unknown, key: unknown) {
  const { source, elements } = await libraryElements(kind)
  const item = elements.find(el => source.key(el) === key)
  if (!item) throw new Error(`not found in ${kind}: ${key}`)
  return item
}

// Adding to an empty queue starts playing it.
async function queueItem(kind: unknown, key: unknown) {
  const item = await libraryElement(kind, key)
  await closeMenu()
  const menu = item.querySelector<HTMLElement>('button[aria-label="Action menu"]')
  if (!menu) throw new Error(`YouTube Music element not found: menu for ${key}`)
  menu.click()
  const add = () => visible('ytmusic-menu-popup-renderer #items > *').find(i => i.textContent!.trim() === 'Add to queue') as HTMLElement | undefined
  await waitFor(() => !!add())
  const count = toasts().length
  add()!.click()
  await waitFor(() => toasts().length > count, 5000)
  if (!toastText().endsWith('added to queue')) throw new Error(toastText())
}

export const commands: Record<string, (args: Record<string, unknown>) => unknown> = {
  state: () => ({ state: readState() }),
  play: () => {
    if (!playing()) button('Play').click()
  },
  pause: () => {
    if (playing()) button('Pause').click()
  },
  toggle: () => button(playing() ? 'Pause' : 'Play').click(),
  shuffle: ({ enabled }) => setPressed(SHUFFLE, enabled),
  autoplay: ({ enabled }) => setPressed(AUTOPLAY, enabled),
  'stop-after': () => {
    stopAfter = stopAfter ? null : title()
  },
  next: () => button('Next').click(),
  previous: async () => {
    if (position() >= PREVIOUS_RESTARTS_AFTER) {
      button('Previous').click()
      await waitFor(() => position() < 1)
    }
    button('Previous').click()
  },
  restart: () => setRange('Seek slider', 0),
  seek: ({ seconds }) => setRange('Seek slider', seconds),
  volume: ({ level }) => setRange('Volume', level),
  library: async ({ kind }) => {
    const { source, elements } = await libraryElements(kind)
    const items: LibraryItem[] = elements.map(el => ({ key: source.key(el), title: text(el, '.title'), subtitle: text(el, '.subtitle, .secondary-flex-columns') }))
    return { items }
  },
  search: async ({ query, filter }) => {
    await search(query, filter)
    return commands.library({ kind: 'search' })
  },
  save_to_playlist: ({ playlist }) => saveToPlaylist(playlist),
  queue_item: ({ kind, key }) => queueItem(kind, key),
  play_item: async ({ kind, key }) => {
    const item = await libraryElement(kind, key)
    const playButton = () => item.querySelector<HTMLElement>('ytmusic-play-button-renderer')
    // The album grid only adds Play buttons to cards once the page has scrolled near them.
    if (!playButton()) {
      item.scrollIntoView({ block: 'center' })
      await waitFor(() => !!playButton()).catch(() => {})
    }
    const play = playButton()
    if (!play) throw new Error(`YouTube Music element not found: play button for ${key}`)
    play.click()
  },
}

export function readState(): State {
  const video = find<HTMLVideoElement>('video')
  // Not video.currentTime/duration: with gapless playback the video's timeline runs on across tracks.
  const seek = slider('Seek slider')
  const metadata = navigator.mediaSession.metadata
  return {
    title: metadata?.title ?? null,
    artist: metadata?.artist ?? null,
    album: metadata?.album ?? null,
    artworkUrl: metadata?.artwork.at(-1)?.src ?? null,
    state: video.ended ? 'ended' : video.paused ? 'paused' : video.readyState < 3 ? 'buffering' : 'playing',
    position: Number(seek.value),
    duration: Number(seek.max),
    at: Date.now(),
    volume: Math.round(video.volume * 100),
    liked: pressed('ytmusic-miniplayer button[aria-label^="like this video"]'),
    shuffle: pressed(SHUFFLE),
    autoplay: pressed(AUTOPLAY),
    stopAfter: stopAfter !== null,
    queue: [...queueItems()].map(item => ({
      title: text(item, '.song-title'),
      artist: text(item, '.byline'),
      duration: seconds(text(item, '.duration')),
      current: item.hasAttribute('selected'),
    })),
  }
}

// Media events don't bubble, but a capturing listener on the document still sees them.
export function onPlayerEvent(callback: () => void) {
  for (const type of ['play', 'pause', 'playing', 'waiting', 'ended', 'seeked', 'volumechange']) {
    document.addEventListener(type, callback, true)
  }
}
