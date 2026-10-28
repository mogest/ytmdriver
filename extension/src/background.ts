type Port = chrome.runtime.Port
type Command = { id: unknown; cmd: string; args?: object }
type Reply = (message: object) => void

const tabs: Port[] = []
let playingTab: Port | undefined
let native: Port | undefined
const pending = new Map<unknown, Reply>()

const target = () => (playingTab && tabs.includes(playingTab) ? playingTab : tabs.at(-1))

function emit(message: object) {
  if (native) native.postMessage(message)
  else console.log(message)
}

function command(request: Command, reply: Reply) {
  const tab = target()
  if (!tab) return reply({ id: request.id, ok: false, error: 'no YouTube Music tab open' })
  pending.set(request.id, reply)
  tab.postMessage(request)
}

function connectNative() {
  if (native) return
  const port = chrome.runtime.connectNative('com.mogest.ytc')
  port.onMessage.addListener(request => command(request, reply => port.postMessage(reply)))
  port.onDisconnect.addListener(() => {
    console.warn('native host disconnected:', chrome.runtime.lastError?.message)
    native = undefined
    if (tabs.length) setTimeout(connectNative, 5000)
  })
  native = port
}

chrome.runtime.onConnect.addListener(port => {
  tabs.push(port)
  port.onMessage.addListener(message => {
    if (message.event) {
      if (message.state.state === 'playing') playingTab = port
      if (port === target()) emit(message)
    } else {
      pending.get(message.id)?.(message)
      pending.delete(message.id)
    }
  })
  port.onDisconnect.addListener(() => {
    tabs.splice(tabs.indexOf(port), 1)
    if (tabs.length === 0) {
      native?.disconnect()
      native = undefined
    }
  })
  connectNative()
})

// For testing from the service worker console: ytc('next'), ytc('seek', {seconds: 60})
let debugId = 0
Object.assign(globalThis, {
  ytc: (cmd: string, args?: object) => command({ id: `debug${++debugId}`, cmd, args }, console.log),
})
