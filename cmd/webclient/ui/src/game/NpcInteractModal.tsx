// NpcInteractModal renders modals for non-combat NPC interactions:
// HealerModal, TrainerModal, TechTrainerModal, FixerModal, BankerModal, and GenericNpcModal.
import { useState } from 'react'
import { useGame } from './GameContext'
import type { HealerView, TrainerView, TechTrainerView, TechOfferEntry, FixerView, JobOfferEntry } from '../proto'

// ---------- Healer Modal ----------

function HealerModal({ view, onClose }: { view: HealerView; onClose: () => void }) {
  const { sendMessage } = useGame()
  const npcName = view.npcName ?? view.npc_name ?? 'Healer'
  const pricePerHp = view.pricePerHp ?? view.price_per_hp ?? 0
  const missingHp = view.missingHp ?? view.missing_hp ?? 0
  const fullHealCost = view.fullHealCost ?? view.full_heal_cost ?? 0
  const capacityRemaining = view.capacityRemaining ?? view.capacity_remaining ?? 0
  const playerCurrency = view.playerCurrency ?? view.player_currency ?? 0
  const currentHp = view.currentHp ?? view.current_hp ?? 0
  const maxHp = view.maxHp ?? view.max_hp ?? 0

  const canAffordFull = playerCurrency >= fullHealCost
  const atFullHp = missingHp === 0
  const noCapacity = capacityRemaining === 0

  function handleFullHeal() {
    sendMessage('HealRequest', { npc_name: npcName })
    onClose()
  }

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <h3 style={styles.title}>{npcName}</h3>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          <div style={styles.infoGrid}>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>HP</span>
              <span style={styles.infoValue}>{currentHp} / {maxHp}</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Price per HP</span>
              <span style={styles.infoValue}>{pricePerHp} Crypto</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Your Crypto</span>
              <span style={styles.infoValue}>{playerCurrency} Crypto</span>
            </div>
            {capacityRemaining > 0 && (
              <div style={styles.infoRow}>
                <span style={styles.infoLabel}>Healer capacity</span>
                <span style={styles.infoValue}>{capacityRemaining} HP remaining today</span>
              </div>
            )}
          </div>
          {atFullHp ? (
            <p style={styles.notice}>You are already at full health.</p>
          ) : noCapacity ? (
            <p style={styles.notice}>{npcName} has exhausted their daily healing capacity.</p>
          ) : (
            <div style={styles.actions}>
              <button
                style={{ ...styles.actionBtn, ...(canAffordFull ? styles.actionBtnGreen : styles.actionBtnDisabled) }}
                onClick={handleFullHeal}
                disabled={!canAffordFull}
                type="button"
              >
                Heal to full — {fullHealCost} Crypto
              </button>
              {!canAffordFull && (
                <p style={styles.notice}>You need {fullHealCost} Crypto but only have {playerCurrency} Crypto.</p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ---------- Trainer Modal ----------

function JobRow({ job, playerCurrency, onTrain }: {
  job: JobOfferEntry
  playerCurrency: number
  onTrain: (jobId: string) => void
}) {
  const jobId = job.jobId ?? job.job_id ?? ''
  const jobName = job.jobName ?? job.job_name ?? jobId
  const cost = job.trainingCost ?? job.training_cost ?? 0
  const available = job.available ?? false
  const alreadyTrained = job.alreadyTrained ?? job.already_trained ?? false
  const reason = job.unavailableReason ?? job.unavailable_reason ?? ''
  const canAfford = playerCurrency >= cost

  return (
    <tr style={styles.tableRow}>
      <td style={styles.tdName}>
        <div style={styles.jobNameCell}>
          {jobName}
          {alreadyTrained && <span style={styles.trainedBadge}>trained</span>}
        </div>
        {!available && !alreadyTrained && reason && (
          <div style={styles.reason}>{reason}</div>
        )}
      </td>
      <td style={{ ...styles.tdNum, color: canAfford ? '#aaa' : '#665' }}>{cost} Crypto</td>
      <td style={styles.tdAction}>
        {available && !alreadyTrained && (
          <button
            style={{ ...styles.trainBtn, ...(canAfford ? {} : styles.trainBtnDisabled) }}
            onClick={() => onTrain(jobId)}
            disabled={!canAfford}
            type="button"
            title={canAfford ? `Train ${jobName}` : `Need ${cost} Crypto`}
          >
            Train
          </button>
        )}
      </td>
    </tr>
  )
}

function TrainerModal({ view, onClose }: { view: TrainerView; onClose: () => void }) {
  const { sendMessage } = useGame()
  const npcName = view.npcName ?? view.npc_name ?? 'Trainer'
  const jobs = view.jobs ?? []
  const playerCurrency = view.playerCurrency ?? view.player_currency ?? 0

  function handleTrain(jobId: string) {
    sendMessage('TrainJobRequest', { npc_name: npcName, job_id: jobId })
    onClose()
  }

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{npcName}</h3>
            <span style={styles.currency}>{playerCurrency} Crypto</span>
          </div>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          {jobs.length === 0 ? (
            <p style={{ color: '#666' }}>No jobs available.</p>
          ) : (
            <table style={styles.table}>
              <thead>
                <tr>
                  <th style={{ ...styles.th, textAlign: 'left' }}>Job</th>
                  <th style={styles.th}>Cost</th>
                  <th style={styles.th}></th>
                </tr>
              </thead>
              <tbody>
                {jobs.map((job, i) => (
                  <JobRow
                    key={job.jobId ?? job.job_id ?? i}
                    job={job}
                    playerCurrency={playerCurrency}

                    onTrain={handleTrain}
                  />
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}

// ---------- Tech Trainer Modal ----------

function TechOfferRow({ offer, playerCurrency, onTrain }: {
  offer: TechOfferEntry
  playerCurrency: number
  onTrain: (techId: string) => void
}) {
  const techId = offer.techId ?? offer.tech_id ?? ''
  const techName = offer.techName ?? offer.tech_name ?? techId
  const techLevel = offer.techLevel ?? offer.tech_level ?? 0
  const cost = offer.cost ?? 0
  const canAfford = playerCurrency >= cost

  return (
    <tr style={styles.tableRow}>
      <td style={styles.tdName}>
        <div style={styles.jobNameCell}>{techName}</div>
        {offer.description && <div style={styles.reason}>{offer.description}</div>}
      </td>
      <td style={{ ...styles.tdNum, color: '#aaa' }}>L{techLevel}</td>
      <td style={{ ...styles.tdNum, color: canAfford ? '#aaa' : '#665' }}>{cost} Crypto</td>
      <td style={styles.tdAction}>
        <button
          style={{ ...styles.trainBtn, ...(canAfford ? {} : styles.trainBtnDisabled) }}
          onClick={() => onTrain(techId)}
          disabled={!canAfford}
          type="button"
          title={canAfford ? `Train ${techName}` : `Need ${cost} Crypto`}
        >
          Train
        </button>
      </td>
    </tr>
  )
}

function TechTrainerModal({ view, onClose }: { view: TechTrainerView; onClose: () => void }) {
  const { sendMessage } = useGame()
  const npcName = view.npcName ?? view.npc_name ?? 'Trainer'
  const tradition = view.tradition ?? ''
  const offers = view.offers ?? []
  const playerCurrency = view.playerCurrency ?? view.player_currency ?? 0

  function handleTrain(techId: string) {
    sendMessage('TrainTechRequest', { npc_name: npcName, tech_id: techId })
    onClose()
  }

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{npcName}</h3>
            {tradition && <span style={styles.npcTypeBadge}>{tradition}</span>}
            <span style={styles.currency}>{playerCurrency} Crypto</span>
          </div>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {offers.length === 0 ? (
            <p style={{ color: '#666', fontFamily: 'monospace', fontSize: '0.85rem' }}>
              You have no pending technology slots for this tradition.
            </p>
          ) : (
            <table style={styles.table}>
              <thead>
                <tr>
                  <th style={{ ...styles.th, textAlign: 'left' }}>Technology</th>
                  <th style={styles.th}>Lvl</th>
                  <th style={styles.th}>Cost</th>
                  <th style={styles.th}></th>
                </tr>
              </thead>
              <tbody>
                {offers.map((offer, i) => (
                  <TechOfferRow
                    key={offer.techId ?? offer.tech_id ?? i}
                    offer={offer}
                    playerCurrency={playerCurrency}
                    onTrain={handleTrain}
                  />
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}

// ---------- Fixer Modal ----------

function FixerModal({ view, onClose }: { view: FixerView; onClose: () => void }) {
  const { sendMessage } = useGame()
  const npcName = view.npcName ?? view.npc_name ?? 'Fixer'
  const currentWanted = view.currentWanted ?? view.current_wanted ?? 0
  const playerCurrency = view.playerCurrency ?? view.player_currency ?? 0
  const bribeCosts = view.bribeCosts ?? view.bribe_costs ?? {}

  function handleBribe(level: number) {
    sendMessage('BribeRequest', { npc_name: npcName, level })
    onClose()
  }

  const clearableLevels = Object.keys(bribeCosts)
    .map(Number)
    .filter((lvl) => lvl >= 1 && lvl <= currentWanted)
    .sort((a, b) => a - b)

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{npcName}</h3>
            <span style={styles.npcTypeBadge}>Fixer</span>
          </div>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          <div style={styles.infoGrid}>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Wanted Level</span>
              <span style={styles.infoValue}>{currentWanted}</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Your Crypto</span>
              <span style={styles.infoValue}>{playerCurrency} Crypto</span>
            </div>
          </div>
          {currentWanted === 0 ? (
            <p style={styles.notice}>You are not wanted here. There is nothing to clear.</p>
          ) : clearableLevels.length === 0 ? (
            <p style={styles.notice}>{npcName} cannot help you at your current wanted level.</p>
          ) : (
            <div style={styles.actions}>
              {clearableLevels.map((level) => {
                const cost = bribeCosts[level] ?? 0
                const canAfford = playerCurrency >= cost
                return (
                  <button
                    key={level}
                    style={{ ...styles.actionBtn, ...(canAfford ? styles.actionBtnGreen : styles.actionBtnDisabled) }}
                    onClick={() => handleBribe(level)}
                    disabled={!canAfford}
                    type="button"
                  >
                    Clear wanted level {level} — {cost} Crypto
                  </button>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ---------- Banker Modal ----------

function BankerModal({
  view,
  onClose,
}: {
  view: { name: string; description: string; npcType: string; level: number; health: string }
  onClose: () => void
}) {
  const { sendMessage, state } = useGame()
  const [amount, setAmount] = useState('')
  const playerCurrency = state.characterSheet?.currency ?? state.inventoryView?.currency ?? null

  function handleDeposit() {
    const n = parseInt(amount, 10)
    if (!n || n <= 0) return
    sendMessage('StashDepositRequest', { npc_name: view.name, amount: n })
    setAmount('')
    onClose()
  }

  function handleWithdraw() {
    const n = parseInt(amount, 10)
    if (!n || n <= 0) return
    sendMessage('StashWithdrawRequest', { npc_name: view.name, amount: n })
    setAmount('')
    onClose()
  }

  function handleBalance() {
    sendMessage('StashBalanceRequest', { npc_name: view.name })
    onClose()
  }

  const amountNum = parseInt(amount, 10)
  const validAmount = !isNaN(amountNum) && amountNum > 0

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={{ ...styles.modal, maxWidth: '440px' }} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{view.name}</h3>
            <span style={styles.npcTypeBadge}>Banker</span>
          </div>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          {playerCurrency !== null && (
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Carried Crypto</span>
              <span style={styles.infoValue}>{playerCurrency}</span>
            </div>
          )}
          <div style={styles.bankerSection}>
            <input
              style={styles.bankerInput}
              type="number"
              min="1"
              placeholder="Amount"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && validAmount) handleDeposit() }}
            />
            <div style={styles.bankerBtns}>
              <button
                style={{ ...styles.actionBtn, ...(validAmount ? styles.actionBtnGreen : styles.actionBtnDisabled) }}
                onClick={handleDeposit}
                disabled={!validAmount}
                type="button"
              >
                Deposit
              </button>
              <button
                style={{ ...styles.actionBtn, ...(validAmount ? styles.actionBtnBlue : styles.actionBtnDisabled) }}
                onClick={handleWithdraw}
                disabled={!validAmount}
                type="button"
              >
                Withdraw
              </button>
            </div>
            <button style={{ ...styles.actionBtn, ...styles.actionBtnGray }} onClick={handleBalance} type="button">
              Check Stash Balance
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ---------- Hire Modal (hireling NPC) ----------

function HireModal({
  view,
  onClose,
}: {
  view: { name: string; description: string; npcType: string; level: number; health: string }
  onClose: () => void
}) {
  const { sendMessage } = useGame()

  function handleHire() {
    sendMessage('HireRequest', { npc_name: view.name })
    onClose()
  }

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={{ ...styles.modal, maxWidth: '480px' }} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{view.name}</h3>
            <span style={styles.npcTypeBadge}>Hireling</span>
          </div>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          <div style={styles.infoRow}>
            <span style={styles.infoLabel}>Level</span>
            <span style={styles.infoValue}>{view.level}</span>
          </div>
          {view.health && (
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Condition</span>
              <span style={styles.infoValue}>{view.health}</span>
            </div>
          )}
          <div style={{ ...styles.actions, marginTop: '0.75rem' }}>
            <button
              style={{ ...styles.actionBtn, ...styles.actionBtnGreen }}
              onClick={handleHire}
              type="button"
            >
              Hire
            </button>
            <button
              style={{ ...styles.actionBtn, background: '#444' }}
              onClick={onClose}
              type="button"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ---------- Generic NPC Modal ----------

const NPC_TYPE_LABELS: Record<string, string> = {
  quest_giver: 'Quest Giver',
  banker: 'Banker',
  fixer: 'Fixer',
  hireling: 'Hireling',
  crafter: 'Crafter',
  chip_doc: 'Chip Doc',
  guard: 'Guard',
}

function GenericNpcModal({
  view,
  onClose,
}: {
  view: { name: string; description: string; npcType: string; level: number; health: string }
  onClose: () => void
}) {
  const typeLabel = NPC_TYPE_LABELS[view.npcType] ?? view.npcType.replace(/_/g, ' ')

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={{ ...styles.modal, maxWidth: '480px' }} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <div style={styles.headerLeft}>
            <h3 style={styles.title}>{view.name}</h3>
            {typeLabel && <span style={styles.npcTypeBadge}>{typeLabel}</span>}
          </div>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          <div style={styles.infoRow}>
            <span style={styles.infoLabel}>Level</span>
            <span style={styles.infoValue}>{view.level}</span>
          </div>
          {view.health && (
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Condition</span>
              <span style={styles.infoValue}>{view.health}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ---------- Rest Modal (motel_keeper / brothel_keeper) ----------

function RestModal({ view, onClose }: { view: import('../proto').RestView; onClose: () => void }) {
  const { sendMessage } = useGame()
  const npcName = view.npcName ?? view.npc_name ?? 'Keeper'
  const restCost = view.restCost ?? view.rest_cost ?? 0
  const playerCurrency = view.playerCurrency ?? view.player_currency ?? 0
  const currentHp = view.currentHp ?? view.current_hp ?? 0
  const maxHp = view.maxHp ?? view.max_hp ?? 0

  const atFullHp = currentHp >= maxHp
  const canAfford = playerCurrency >= restCost

  function handleRest() {
    sendMessage('RestRequest', {})
    onClose()
  }

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.header}>
          <h3 style={styles.title}>{npcName}</h3>
          <button style={styles.closeBtn} onClick={onClose} type="button">✕</button>
        </div>
        <div style={styles.body}>
          {view.description && <p style={styles.desc}>{view.description}</p>}
          <div style={styles.infoGrid}>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>HP</span>
              <span style={styles.infoValue}>{currentHp} / {maxHp}</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Cost</span>
              <span style={styles.infoValue}>{restCost} Crypto</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Your Crypto</span>
              <span style={styles.infoValue}>{playerCurrency} Crypto</span>
            </div>
          </div>
          {atFullHp && (
            <p style={{ color: '#8b8', marginTop: '0.5rem', fontSize: '0.85rem' }}>
              You are already at full health.
            </p>
          )}
          {!canAfford && (
            <p style={{ color: '#f88', marginTop: '0.5rem', fontSize: '0.85rem' }}>
              You cannot afford to rest here.
            </p>
          )}
          <div style={{ ...styles.actions, marginTop: '0.75rem' }}>
            <button
              style={{ ...styles.actionBtn, ...styles.actionBtnGreen, opacity: canAfford ? 1 : 0.4 }}
              onClick={handleRest}
              disabled={!canAfford}
              type="button"
            >
              Rest ({restCost} Crypto)
            </button>
            <button style={{ ...styles.actionBtn, background: '#444' }} onClick={onClose} type="button">
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

// ---------- Root export ----------

export function NpcInteractModal() {
  const { state, clearHealer, clearTrainer, clearTechTrainer, clearFixer, clearRestView, clearNpcView } = useGame()

  if (state.healerView) {
    return <HealerModal view={state.healerView} onClose={clearHealer} />
  }
  if (state.trainerView) {
    return <TrainerModal view={state.trainerView} onClose={clearTrainer} />
  }
  if (state.techTrainerView) {
    return <TechTrainerModal view={state.techTrainerView} onClose={clearTechTrainer} />
  }
  if (state.fixerView) {
    return <FixerModal view={state.fixerView} onClose={clearFixer} />
  }
  if (state.restView) {
    return <RestModal view={state.restView} onClose={clearRestView} />
  }
  if (state.npcView) {
    if (state.npcView.npcType === 'banker') {
      return <BankerModal view={state.npcView} onClose={clearNpcView} />
    }
    if (state.npcView.npcType === 'hireling') {
      return <HireModal view={state.npcView} onClose={clearNpcView} />
    }
    return <GenericNpcModal view={state.npcView} onClose={clearNpcView} />
  }
  return null
}

// ---------- Styles ----------

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.75)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 200,
  },
  modal: {
    background: '#111',
    border: '1px solid #333',
    borderRadius: '6px',
    width: 'min(560px, 95vw)',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0.6rem 1rem',
    borderBottom: '1px solid #2a2a2a',
    flexShrink: 0,
  },
  headerLeft: {
    display: 'flex',
    alignItems: 'baseline',
    gap: '0.6rem',
  },
  title: {
    margin: 0,
    color: '#e0c060',
    fontSize: '1rem',
    fontFamily: 'monospace',
  },
  currency: {
    color: '#7bc',
    fontSize: '0.8rem',
    fontFamily: 'monospace',
  },
  npcTypeBadge: {
    fontSize: '0.65rem',
    padding: '0.1rem 0.4rem',
    borderRadius: '3px',
    background: '#1a1a2a',
    border: '1px solid #3a3a5a',
    color: '#778',
    fontFamily: 'monospace',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.05em',
  },
  closeBtn: {
    background: 'none',
    border: '1px solid #444',
    color: '#888',
    cursor: 'pointer',
    fontFamily: 'monospace',
    borderRadius: '3px',
    padding: '0.1rem 0.4rem',
  },
  body: {
    padding: '0.75rem 1rem',
    overflowY: 'auto',
  },
  desc: {
    color: '#aaa',
    fontSize: '0.85rem',
    fontFamily: 'monospace',
    lineHeight: 1.5,
    margin: '0 0 0.75rem',
  },
  infoGrid: {
    marginBottom: '0.75rem',
  },
  infoRow: {
    display: 'flex',
    justifyContent: 'space-between',
    padding: '0.2rem 0',
    fontSize: '0.82rem',
    fontFamily: 'monospace',
    borderBottom: '1px solid #1a1a1a',
  },
  infoLabel: { color: '#7af' },
  infoValue: { color: '#ccc' },
  notice: {
    color: '#666',
    fontSize: '0.8rem',
    fontFamily: 'monospace',
    margin: '0.5rem 0 0',
  },
  actions: {
    marginTop: '0.75rem',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '0.4rem',
  },
  actionBtn: {
    padding: '0.35rem 0.75rem',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    border: '1px solid transparent',
  },
  actionBtnGreen: {
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
  },
  actionBtnDisabled: {
    background: '#1a1a1a',
    border: '1px solid #333',
    color: '#555',
    cursor: 'not-allowed',
  },
  // Trainer table
  table: {
    width: '100%',
    borderCollapse: 'collapse',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
  },
  th: {
    color: '#7af',
    fontSize: '0.72rem',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.06em',
    padding: '0.3rem 0.5rem',
    borderBottom: '1px solid #2a2a2a',
    textAlign: 'right' as const,
  },
  tableRow: {
    borderBottom: '1px solid #1a1a1a',
  },
  tdName: {
    color: '#ddd',
    padding: '0.35rem 0.5rem',
    textAlign: 'left' as const,
  },
  tdNum: {
    padding: '0.35rem 0.5rem',
    textAlign: 'right' as const,
    whiteSpace: 'nowrap' as const,
  },
  tdAction: {
    padding: '0.25rem 0.5rem',
    textAlign: 'center' as const,
  },
  jobNameCell: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.4rem',
  },
  trainedBadge: {
    fontSize: '0.62rem',
    padding: '0.08rem 0.35rem',
    borderRadius: '3px',
    background: '#1a2a3a',
    border: '1px solid #2a4a6a',
    color: '#7bc',
  },
  reason: {
    color: '#555',
    fontSize: '0.72rem',
    marginTop: '0.1rem',
  },
  trainBtn: {
    padding: '0.15rem 0.5rem',
    background: '#1a2a1a',
    border: '1px solid #4a6a2a',
    color: '#8d4',
    borderRadius: '3px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  trainBtnDisabled: {
    background: '#1a1a1a',
    border: '1px solid #333',
    color: '#555',
    cursor: 'not-allowed',
  },
  bankerSection: {
    marginTop: '0.75rem',
    display: 'flex',
    flexDirection: 'column' as const,
    gap: '0.5rem',
  },
  bankerInput: {
    background: '#0d0d0d',
    border: '1px solid #333',
    color: '#ccc',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    padding: '0.3rem 0.5rem',
    borderRadius: '3px',
    width: '100%',
    boxSizing: 'border-box' as const,
  },
  bankerBtns: {
    display: 'flex',
    gap: '0.4rem',
  },
  actionBtnBlue: {
    background: '#1a1a2a',
    border: '1px solid #2a4a8a',
    color: '#7af',
  },
  actionBtnOrange: {
    background: '#2a1a0a',
    border: '1px solid #8a4a1a',
    color: '#c84',
  },
  actionBtnGray: {
    background: '#151515',
    border: '1px solid #333',
    color: '#888',
  },
}
