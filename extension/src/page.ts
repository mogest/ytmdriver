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

const playing = () => document.querySelector('ytmusic-miniplayer button[aria-label="Pause"]') !== null
const position = () => Number(slider('Seek slider').value)

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

const text = (el: Element, selector: string) => el.querySelector(selector)?.textContent?.trim() ?? ''

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
}

async function libraryElements(kind: unknown) {
  const source = library[String(kind)]
  if (!source) throw new Error(`unknown library kind: ${kind}`)
  await source.show?.()
  return { source, elements: source.items() }
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
  shuffle: () => find<HTMLElement>(SHUFFLE).click(),
  autoplay: () => find<HTMLElement>(AUTOPLAY).click(),
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
    const items: LibraryItem[] = elements.map(el => ({ key: source.key(el), title: text(el, '.title'), subtitle: text(el, '.subtitle') }))
    return { items }
  },
  play_item: async ({ kind, key }) => {
    const { source, elements } = await libraryElements(kind)
    const item = elements.find(el => source.key(el) === key)
    if (!item) throw new Error(`not found in ${kind}: ${key}`)
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
