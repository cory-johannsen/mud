import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AssetPackProvider, useAssetPack } from './AssetPackContext'
import { AssetErrorScreen } from './AssetErrorScreen'
import type { JSX } from 'react'

// ─── IndexedDB mock ──────────────────────────────────────────────────────────

const idbStore: Map<string, Uint8Array> = new Map()

function makeIdbRequest<T>(result: T): IDBRequest<T> {
  const req = {
    result,
    error: null,
    readyState: 'done',
    onsuccess: null as ((e: Event) => void) | null,
    onerror: null as ((e: Event) => void) | null,
    transaction: null,
    source: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  } as unknown as IDBRequest<T>
  // Fire onsuccess asynchronously
  setTimeout(() => {
    if (req.onsuccess) req.onsuccess({} as Event)
  }, 0)
  return req
}

function makeMockIDB() {
  const mockObjectStore = {
    get: vi.fn((key: string) => makeIdbRequest(idbStore.get(key))),
    put: vi.fn((value: Uint8Array, key: string) => {
      idbStore.set(key, value)
      return makeIdbRequest(undefined)
    }),
    getAllKeys: vi.fn(() => makeIdbRequest(Array.from(idbStore.keys()))),
  }

  const mockTransaction = {
    objectStore: vi.fn(() => mockObjectStore),
  }

  const mockDB = {
    transaction: vi.fn(() => mockTransaction),
    createObjectStore: vi.fn(),
  }

  const openRequest = {
    result: mockDB,
    error: null,
    readyState: 'done',
    onupgradeneeded: null as ((e: IDBVersionChangeEvent) => void) | null,
    onsuccess: null as ((e: Event) => void) | null,
    onerror: null as ((e: Event) => void) | null,
    source: null,
    transaction: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }

  setTimeout(() => {
    if (openRequest.onsuccess) openRequest.onsuccess({} as Event)
  }, 0)

  const mockIDBFactory = {
    open: vi.fn(() => openRequest),
    deleteDatabase: vi.fn(),
    cmp: vi.fn(),
    databases: vi.fn(),
  } as unknown as IDBFactory

  return { mockIDBFactory, mockDB, mockObjectStore }
}

// ─── Helper component ────────────────────────────────────────────────────────

function StatusReader(): JSX.Element {
  const { status } = useAssetPack()
  return <div data-testid="status">{status}</div>
}

// ─── Setup / teardown ────────────────────────────────────────────────────────

beforeEach(() => {
  idbStore.clear()
  vi.clearAllMocks()
  localStorage.clear()

  const { mockIDBFactory } = makeMockIDB()
  Object.defineProperty(globalThis, 'indexedDB', {
    value: mockIDBFactory,
    writable: true,
    configurable: true,
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

// ─── Tests ───────────────────────────────────────────────────────────────────

describe('useAssetPack', () => {
  it('returns status "loading" initially before fetch resolves', () => {
    // Make fetch hang so we can capture the initial state
    vi.spyOn(globalThis, 'fetch').mockImplementation(
      () => new Promise(() => { /* never resolves */ })
    )

    const { getByTestId } = render(
      <AssetPackProvider>
        <StatusReader />
      </AssetPackProvider>
    )

    // The status should start as 'loading'
    expect(getByTestId('status').textContent).toBe('loading')
  })

  it('transitions to "error" when fetch returns 500 and no cache exists', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(null, { status: 500 })
    )

    render(
      <AssetPackProvider>
        <StatusReader />
      </AssetPackProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('status').textContent).toBe('error')
    }, { timeout: 3000 })
  })

  it('transitions to "error" when fetch throws a network error and no cache exists', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'))

    render(
      <AssetPackProvider>
        <StatusReader />
      </AssetPackProvider>
    )

    await waitFor(() => {
      expect(screen.getByTestId('status').textContent).toBe('error')
    }, { timeout: 3000 })
  })

  it('throws when useAssetPack is used outside of AssetPackProvider', () => {
    // Suppress React error boundary console output
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(() => render(<StatusReader />)).toThrow(
      'useAssetPack must be used within an AssetPackProvider'
    )

    consoleError.mockRestore()
  })
})

describe('AssetErrorScreen', () => {
  it('renders the "Asset pack unavailable" heading', () => {
    render(<AssetErrorScreen onRetry={() => {}} />)
    expect(screen.getByRole('heading', { name: /asset pack unavailable/i })).toBeInTheDocument()
  })

  it('renders a Retry button', () => {
    render(<AssetErrorScreen onRetry={() => {}} />)
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument()
  })

  it('calls onRetry when the Retry button is clicked', async () => {
    const user = userEvent.setup()
    const onRetry = vi.fn()

    render(<AssetErrorScreen onRetry={onRetry} />)
    await user.click(screen.getByRole('button', { name: /retry/i }))

    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('renders explanation text about PixiJS room scene', () => {
    render(<AssetErrorScreen onRetry={() => {}} />)
    expect(screen.getByText(/pixi.*room scene/i)).toBeInTheDocument()
  })
})
