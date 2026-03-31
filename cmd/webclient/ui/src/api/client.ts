const BASE = ''

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

function getToken(): string | null {
  return localStorage.getItem('mud_token')
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  auth = true,
): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (auth) {
    const token = getToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
  }

  const resp = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (!resp.ok) {
    let message = resp.statusText
    try {
      const json = (await resp.json()) as { error?: string }
      if (json.error) {
        message = json.error
      }
    } catch {
      // ignore parse failure; use statusText
    }
    throw new ApiError(resp.status, message)
  }

  return resp.json() as Promise<T>
}

export interface AuthResponse {
  token: string
  account_id: number
  role: string
}

// BasicOption covers any selectable item with id/name/description
export interface BasicOption {
  id: string
  name: string
  description: string
}

export interface AbilityBoostGrant {
  fixed: string[]
  free: number
}

export interface SkillChoices {
  pool: string[]
  count: number
}

export interface SkillGrants {
  fixed?: string[]
  choices?: SkillChoices
}

export interface FeatChoices {
  pool: string[]
  count: number
}

export interface FeatGrants {
  general_count?: number
  fixed?: string[]
  choices?: FeatChoices
}

export interface PreparedEntry {
  id: string
  name?: string
  level: number
  description?: string
  action_cost?: number
  range?: string
  tradition?: string
  passive?: boolean
  focus_cost?: boolean
}

export interface SpontaneousEntry {
  id: string
  name?: string
  level: number
  description?: string
  action_cost?: number
  range?: string
  tradition?: string
  passive?: boolean
  focus_cost?: boolean
}

export interface PreparedGrants {
  slots_by_level?: Record<string, number>
  fixed?: PreparedEntry[]
  pool?: PreparedEntry[]
}

export interface SpontaneousGrants {
  known_by_level?: Record<string, number>
  uses_by_level?: Record<string, number>
  fixed?: SpontaneousEntry[]
  pool?: SpontaneousEntry[]
}

export interface TechGrants {
  hardwired?: string[]
  prepared?: PreparedGrants
  spontaneous?: SpontaneousGrants
}

export interface RegionOption {
  id: string
  name: string
  description: string
  modifiers?: Record<string, number>
  ability_boosts?: AbilityBoostGrant
}

export interface TeamOption {
  id: string
  name: string
  description: string
}

export interface ArchetypeOption {
  id: string
  name: string
  description: string
  ability_boosts?: AbilityBoostGrant
  tech_grants?: TechGrants
}

export interface PreparedTechChoice {
  level: number
  index: number
  tech_id: string
}

export interface JobOption {
  id: string
  name: string
  description: string
  archetype: string
  team: string
  key_ability: string
  hit_points_per_level: number
  skill_grants?: SkillGrants
  feat_grants?: FeatGrants
  tech_grants?: TechGrants
}

export interface FeatOption {
  id: string
  name: string
  description: string
  category: string
}

export interface CharacterOptions {
  regions: RegionOption[]
  teams: TeamOption[]
  archetypes: ArchetypeOption[]
  jobs: JobOption[]
  feats: FeatOption[]
}

export interface Character {
  id: number
  name: string
  job: string
  level: number
  current_hp: number
  max_hp: number
  region: string
  archetype: string
}

export interface SpontaneousChoice {
  id: string
  level: number
}

export interface CreateCharacterPayload {
  name: string
  job: string
  team: string
  region: string
  gender: string
  archetype_boosts?: string[]
  region_boosts?: string[]
  skill_choices?: string[]
  feat_choices?: string[]
  general_feat_choices?: string[]
  spontaneous_choices?: SpontaneousChoice[]
  prepared_tech_choices?: PreparedTechChoice[]
}

export const api = {
  auth: {
    login(username: string, password: string): Promise<AuthResponse> {
      return request<AuthResponse>('POST', '/api/auth/login', { username, password }, false)
    },
    register(username: string, password: string): Promise<AuthResponse> {
      return request<AuthResponse>('POST', '/api/auth/register', { username, password }, false)
    },
  },
  characters: {
    list(): Promise<Character[]> {
      return request<Character[]>('GET', '/api/characters')
    },
    create(payload: CreateCharacterPayload): Promise<{ character: Character }> {
      return request<{ character: Character }>('POST', '/api/characters', payload)
    },
    options(): Promise<CharacterOptions> {
      return request<CharacterOptions>('GET', '/api/characters/options')
    },
    checkName(name: string): Promise<{ available: boolean }> {
      return request<{ available: boolean }>(
        'GET',
        `/api/characters/check-name?name=${encodeURIComponent(name)}`,
      )
    },
    play(id: number): Promise<{ token: string }> {
      return request<{ token: string }>('POST', `/api/characters/${id}/play`)
    },
    delete(id: number): Promise<void> {
      return request<void>('DELETE', `/api/characters/${id}`)
    },
  },
}
