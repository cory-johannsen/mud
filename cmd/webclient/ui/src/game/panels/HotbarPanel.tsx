// HotbarPanel renders the 10 hotbar slots (keys 1–9, 0) and executes the bound
// command when a slot is clicked.
import { useGame } from '../GameContext'

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

export function HotbarPanel() {
  const { state, sendCommand } = useGame()
  const { hotbarSlots } = state

  function activate(idx: number) {
    const cmd = hotbarSlots[idx]
    if (cmd) sendCommand(cmd)
  }

  return (
    <div className="hotbar">
      {KEYS.map((key, i) => {
        const label = hotbarSlots[i] ?? ''
        return (
          <button
            key={key}
            className={`hotbar-slot${label ? '' : ' hotbar-slot-empty'}`}
            onClick={() => activate(i)}
            disabled={!label}
            title={label || undefined}
            type="button"
          >
            <span className="hotbar-key">{key}</span>
            <span className="hotbar-label">{label || '—'}</span>
          </button>
        )
      })}
    </div>
  )
}
