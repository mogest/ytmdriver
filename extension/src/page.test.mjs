import assert from 'node:assert/strict'
import { test } from 'node:test'

let control
globalThis.document = {
  addEventListener() {},
  querySelector: () => control,
}
const { commands } = await import('./page.ts')

for (const name of ['shuffle', 'autoplay']) {
  test(`${name} sets either state without toggling repeated requests`, () => {
    for (const initial of [false, true]) {
      for (const enabled of [false, true]) {
        let state = initial
        let clicks = 0
        control = {
          getAttribute: () => String(state),
          click() { state = !state; clicks++ },
        }
        commands[name]({ enabled })
        commands[name]({ enabled })
        assert.equal(state, enabled)
        assert.equal(clicks, initial === enabled ? 0 : 1)
        commands[name]({})
        assert.equal(state, !enabled)
        assert.throws(() => commands[name]({ enabled: 'false' }), /expected a boolean/)
      }
    }
  })

  test(`${name} fails when the control is missing`, () => {
    control = null
    for (const enabled of [false, true]) {
      assert.throws(() => commands[name]({ enabled }), /element not found/)
    }
  })
}
