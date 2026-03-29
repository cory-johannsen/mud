import {
  type ChangeEvent,
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react'
import { api, type CharacterOptions, type CharacterOption, ApiError } from '../api/client'

interface WizardState {
  region: string
  job: string
  archetype: string
  name: string
  gender: string
}

const EMPTY_STATE: WizardState = { region: '', job: '', archetype: '', name: '', gender: '' }
const STEPS = ['Region', 'Job', 'Archetype', 'Name & Gender'] as const
type StepIndex = 0 | 1 | 2 | 3

interface Props {
  onComplete: () => void
  onCancel: () => void
}

export function CharacterWizard({ onComplete, onCancel }: Props) {
  const [step, setStep] = useState<StepIndex>(0)
  const [state, setState] = useState<WizardState>(EMPTY_STATE)
  const [options, setOptions] = useState<CharacterOptions | null>(null)
  const [optionsError, setOptionsError] = useState<string | null>(null)
  const [nameAvailable, setNameAvailable] = useState<boolean | null>(null)
  const [nameChecking, setNameChecking] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    api.characters.options()
      .then((opts) => setOptions(opts))
      .catch(() => setOptionsError('Failed to load character options.'))
  }, [])

  useEffect(() => {
    if (step !== 3 || state.name.length < 3) {
      setNameAvailable(null)
      return
    }
    if (debounceRef.current) clearTimeout(debounceRef.current)
    setNameChecking(true)
    debounceRef.current = setTimeout(() => {
      api.characters.checkName(state.name)
        .then((res) => setNameAvailable(res.available))
        .catch(() => setNameAvailable(null))
        .finally(() => setNameChecking(false))
    }, 300)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [state.name, step])

  const update = useCallback((patch: Partial<WizardState>) => {
    setState((prev) => ({ ...prev, ...patch }))
  }, [])

  function canAdvance(): boolean {
    if (step === 0) return state.region !== ''
    if (step === 1) return state.job !== ''
    if (step === 2) return state.archetype !== ''
    return (
      state.name.length >= 3 &&
      state.name.length <= 20 &&
      state.gender !== '' &&
      nameAvailable === true
    )
  }

  function handleNext() {
    if (step < 3) setStep((s) => (s + 1) as StepIndex)
  }

  function handleBack() {
    if (step > 0) setStep((s) => (s - 1) as StepIndex)
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!canAdvance()) return
    setSubmitError(null)
    setSubmitting(true)
    try {
      await api.characters.create({
        name: state.name,
        job: state.job,
        archetype: state.archetype,
        region: state.region,
        gender: state.gender,
      })
      onComplete()
    } catch (err) {
      if (err instanceof ApiError) {
        setSubmitError(err.message)
      } else {
        setSubmitError('Unexpected error. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (optionsError) {
    return (
      <div style={styles.container}>
        <p style={styles.error}>{optionsError}</p>
        <button style={styles.secondaryBtn} onClick={onCancel}>Back</button>
      </div>
    )
  }

  if (!options) {
    return <div style={styles.container}><p style={styles.status}>Loading options…</p></div>
  }

  // Corrected archetype filter
  const archetypesForJob: CharacterOption[] = (() => {
    if (!state.job) return options.archetypes
    const filtered = options.archetypes.filter((a) => a.id.startsWith(state.job))
    return filtered.length > 0 ? filtered : options.archetypes
  })()

  const previewStats = computePreviewStats(options, state)

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>Create Character</h1>
        <button style={styles.secondaryBtn} onClick={onCancel} type="button">Cancel</button>
      </header>

      <div style={styles.stepBar}>
        {STEPS.map((label, idx) => (
          <div key={label} style={styles.stepItem}>
            <div style={{
              ...styles.stepDot,
              ...(idx < step ? styles.stepDone : {}),
              ...(idx === step ? styles.stepActive : {}),
            }}>
              {idx < step ? '✓' : idx + 1}
            </div>
            <span style={{ ...styles.stepLabel, ...(idx === step ? styles.stepLabelActive : {}) }}>
              {label}
            </span>
          </div>
        ))}
      </div>

      <div style={styles.body}>
        <form onSubmit={handleSubmit} style={styles.stepContent}>
          {step === 0 && (
            <OptionCards
              label="Select a Region"
              options={options.regions}
              selected={state.region}
              onSelect={(id) => update({ region: id })}
            />
          )}
          {step === 1 && (
            <OptionCards
              label="Select a Job"
              options={options.jobs}
              selected={state.job}
              onSelect={(id) => update({ job: id, archetype: '' })}
            />
          )}
          {step === 2 && (
            <OptionCards
              label="Select an Archetype"
              options={archetypesForJob}
              selected={state.archetype}
              onSelect={(id) => update({ archetype: id })}
            />
          )}
          {step === 3 && (
            <NameGenderStep
              name={state.name}
              gender={state.gender}
              nameAvailable={nameAvailable}
              nameChecking={nameChecking}
              onNameChange={(n) => update({ name: n })}
              onGenderChange={(g) => update({ gender: g })}
            />
          )}

          {submitError && <p style={styles.error}>{submitError}</p>}

          <div style={styles.navButtons}>
            {step > 0 && (
              <button style={styles.secondaryBtn} type="button" onClick={handleBack}>
                ← Back
              </button>
            )}
            {step < 3 && (
              <button
                style={{ ...styles.primaryBtn, ...(canAdvance() ? {} : styles.btnDisabled) }}
                type="button"
                onClick={handleNext}
                disabled={!canAdvance()}
              >
                Next →
              </button>
            )}
            {step === 3 && (
              <button
                style={{ ...styles.primaryBtn, ...(canAdvance() ? {} : styles.btnDisabled) }}
                type="submit"
                disabled={!canAdvance() || submitting}
              >
                {submitting ? 'Creating…' : 'Create Character'}
              </button>
            )}
          </div>
        </form>

        <aside style={styles.sidebar}>
          <h3 style={styles.sidebarTitle}>Starting Stats Preview</h3>
          {previewStats.length === 0 ? (
            <p style={styles.sidebarEmpty}>Select options to see stats.</p>
          ) : (
            <dl style={styles.statList}>
              {previewStats.map(([key, val]) => (
                <div key={key} style={styles.statRow}>
                  <dt style={styles.statKey}>{key}</dt>
                  <dd style={styles.statVal}>{val}</dd>
                </div>
              ))}
            </dl>
          )}
          {state.name && <p style={styles.previewName}>{state.name}</p>}
          {state.region && <p style={styles.previewTag}>{state.region}</p>}
          {state.job && <p style={styles.previewTag}>{state.job}{state.archetype ? ` / ${state.archetype}` : ''}</p>}
        </aside>
      </div>
    </div>
  )
}

interface OptionCardsProps {
  label: string
  options: CharacterOption[]
  selected: string
  onSelect: (id: string) => void
}

function OptionCards({ label, options, selected, onSelect }: OptionCardsProps) {
  return (
    <div>
      <h2 style={styles.stepHeading}>{label}</h2>
      {options.length === 0 && <p style={styles.status}>No options available.</p>}
      <div style={styles.optionGrid}>
        {options.map((opt) => (
          <button
            key={opt.id}
            type="button"
            style={{
              ...styles.optionCard,
              ...(selected === opt.id ? styles.optionCardSelected : {}),
            }}
            onClick={() => onSelect(opt.id)}
          >
            <div style={styles.optionName}>{opt.name}</div>
            {opt.description && (
              <div style={styles.optionDesc}>{opt.description}</div>
            )}
          </button>
        ))}
      </div>
    </div>
  )
}

interface NameGenderStepProps {
  name: string
  gender: string
  nameAvailable: boolean | null
  nameChecking: boolean
  onNameChange: (n: string) => void
  onGenderChange: (g: string) => void
}

function NameGenderStep({
  name,
  gender,
  nameAvailable,
  nameChecking,
  onNameChange,
  onGenderChange,
}: NameGenderStepProps) {
  function handleNameInput(e: ChangeEvent<HTMLInputElement>) {
    onNameChange(e.target.value)
  }

  let nameStatus: React.ReactNode = null
  if (name.length >= 3) {
    if (nameChecking) {
      nameStatus = <span style={styles.nameChecking}>Checking…</span>
    } else if (nameAvailable === true) {
      nameStatus = <span style={styles.nameAvailable}>✓ Available</span>
    } else if (nameAvailable === false) {
      nameStatus = <span style={styles.nameTaken}>✗ Name taken</span>
    }
  }

  return (
    <div>
      <h2 style={styles.stepHeading}>Name &amp; Gender</h2>

      <label style={styles.formLabel}>
        Character Name
        <div style={styles.nameInputRow}>
          <input
            style={styles.input}
            type="text"
            value={name}
            onChange={handleNameInput}
            minLength={3}
            maxLength={20}
            autoFocus
            placeholder="3–20 characters"
          />
          <span style={styles.nameStatusBadge}>{nameStatus}</span>
        </div>
      </label>

      <label style={styles.formLabel}>
        Gender
        <select
          style={styles.input}
          value={gender}
          onChange={(e) => onGenderChange(e.target.value)}
        >
          <option value="">Select…</option>
          <option value="male">Male</option>
          <option value="female">Female</option>
          <option value="nonbinary">Non-binary</option>
          <option value="other">Other</option>
        </select>
      </label>
    </div>
  )
}

function computePreviewStats(
  options: CharacterOptions,
  state: WizardState,
): [string, number][] {
  const merged: Record<string, number> = {}

  for (const key of [state.region, state.job, state.archetype]) {
    if (!key) continue
    const stats = options.starting_stats[key]
    if (!stats) continue
    for (const [stat, val] of Object.entries(stats)) {
      merged[stat] = (merged[stat] ?? 0) + val
    }
  }

  return Object.entries(merged).sort(([a], [b]) => a.localeCompare(b))
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: '100vh',
    background: '#0d0d0d',
    color: '#ccc',
    fontFamily: 'monospace',
    padding: '2rem',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: '1.5rem',
  },
  title: { margin: 0, color: '#e0c060', fontSize: '1.5rem' },
  stepBar: {
    display: 'flex',
    gap: '1rem',
    marginBottom: '2rem',
    alignItems: 'center',
  },
  stepItem: { display: 'flex', alignItems: 'center', gap: '0.4rem' },
  stepDot: {
    width: '28px',
    height: '28px',
    borderRadius: '50%',
    background: '#333',
    color: '#888',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: '0.75rem',
    fontWeight: 'bold',
    flexShrink: 0,
  },
  stepDone: { background: '#4caf50', color: '#fff' },
  stepActive: { background: '#e0c060', color: '#111' },
  stepLabel: { fontSize: '0.8rem', color: '#666' },
  stepLabelActive: { color: '#e0c060' },
  body: { display: 'flex', gap: '2rem', alignItems: 'flex-start' },
  stepContent: { flex: 1, minWidth: 0 },
  stepHeading: { color: '#e0c060', margin: '0 0 1rem', fontSize: '1.1rem' },
  optionGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))',
    gap: '0.75rem',
    marginBottom: '1.5rem',
  },
  optionCard: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '6px',
    padding: '0.75rem',
    cursor: 'pointer',
    textAlign: 'left' as const,
    fontFamily: 'monospace',
    color: '#ccc',
    transition: 'border-color 0.15s',
  },
  optionCardSelected: { border: '2px solid #e0c060', color: '#fff' },
  optionName: { fontWeight: 'bold', marginBottom: '0.25rem', color: '#eee' },
  optionDesc: { fontSize: '0.75rem', color: '#888', lineHeight: 1.4 },
  formLabel: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
    fontSize: '0.85rem',
    color: '#aaa',
    marginBottom: '1rem',
  },
  nameInputRow: { display: 'flex', alignItems: 'center', gap: '0.5rem' },
  input: {
    padding: '0.5rem',
    background: '#111',
    border: '1px solid #444',
    borderRadius: '4px',
    color: '#eee',
    fontSize: '1rem',
    fontFamily: 'monospace',
    flex: 1,
  },
  nameStatusBadge: { fontSize: '0.8rem', whiteSpace: 'nowrap' as const },
  nameChecking: { color: '#888' },
  nameAvailable: { color: '#4caf50' },
  nameTaken: { color: '#f55' },
  navButtons: { display: 'flex', gap: '0.75rem', marginTop: '1rem' },
  primaryBtn: {
    padding: '0.5rem 1.25rem',
    background: '#e0c060',
    color: '#111',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
    fontWeight: 'bold',
  },
  btnDisabled: { opacity: 0.4, cursor: 'not-allowed' },
  secondaryBtn: {
    padding: '0.5rem 1rem',
    background: 'none',
    color: '#888',
    border: '1px solid #444',
    borderRadius: '4px',
    cursor: 'pointer',
    fontFamily: 'monospace',
  },
  sidebar: {
    width: '220px',
    flexShrink: 0,
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '8px',
    padding: '1rem',
  },
  sidebarTitle: { margin: '0 0 0.75rem', color: '#e0c060', fontSize: '0.9rem' },
  sidebarEmpty: { color: '#555', fontSize: '0.8rem' },
  statList: { margin: 0, padding: 0 },
  statRow: {
    display: 'flex',
    justifyContent: 'space-between',
    padding: '0.2rem 0',
    borderBottom: '1px solid #222',
  },
  statKey: { color: '#aaa', fontSize: '0.8rem' },
  statVal: { color: '#eee', fontSize: '0.8rem', fontWeight: 'bold' },
  previewName: { marginTop: '0.75rem', color: '#e0c060', fontWeight: 'bold', fontSize: '0.9rem' },
  previewTag: { margin: '0.2rem 0 0', color: '#888', fontSize: '0.75rem' },
  status: { color: '#888' },
  error: { color: '#f55', fontSize: '0.85rem' },
}
