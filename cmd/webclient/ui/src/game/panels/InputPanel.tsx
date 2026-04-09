import { type KeyboardEvent, useRef, useState } from 'react'
import { useGame } from '../GameContext'
import { useCommandHistory } from '../useCommandHistory'

const MOVE_RE = /^(n|s|e|w|north|south|east|west|up|down|northeast|northwest|southeast|southwest|ne|nw|se|sw)$/i

const DIRECTION_MAP: Record<string, string> = {
  n: 'north', s: 'south', e: 'east', w: 'west',
  ne: 'northeast', nw: 'northwest', se: 'southeast', sw: 'southwest',
  north: 'north', south: 'south', east: 'east', west: 'west',
  up: 'up', down: 'down',
  northeast: 'northeast', northwest: 'northwest',
  southeast: 'southeast', southwest: 'southwest',
}

// Short compass forms accepted by StepRequest/StrideRequest (no up/down on combat grid).
const COMBAT_DIR_MAP: Record<string, string> = {
  n: 'n', s: 's', e: 'e', w: 'w', ne: 'ne', nw: 'nw', se: 'se', sw: 'sw',
  north: 'n', south: 's', east: 'e', west: 'w',
  northeast: 'ne', northwest: 'nw', southeast: 'se', southwest: 'sw',
}

export function InputPanel() {
  const { state, sendMessage, sendCommand } = useGame()
  const [value, setValue] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const history = useCommandHistory()

  function submit(cmd: string) {
    const trimmed = cmd.trim()
    if (!trimmed) return
    history.push(trimmed)

    // Move shortcut dispatch.
    if (MOVE_RE.test(trimmed)) {
      const lower = trimmed.toLowerCase()
      const combatDir = COMBAT_DIR_MAP[lower]
      if (state.combatRound !== null && combatDir) {
        // In combat: compass shortcuts step 1 square on the combat grid.
        sendMessage('StepRequest', { direction: combatDir })
      } else {
        const dir = DIRECTION_MAP[lower] ?? lower
        sendMessage('MoveRequest', { direction: dir })
      }
    } else {
      sendCommand(trimmed)
    }

    setValue('')
    history.reset()
    inputRef.current?.focus()
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      setValue(history.navigateUp())
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      setValue(history.navigateDown())
    }
  }

  return (
    <form
      className="input-form"
      onSubmit={(e) => { e.preventDefault(); submit(value) }}
    >
      <input
        ref={inputRef}
        className="input-field"
        type="text"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={handleKeyDown}
        autoFocus
        placeholder="Enter command…"
        autoComplete="off"
        autoCorrect="off"
        spellCheck={false}
      />
      <button className="input-send" type="submit">Send</button>
    </form>
  )
}
