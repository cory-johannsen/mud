import {
  type ChangeEvent,
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import {
  api,
  type CharacterOptions,
  type RegionOption,
  type ArchetypeOption,
  type JobOption,
  type BasicOption,
  type SpontaneousChoice,
  ApiError,
} from '../api/client'

interface WizardState {
  region: string
  team: string
  archetype: string
  job: string
  name: string
  gender: string
  // choice fields
  archetypeBoosts: string[]
  regionBoosts: string[]
  skillChoices: string[]
  featChoices: string[]
  generalFeatChoices: string[]
  spontaneousChoices: SpontaneousChoice[]
}

const EMPTY_STATE: WizardState = {
  region: '',
  team: '',
  archetype: '',
  job: '',
  name: '',
  gender: '',
  archetypeBoosts: [],
  regionBoosts: [],
  skillChoices: [],
  featChoices: [],
  generalFeatChoices: [],
  spontaneousChoices: [],
}

const ALL_ABILITIES = ['brutality', 'grit', 'quickness', 'reasoning', 'savvy', 'flair']

interface Props {
  onComplete: () => void
  onCancel: () => void
}

function computeSteps(options: CharacterOptions | null, state: WizardState): string[] {
  const steps = ['Region', 'Team', 'Archetype', 'Job']
  if (!options) return [...steps, 'Name & Gender']

  const region = options.regions.find((r) => r.id === state.region)
  const archetype = options.archetypes.find((a) => a.id === state.archetype)
  const job = options.jobs.find((j) => j.id === state.job)

  const archetypeFreeBoosts = archetype?.ability_boosts?.free ?? 0
  const regionFreeBoosts = region?.ability_boosts?.free ?? 0
  if (archetypeFreeBoosts + regionFreeBoosts > 0) {
    steps.push('Ability Boosts')
  }

  if (job?.skill_grants?.choices && job.skill_grants.choices.count > 0) {
    steps.push('Skills')
  }

  if ((job?.feat_grants?.choices?.count ?? 0) + (job?.feat_grants?.general_count ?? 0) > 0) {
    steps.push('Feats')
  }

  if ((job?.tech_grants?.spontaneous?.pool?.length ?? 0) > 0) {
    steps.push('Technology')
  }

  steps.push('Name & Gender')
  return steps
}

export function CharacterWizard({ onComplete, onCancel }: Props) {
  const [step, setStep] = useState<number>(0)
  const [state, setState] = useState<WizardState>(EMPTY_STATE)
  const [options, setOptions] = useState<CharacterOptions | null>(null)
  const [optionsError, setOptionsError] = useState<string | null>(null)
  const [nameAvailable, setNameAvailable] = useState<boolean | null>(null)
  const [nameChecking, setNameChecking] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const computedSteps = useMemo(() => computeSteps(options, state), [options, state])
  const lastStep = computedSteps.length - 1

  useEffect(() => {
    api.characters.options()
      .then((opts) => setOptions(opts))
      .catch(() => setOptionsError('Failed to load character options.'))
  }, [])

  // Clamp step when computedSteps shrinks (e.g., selecting a new job removes ability boost step)
  useEffect(() => {
    setStep((s) => Math.min(s, computedSteps.length - 1))
  }, [computedSteps.length])

  useEffect(() => {
    if (computedSteps[step] !== 'Name & Gender' || state.name.length < 3) {
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
  }, [state.name, step, computedSteps])

  const update = useCallback((patch: Partial<WizardState>) => {
    setState((prev) => {
      const next = { ...prev, ...patch }
      // Reset downstream selections when upstream changes
      if (patch.team !== undefined && patch.team !== prev.team) {
        next.archetype = ''
        next.job = ''
        next.archetypeBoosts = []
        next.regionBoosts = []
        next.skillChoices = []
        next.featChoices = []
        next.generalFeatChoices = []
        next.spontaneousChoices = []
      }
      if (patch.archetype !== undefined && patch.archetype !== prev.archetype) {
        next.job = ''
        next.archetypeBoosts = []
        next.skillChoices = []
        next.featChoices = []
        next.generalFeatChoices = []
        next.spontaneousChoices = []
      }
      if (patch.job !== undefined && patch.job !== prev.job) {
        next.skillChoices = []
        next.featChoices = []
        next.generalFeatChoices = []
        next.spontaneousChoices = []
      }
      return next
    })
  }, [])

  const region = options?.regions.find((r) => r.id === state.region)
  const archetype = options?.archetypes.find((a) => a.id === state.archetype)
  const job = options?.jobs.find((j) => j.id === state.job)

  const archetypeFreeBoosts = archetype?.ability_boosts?.free ?? 0
  const regionFreeBoosts = region?.ability_boosts?.free ?? 0

  function canAdvance(): boolean {
    const currentStep = computedSteps[step]
    if (currentStep === 'Region') return state.region !== ''
    if (currentStep === 'Team') return state.team !== ''
    if (currentStep === 'Archetype') return state.archetype !== ''
    if (currentStep === 'Job') return state.job !== ''
    if (currentStep === 'Ability Boosts') {
      const needed = archetypeFreeBoosts + regionFreeBoosts
      const chosen = state.archetypeBoosts.length + state.regionBoosts.length
      return chosen >= needed
    }
    if (currentStep === 'Skills') {
      const needed = job?.skill_grants?.choices?.count ?? 0
      return state.skillChoices.length >= needed
    }
    if (currentStep === 'Feats') {
      const jobFeatCount = job?.feat_grants?.choices?.count ?? 0
      const generalCount = job?.feat_grants?.general_count ?? 0
      return state.featChoices.length >= jobFeatCount && state.generalFeatChoices.filter((f) => f.trim() !== '').length >= generalCount
    }
    if (currentStep === 'Technology') {
      const spontPool = job?.tech_grants?.spontaneous?.pool ?? []
      const spontSlots = Object.values(job?.tech_grants?.spontaneous?.known_by_level ?? {}).reduce((a, b) => a + b, 0)
      const fixedSpont = (job?.tech_grants?.spontaneous?.fixed ?? []).length
      const needed = Math.max(0, spontSlots - fixedSpont)
      // Ensure they haven't chosen more than the pool allows
      return state.spontaneousChoices.length >= needed && state.spontaneousChoices.length <= spontPool.length
    }
    // Name & Gender
    return (
      state.name.length >= 3 &&
      state.name.length <= 20 &&
      state.gender !== '' &&
      nameAvailable === true
    )
  }

  function handleNext() {
    if (step < lastStep) setStep((s) => s + 1)
  }

  function handleBack() {
    if (step > 0) setStep((s) => s - 1)
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
        team: state.team,
        region: state.region,
        gender: state.gender,
        archetype_boosts: state.archetypeBoosts.length > 0 ? state.archetypeBoosts : undefined,
        region_boosts: state.regionBoosts.length > 0 ? state.regionBoosts : undefined,
        skill_choices: state.skillChoices.length > 0 ? state.skillChoices : undefined,
        feat_choices: state.featChoices.length > 0 ? state.featChoices : undefined,
        general_feat_choices: state.generalFeatChoices.length > 0 ? state.generalFeatChoices : undefined,
        spontaneous_choices: state.spontaneousChoices.length > 0 ? state.spontaneousChoices : undefined,
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

  // Archetypes that have at least one job available for the selected team
  const archetypesForTeam: BasicOption[] = (() => {
    if (!state.team) return options.archetypes
    const validArchetypes = new Set<string>()
    for (const j of options.jobs) {
      if ((j.team === state.team || !j.team) && j.archetype) {
        validArchetypes.add(j.archetype)
      }
    }
    const filtered = options.archetypes.filter((a) => validArchetypes.has(a.id))
    return filtered.length > 0 ? filtered : options.archetypes
  })()

  // Jobs filtered by selected team and archetype
  const jobsForTeamAndArchetype: BasicOption[] = (() => {
    return options.jobs.filter((j) => {
      const teamMatch = !j.team || j.team === state.team
      const archetypeMatch = !state.archetype || j.archetype === state.archetype
      return teamMatch && archetypeMatch
    })
  })()

  const previewStats = computePreviewStats(options, state)
  const currentStepName = computedSteps[step]

  return (
    <div style={styles.container}>
      <header style={styles.header}>
        <h1 style={styles.title}>Create Character</h1>
        <button style={styles.secondaryBtn} onClick={onCancel} type="button">Cancel</button>
      </header>

      <div style={styles.stepBar}>
        {computedSteps.map((label, idx) => (
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
          {currentStepName === 'Region' && (
            <OptionCards
              label="Select a Region"
              options={options.regions}
              selected={state.region}
              onSelect={(id) => update({ region: id })}
            />
          )}
          {currentStepName === 'Team' && (
            <OptionCards
              label="Select a Team"
              options={options.teams}
              selected={state.team}
              onSelect={(id) => update({ team: id })}
            />
          )}
          {currentStepName === 'Archetype' && (
            <OptionCards
              label="Select an Archetype"
              options={archetypesForTeam}
              selected={state.archetype}
              onSelect={(id) => update({ archetype: id })}
            />
          )}
          {currentStepName === 'Job' && (
            <OptionCards
              label="Select a Job"
              options={jobsForTeamAndArchetype}
              selected={state.job}
              onSelect={(id) => update({ job: id })}
            />
          )}
          {currentStepName === 'Ability Boosts' && (
            <AbilityBoostsStep
              region={region}
              archetype={archetype}
              archetypeBoosts={state.archetypeBoosts}
              regionBoosts={state.regionBoosts}
              onArchetypeBoostChange={(boosts) => update({ archetypeBoosts: boosts })}
              onRegionBoostChange={(boosts) => update({ regionBoosts: boosts })}
            />
          )}
          {currentStepName === 'Skills' && job && (
            <SkillsStep
              job={job}
              skillChoices={state.skillChoices}
              onSkillChoicesChange={(choices) => update({ skillChoices: choices })}
            />
          )}
          {currentStepName === 'Feats' && job && (
            <FeatsStep
              job={job}
              featChoices={state.featChoices}
              generalFeatChoices={state.generalFeatChoices}
              onFeatChoicesChange={(choices) => update({ featChoices: choices })}
              onGeneralFeatChoicesChange={(choices) => update({ generalFeatChoices: choices })}
            />
          )}
          {currentStepName === 'Technology' && job && (
            <TechnologyStep
              job={job}
              spontaneousChoices={state.spontaneousChoices}
              onSpontaneousChoicesChange={(choices) => update({ spontaneousChoices: choices })}
            />
          )}
          {currentStepName === 'Name & Gender' && (
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
            {step < lastStep && (
              <button
                style={{ ...styles.primaryBtn, ...(canAdvance() ? {} : styles.btnDisabled) }}
                type="button"
                onClick={handleNext}
                disabled={!canAdvance()}
              >
                Next →
              </button>
            )}
            {step === lastStep && (
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
          {state.region && <p style={styles.previewTag}>{region?.name ?? state.region}</p>}
          {state.team && (
            <p style={styles.previewTag}>
              {options.teams.find((t) => t.id === state.team)?.name ?? state.team}
              {state.archetype ? ` / ${archetype?.name ?? state.archetype}` : ''}
            </p>
          )}
          {state.job && <p style={styles.previewTag}>{job?.name ?? state.job}</p>}
          {(state.archetypeBoosts.length > 0 || state.regionBoosts.length > 0) && (
            <p style={styles.previewTag}>
              Boosts: {[...state.archetypeBoosts, ...state.regionBoosts].join(', ')}
            </p>
          )}
        </aside>
      </div>
    </div>
  )
}

interface OptionCardsProps {
  label: string
  options: BasicOption[]
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

interface AbilityBoostsStepProps {
  region: RegionOption | undefined
  archetype: ArchetypeOption | undefined
  archetypeBoosts: string[]
  regionBoosts: string[]
  onArchetypeBoostChange: (boosts: string[]) => void
  onRegionBoostChange: (boosts: string[]) => void
}

function AbilityBoostsStep({
  region,
  archetype,
  archetypeBoosts,
  regionBoosts,
  onArchetypeBoostChange,
  onRegionBoostChange,
}: AbilityBoostsStepProps) {
  const archetypeFixed = archetype?.ability_boosts?.fixed ?? []
  const archetypeFree = archetype?.ability_boosts?.free ?? 0
  const regionFixed = region?.ability_boosts?.fixed ?? []
  const regionFree = region?.ability_boosts?.free ?? 0

  // Abilities already taken by fixed or chosen boosts (across both archetype and region)
  function takenAbilities(excludeSource: 'archetype' | 'region', excludeIndex: number): Set<string> {
    const taken = new Set<string>()
    for (const a of archetypeFixed) taken.add(a)
    for (const a of regionFixed) taken.add(a)
    archetypeBoosts.forEach((a, i) => {
      if (excludeSource === 'archetype' && i === excludeIndex) return
      if (a) taken.add(a)
    })
    regionBoosts.forEach((a, i) => {
      if (excludeSource === 'region' && i === excludeIndex) return
      if (a) taken.add(a)
    })
    return taken
  }

  function handleArchetypeBoost(index: number, value: string) {
    const updated = [...archetypeBoosts]
    updated[index] = value
    onArchetypeBoostChange(updated)
  }

  function handleRegionBoost(index: number, value: string) {
    const updated = [...regionBoosts]
    updated[index] = value
    onRegionBoostChange(updated)
  }

  return (
    <div>
      <h2 style={styles.stepHeading}>Ability Boosts</h2>
      <p style={styles.stepSubtext}>
        Choose ability boosts granted by your archetype and region.
        Each boost increases an ability score by +2. You cannot boost the same ability twice.
      </p>

      {archetypeFree > 0 || archetypeFixed.length > 0 ? (
        <div style={styles.boostSection}>
          <h3 style={styles.boostSectionTitle}>{archetype?.name ?? 'Archetype'} Boosts</h3>
          {archetypeFixed.length > 0 && (
            <div style={styles.fixedBoostList}>
              {archetypeFixed.map((ab) => (
                <div key={ab} style={styles.fixedBoost}>
                  <span style={styles.fixedBoostLabel}>{capitalize(ab)}</span>
                  <span style={styles.fixedBoostBadge}>+2 (fixed)</span>
                </div>
              ))}
            </div>
          )}
          {Array.from({ length: archetypeFree }).map((_, i) => {
            const taken = takenAbilities('archetype', i)
            return (
              <div key={i} style={styles.freeBoostRow}>
                <label style={styles.freeBoostLabel}>Free Boost #{i + 1}</label>
                <select
                  style={styles.boostSelect}
                  value={archetypeBoosts[i] ?? ''}
                  onChange={(e) => handleArchetypeBoost(i, e.target.value)}
                >
                  <option value="">Select ability…</option>
                  {ALL_ABILITIES.map((ab) => (
                    <option key={ab} value={ab} disabled={taken.has(ab) && archetypeBoosts[i] !== ab}>
                      {capitalize(ab)}
                    </option>
                  ))}
                </select>
              </div>
            )
          })}
        </div>
      ) : null}

      {regionFree > 0 || regionFixed.length > 0 ? (
        <div style={styles.boostSection}>
          <h3 style={styles.boostSectionTitle}>{region?.name ?? 'Region'} Boosts</h3>
          {regionFixed.length > 0 && (
            <div style={styles.fixedBoostList}>
              {regionFixed.map((ab) => (
                <div key={ab} style={styles.fixedBoost}>
                  <span style={styles.fixedBoostLabel}>{capitalize(ab)}</span>
                  <span style={styles.fixedBoostBadge}>+2 (fixed)</span>
                </div>
              ))}
            </div>
          )}
          {Array.from({ length: regionFree }).map((_, i) => {
            const taken = takenAbilities('region', i)
            return (
              <div key={i} style={styles.freeBoostRow}>
                <label style={styles.freeBoostLabel}>Free Boost #{i + 1}</label>
                <select
                  style={styles.boostSelect}
                  value={regionBoosts[i] ?? ''}
                  onChange={(e) => handleRegionBoost(i, e.target.value)}
                >
                  <option value="">Select ability…</option>
                  {ALL_ABILITIES.map((ab) => (
                    <option key={ab} value={ab} disabled={taken.has(ab) && regionBoosts[i] !== ab}>
                      {capitalize(ab)}
                    </option>
                  ))}
                </select>
              </div>
            )
          })}
        </div>
      ) : null}
    </div>
  )
}

interface SkillsStepProps {
  job: JobOption
  skillChoices: string[]
  onSkillChoicesChange: (choices: string[]) => void
}

function SkillsStep({ job, skillChoices, onSkillChoicesChange }: SkillsStepProps) {
  const fixed = job.skill_grants?.fixed ?? []
  const choicePool = job.skill_grants?.choices?.pool ?? []
  const choiceCount = job.skill_grants?.choices?.count ?? 0

  function toggle(skillId: string) {
    if (skillChoices.includes(skillId)) {
      onSkillChoicesChange(skillChoices.filter((s) => s !== skillId))
    } else if (skillChoices.length < choiceCount) {
      onSkillChoicesChange([...skillChoices, skillId])
    }
  }

  return (
    <div>
      <h2 style={styles.stepHeading}>Skills</h2>
      {fixed.length > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>Fixed Skills (granted automatically)</h3>
          <div style={styles.grantList}>
            {fixed.map((s) => (
              <div key={s} style={styles.grantItem}>{s}</div>
            ))}
          </div>
        </div>
      )}
      {choicePool.length > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>
            Choose {choiceCount} Skill{choiceCount !== 1 ? 's' : ''} ({skillChoices.length}/{choiceCount} selected)
          </h3>
          <div style={styles.choiceGrid}>
            {choicePool.map((s) => {
              const selected = skillChoices.includes(s)
              const disabled = !selected && skillChoices.length >= choiceCount
              return (
                <button
                  key={s}
                  type="button"
                  style={{
                    ...styles.choiceBtn,
                    ...(selected ? styles.choiceBtnSelected : {}),
                    ...(disabled ? styles.choiceBtnDisabled : {}),
                  }}
                  onClick={() => toggle(s)}
                  disabled={disabled}
                >
                  {s}
                </button>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}

interface FeatsStepProps {
  job: JobOption
  featChoices: string[]
  generalFeatChoices: string[]
  onFeatChoicesChange: (choices: string[]) => void
  onGeneralFeatChoicesChange: (choices: string[]) => void
}

function FeatsStep({
  job,
  featChoices,
  generalFeatChoices,
  onFeatChoicesChange,
  onGeneralFeatChoicesChange,
}: FeatsStepProps) {
  const fixedFeats = job.feat_grants?.fixed ?? []
  const choicePool = job.feat_grants?.choices?.pool ?? []
  const choiceCount = job.feat_grants?.choices?.count ?? 0
  const generalCount = job.feat_grants?.general_count ?? 0

  function toggleJobFeat(featId: string) {
    if (featChoices.includes(featId)) {
      onFeatChoicesChange(featChoices.filter((f) => f !== featId))
    } else if (featChoices.length < choiceCount) {
      onFeatChoicesChange([...featChoices, featId])
    }
  }

  // General feats use a text input approach since there's no predefined pool
  function handleGeneralFeatInput(index: number, value: string) {
    const updated = [...generalFeatChoices]
    updated[index] = value
    onGeneralFeatChoicesChange(updated)
  }

  return (
    <div>
      <h2 style={styles.stepHeading}>Feats</h2>
      {fixedFeats.length > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>Fixed Feats (granted automatically)</h3>
          <div style={styles.grantList}>
            {fixedFeats.map((f) => (
              <div key={f} style={styles.grantItem}>{f}</div>
            ))}
          </div>
        </div>
      )}
      {choicePool.length > 0 && choiceCount > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>
            Choose {choiceCount} Job Feat{choiceCount !== 1 ? 's' : ''} ({featChoices.length}/{choiceCount} selected)
          </h3>
          <div style={styles.choiceGrid}>
            {choicePool.map((f) => {
              const selected = featChoices.includes(f)
              const disabled = !selected && featChoices.length >= choiceCount
              return (
                <button
                  key={f}
                  type="button"
                  style={{
                    ...styles.choiceBtn,
                    ...(selected ? styles.choiceBtnSelected : {}),
                    ...(disabled ? styles.choiceBtnDisabled : {}),
                  }}
                  onClick={() => toggleJobFeat(f)}
                  disabled={disabled}
                >
                  {f}
                </button>
              )
            })}
          </div>
        </div>
      )}
      {generalCount > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>
            General Feat{generalCount !== 1 ? 's' : ''} ({generalFeatChoices.filter((f) => f.trim() !== '').length}/{generalCount} chosen)
          </h3>
          <p style={styles.stepSubtext}>Enter the ID of each general feat you wish to take.</p>
          {Array.from({ length: generalCount }).map((_, i) => (
            <div key={i} style={styles.freeBoostRow}>
              <label style={styles.freeBoostLabel}>General Feat #{i + 1}</label>
              <input
                style={styles.input}
                type="text"
                value={generalFeatChoices[i] ?? ''}
                onChange={(e) => handleGeneralFeatInput(i, e.target.value)}
                placeholder="feat ID"
              />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

interface TechnologyStepProps {
  job: JobOption
  spontaneousChoices: SpontaneousChoice[]
  onSpontaneousChoicesChange: (choices: SpontaneousChoice[]) => void
}

function TechnologyStep({ job, spontaneousChoices, onSpontaneousChoicesChange }: TechnologyStepProps) {
  const spontPool = job.tech_grants?.spontaneous?.pool ?? []
  const spontSlots = Object.values(job.tech_grants?.spontaneous?.known_by_level ?? {}).reduce((a, b) => a + b, 0)
  const fixedSpont = job.tech_grants?.spontaneous?.fixed ?? []
  const needed = Math.max(0, spontSlots - fixedSpont.length)
  const hardwired = job.tech_grants?.hardwired ?? []

  function toggleSpont(entry: { id: string; level: number }) {
    const idx = spontaneousChoices.findIndex((c) => c.id === entry.id && c.level === entry.level)
    if (idx >= 0) {
      onSpontaneousChoicesChange(spontaneousChoices.filter((_, i) => i !== idx))
    } else if (spontaneousChoices.length < needed) {
      onSpontaneousChoicesChange([...spontaneousChoices, { id: entry.id, level: entry.level }])
    }
  }

  function isSelected(entry: { id: string; level: number }) {
    return spontaneousChoices.some((c) => c.id === entry.id && c.level === entry.level)
  }

  return (
    <div>
      <h2 style={styles.stepHeading}>Technology</h2>
      {hardwired.length > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>Hardwired Technologies (always known)</h3>
          <div style={styles.grantList}>
            {hardwired.map((t) => (
              <div key={t} style={styles.grantItem}>{t}</div>
            ))}
          </div>
        </div>
      )}
      {fixedSpont.length > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>Fixed Spontaneous Tech (granted automatically)</h3>
          <div style={styles.grantList}>
            {fixedSpont.map((t) => (
              <div key={`${t.id}-${t.level}`} style={styles.grantItem}>
                {t.id} <span style={styles.levelBadge}>Lv{t.level}</span>
              </div>
            ))}
          </div>
        </div>
      )}
      {spontPool.length > 0 && needed > 0 && (
        <div style={styles.grantSection}>
          <h3 style={styles.grantSectionTitle}>
            Choose {needed} Spontaneous Tech{needed !== 1 ? 's' : ''} ({spontaneousChoices.length}/{needed} selected)
          </h3>
          <div style={styles.choiceGrid}>
            {spontPool.map((entry) => {
              const selected = isSelected(entry)
              const disabled = !selected && spontaneousChoices.length >= needed
              return (
                <button
                  key={`${entry.id}-${entry.level}`}
                  type="button"
                  style={{
                    ...styles.choiceBtn,
                    ...(selected ? styles.choiceBtnSelected : {}),
                    ...(disabled ? styles.choiceBtnDisabled : {}),
                  }}
                  onClick={() => toggleSpont(entry)}
                  disabled={disabled}
                >
                  <div>{entry.id}</div>
                  <div style={styles.levelBadge}>Level {entry.level}</div>
                </button>
              )
            })}
          </div>
        </div>
      )}
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

  const region = options.regions.find((r) => r.id === state.region)
  if (region?.modifiers) {
    for (const [stat, val] of Object.entries(region.modifiers)) {
      merged[stat] = (merged[stat] ?? 0) + val
    }
  }

  return Object.entries(merged).sort(([a], [b]) => a.localeCompare(b))
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1)
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
    flexWrap: 'wrap',
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
  stepSubtext: { color: '#888', fontSize: '0.8rem', marginBottom: '1rem' },
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
  // Boost step styles
  boostSection: { marginBottom: '1.5rem' },
  boostSectionTitle: { color: '#bbb', fontSize: '0.9rem', margin: '0 0 0.75rem' },
  fixedBoostList: { display: 'flex', flexWrap: 'wrap' as const, gap: '0.5rem', marginBottom: '0.75rem' },
  fixedBoost: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.4rem',
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '4px',
    padding: '0.3rem 0.6rem',
  },
  fixedBoostLabel: { color: '#eee', fontSize: '0.85rem' },
  fixedBoostBadge: { color: '#888', fontSize: '0.75rem' },
  freeBoostRow: { display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.5rem' },
  freeBoostLabel: { color: '#aaa', fontSize: '0.85rem', width: '110px', flexShrink: 0 },
  boostSelect: {
    padding: '0.4rem 0.5rem',
    background: '#111',
    border: '1px solid #444',
    borderRadius: '4px',
    color: '#eee',
    fontSize: '0.9rem',
    fontFamily: 'monospace',
    flex: 1,
  },
  // Grant/choice step styles
  grantSection: { marginBottom: '1.5rem' },
  grantSectionTitle: { color: '#bbb', fontSize: '0.9rem', margin: '0 0 0.75rem' },
  grantList: { display: 'flex', flexWrap: 'wrap' as const, gap: '0.4rem', marginBottom: '0.75rem' },
  grantItem: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '4px',
    padding: '0.25rem 0.6rem',
    color: '#ccc',
    fontSize: '0.8rem',
  },
  choiceGrid: {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: '0.5rem',
    marginBottom: '0.75rem',
  },
  choiceBtn: {
    background: '#1a1a1a',
    border: '1px solid #333',
    borderRadius: '4px',
    padding: '0.4rem 0.8rem',
    color: '#ccc',
    fontSize: '0.8rem',
    cursor: 'pointer',
    fontFamily: 'monospace',
    textAlign: 'center' as const,
  },
  choiceBtnSelected: { border: '2px solid #e0c060', color: '#fff', background: '#2a2a1a' },
  choiceBtnDisabled: { opacity: 0.4, cursor: 'not-allowed' },
  levelBadge: { color: '#888', fontSize: '0.7rem' },
}
