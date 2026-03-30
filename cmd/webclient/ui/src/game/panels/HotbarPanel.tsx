// HotbarPanel renders the 10 hotbar slots (keys 1–9, 0) and executes the bound
// command when a slot is clicked.
import { useGame } from '../GameContext'

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '0']

// Commands that require a target argument. When fired from the hotbar during
// combat and no argument is present, the first NPC in turn order is appended.
const COMBAT_TARGET_CMDS = new Set([
  'attack', 'att', 'kill',
  'strike', 'st',
  'burst', 'bf',
])

export function HotbarPanel() {
  const { state, sendCommand } = useGame()
  const { hotbarSlots, combatRound, characterInfo } = state

  function activate(idx: number) {
    let cmd = hotbarSlots[idx]
    if (!cmd) return

    // Auto-fill combat target when the command has no argument and we're in combat.
    if (combatRound) {
      const words = cmd.trim().split(/\s+/)
      const verb = words[0].toLowerCase()
      if (words.length === 1 && COMBAT_TARGET_CMDS.has(verb)) {
        const playerName = characterInfo?.name ?? ''
        const turnOrder = combatRound.turnOrder ?? combatRound.turn_order ?? []
        const target = turnOrder.find((n) => n !== playerName)
        if (target) {
          cmd = `${cmd} ${target}`
        }
      }
    }

    sendCommand(cmd)
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
