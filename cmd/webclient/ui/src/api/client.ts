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

export interface CharacterOption {
  id: string
  name: string
  description: string
}

export interface CharacterOptions {
  regions: CharacterOption[]
  jobs: CharacterOption[]
  archetypes: CharacterOption[]
  starting_stats: Record<string, Record<string, number>>
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

export interface CreateCharacterPayload {
  name: string
  job: string
  archetype: string
  region: string
  gender: string
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
  },
}
