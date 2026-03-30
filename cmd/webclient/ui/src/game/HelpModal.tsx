import type { CSSProperties } from 'react'

interface HelpSection {
  title: string
  commands: Array<{ cmd: string; aliases?: string; desc: string }>
}

const HELP_SECTIONS: HelpSection[] = [
  {
    title: 'Movement',
    commands: [
      { cmd: 'north / south / east / west', aliases: 'n s e w', desc: 'Move in a cardinal direction' },
      { cmd: 'northeast / northwest / southeast / southwest', aliases: 'ne nw se sw', desc: 'Move diagonally' },
      { cmd: 'up / down', aliases: 'u d', desc: 'Move vertically' },
    ],
  },
  {
    title: 'World',
    commands: [
      { cmd: 'look', aliases: 'l', desc: 'Look around the current room' },
      { cmd: 'exits', desc: 'List available exits' },
      { cmd: 'examine <target>', aliases: 'ex', desc: 'Examine an NPC or object' },
      { cmd: 'inventory', aliases: 'inv i', desc: 'Show backpack contents and currency' },
      { cmd: 'get <item>', aliases: 'take', desc: 'Pick up item from room floor' },
      { cmd: 'drop <item>', desc: 'Drop an item from your backpack' },
      { cmd: 'balance', aliases: 'bal', desc: 'Show your currency' },
      { cmd: 'char', aliases: 'sheet', desc: 'Display your character sheet' },
    ],
  },
  {
    title: 'Combat',
    commands: [
      { cmd: 'attack <target>', aliases: 'att kill', desc: 'Attack a target (starts combat)' },
      { cmd: 'strike <target>', aliases: 'st', desc: 'Full attack routine (2 AP, two hits)' },
      { cmd: 'burst <target>', aliases: 'bf', desc: 'Burst fire (2 AP, 2 attacks)' },
      { cmd: 'auto', aliases: 'af', desc: 'Automatic fire at all enemies (3 AP)' },
      { cmd: 'throw <item>', aliases: 'gr', desc: 'Throw an explosive' },
      { cmd: 'stride', aliases: 'str close move approach', desc: 'Close 25ft toward enemy (1 AP)' },
      { cmd: 'reload', aliases: 'rl', desc: 'Reload equipped weapon (1 AP)' },
      { cmd: 'pass', aliases: 'p', desc: 'Forfeit remaining action points' },
      { cmd: 'flee', aliases: 'run', desc: 'Attempt to flee combat' },
      { cmd: 'status', aliases: 'cond', desc: 'Show your active conditions' },
      { cmd: 'equip <weapon> [slot]', aliases: 'eq', desc: 'Equip a weapon' },
      { cmd: 'loadout [1|2]', aliases: 'lo prep kit', desc: 'Display or swap weapon presets' },
      { cmd: 'wear <item> <slot>', desc: 'Equip a piece of armor' },
      { cmd: 'remove <slot>', aliases: 'rem', desc: 'Remove equipped armor' },
    ],
  },
  {
    title: 'NPCs & Social',
    commands: [
      { cmd: 'talk <npc>', desc: 'Talk to a quest giver NPC' },
      { cmd: 'heal <npc>', desc: 'Ask a healer to restore HP' },
      { cmd: 'browse <npc>', desc: "Browse a merchant's inventory" },
      { cmd: 'buy <npc> <item> [qty]', desc: 'Buy an item from a merchant' },
      { cmd: 'sell <npc> <item> [qty]', desc: 'Sell an item to a merchant' },
      { cmd: 'negotiate <npc>', aliases: 'neg', desc: 'Negotiate prices with a merchant' },
      { cmd: 'hire <npc>', desc: 'Hire a hireling NPC' },
      { cmd: 'dismiss', desc: 'Dismiss your current hireling' },
    ],
  },
  {
    title: 'Character',
    commands: [
      { cmd: 'jobs', desc: 'List your current jobs' },
      { cmd: 'setjob <job>', desc: 'Set your active job' },
      { cmd: 'faction', desc: 'Show faction standing' },
    ],
  },
  {
    title: 'Communication',
    commands: [
      { cmd: 'say <message>', desc: 'Say something to the room' },
      { cmd: 'emote <action>', aliases: 'em', desc: 'Perform an emote action' },
      { cmd: 'who', desc: 'List players in the room' },
    ],
  },
  {
    title: 'System',
    commands: [
      { cmd: 'switch', desc: 'Switch to a different character' },
      { cmd: 'quit', aliases: 'exit', desc: 'Disconnect from the game' },
    ],
  },
]

interface HelpModalProps {
  onClose: () => void
}

export function HelpModal({ onClose }: HelpModalProps) {
  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <h2 style={styles.title}>Command Reference</h2>
          <button style={styles.closeBtn} onClick={onClose}>✕</button>
        </div>
        <div style={styles.body}>
          {HELP_SECTIONS.map((section) => (
            <div key={section.title} style={styles.section}>
              <h3 style={styles.sectionTitle}>{section.title}</h3>
              <table style={styles.table}>
                <tbody>
                  {section.commands.map((c) => (
                    <tr key={c.cmd}>
                      <td style={styles.cmdCell}>{c.cmd}</td>
                      {c.aliases
                        ? <td style={styles.aliasCell}>[{c.aliases}]</td>
                        : <td style={styles.aliasCell} />}
                      <td style={styles.descCell}>{c.desc}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.75)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 1000,
  },
  modal: {
    background: '#1a1a1a',
    border: '1px solid #444',
    borderRadius: '8px',
    width: 'min(820px, 95vw)',
    maxHeight: '85vh',
    display: 'flex',
    flexDirection: 'column',
    fontFamily: 'monospace',
    color: '#ccc',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '1rem 1.25rem 0.75rem',
    borderBottom: '1px solid #333',
  },
  title: {
    margin: 0,
    color: '#e0c060',
    fontSize: '1.1rem',
  },
  closeBtn: {
    background: 'none',
    border: 'none',
    color: '#888',
    cursor: 'pointer',
    fontSize: '1rem',
    padding: '0.25rem 0.5rem',
  },
  body: {
    overflow: 'auto',
    padding: '0.75rem 1.25rem 1.25rem',
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))',
    gap: '1rem',
    alignItems: 'start',
  },
  section: {
    background: '#111',
    border: '1px solid #2a2a2a',
    borderRadius: '6px',
    padding: '0.6rem 0.75rem',
  },
  sectionTitle: {
    margin: '0 0 0.4rem',
    color: '#7af',
    fontSize: '0.8rem',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.08em',
  },
  table: {
    width: '100%',
    borderCollapse: 'collapse',
    fontSize: '0.78rem',
  },
  cmdCell: {
    color: '#e0c060',
    paddingRight: '0.5rem',
    paddingBottom: '0.2rem',
    whiteSpace: 'nowrap' as const,
    verticalAlign: 'top',
  },
  aliasCell: {
    color: '#666',
    paddingRight: '0.5rem',
    paddingBottom: '0.2rem',
    whiteSpace: 'nowrap' as const,
    verticalAlign: 'top',
    fontSize: '0.72rem',
  },
  descCell: {
    color: '#aaa',
    paddingBottom: '0.2rem',
    verticalAlign: 'top',
  },
}
