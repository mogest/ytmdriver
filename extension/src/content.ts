import { commands, onPlayerEvent, readState, type State } from './page'

const PULSE_MS = 15_000
const DRIFT_SECONDS = 2

let port: chrome.runtime.Port
let last: State | undefined

// Chrome won't start media in a tab that hasn't been visible since it loaded.
const STARTS_MEDIA = ['play', 'toggle', 'seek', 'restart', 'next', 'previous', 'play_item']
let shown = document.visibilityState === 'visible'
document.addEventListener('visibilitychange', () => (shown ||= document.visibilityState === 'visible'))

function connect() {
  port = chrome.runtime.connect({ name: 'ytm' })
  port.onMessage.addListener(async request => port.postMessage(await handle(request)))
  // The background service worker can be restarted at any time; reconnecting wakes it.
  port.onDisconnect.addListener(() => setTimeout(connect, 1000))
  last = undefined
}

async function handle({ id, cmd, args }: { id: unknown; cmd: string; args?: Record<string, unknown> }) {
  try {
    const command = commands[cmd]
    if (!command) throw new Error(`unknown command: ${cmd}`)
    if (!shown && STARTS_MEDIA.includes(cmd)) {
      throw new Error("YouTube Music tab hasn't been shown since it loaded; switch to it once (Chrome restriction)")
    }
    const result = await command(args ?? {})
    return { id, ok: true, ...(result as object) }
  } catch (e) {
    return { id, ok: false, error: (e as Error).message }
  }
}

const withoutPosition = ({ position, at, ...rest }: State) => JSON.stringify(rest)

// Sends state when anything other than position changes, when position isn't where a client
// extrapolating from the last state would expect it (seeks, track changes), and periodically while playing.
function sendState() {
  let state
  try {
    state = readState()
  } catch {
    return
  }
  if (last) {
    const elapsed = last.state === 'playing' ? (state.at - last.at) / 1000 : 0
    const drifted = Math.abs(state.position - (last.position + elapsed)) > DRIFT_SECONDS
    const pulse = state.state === 'playing' && state.at - last.at >= PULSE_MS
    if (!drifted && !pulse && withoutPosition(state) === withoutPosition(last)) return
  }
  last = state
  port.postMessage({ event: 'state', state })
}

connect()
onPlayerEvent(sendState)
setInterval(sendState, 1000)
