import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { unzip } from 'fflate'

// ─── Public Types ────────────────────────────────────────────────────────────

export type AssetStatus = 'loading' | 'downloading' | 'ready' | 'error'

export interface TilesConfig {
  categories: Record<string, string>  // zone name pattern → tileset name
  tilesets: Record<string, { file: string; width: number; height: number }>
}

/** Map<filename, blobUrl> — typed as unknown to avoid PixiJS import at context level */
export type PixiTextureMap = Map<string, unknown>

export interface AssetPackContextValue {
  status: AssetStatus
  progress: number          // 0-100 during download
  textures: PixiTextureMap
  tilesConfig: TilesConfig | null
}

// ─── Constants ────────────────────────────────────────────────────────────────

const VERSION_KEY = 'mud-asset-version'
const IDB_DB_NAME = 'mud-assets'
const IDB_STORE_NAME = 'files'

// ─── IndexedDB helpers ────────────────────────────────────────────────────────

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(IDB_DB_NAME, 1)
    req.onupgradeneeded = () => {
      req.result.createObjectStore(IDB_STORE_NAME)
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

function idbGet(db: IDBDatabase, key: string): Promise<Uint8Array | undefined> {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(IDB_STORE_NAME, 'readonly')
    const req = tx.objectStore(IDB_STORE_NAME).get(key)
    req.onsuccess = () => resolve(req.result as Uint8Array | undefined)
    req.onerror = () => reject(req.error)
  })
}

function idbPut(db: IDBDatabase, key: string, value: Uint8Array): Promise<void> {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(IDB_STORE_NAME, 'readwrite')
    const req = tx.objectStore(IDB_STORE_NAME).put(value, key)
    req.onsuccess = () => resolve()
    req.onerror = () => reject(req.error)
  })
}

function idbGetAllKeys(db: IDBDatabase): Promise<string[]> {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(IDB_STORE_NAME, 'readonly')
    const req = tx.objectStore(IDB_STORE_NAME).getAllKeys()
    req.onsuccess = () => resolve(req.result as string[])
    req.onerror = () => reject(req.error)
  })
}

// ─── SHA-256 verification ─────────────────────────────────────────────────────

async function verifySha256(data: Uint8Array, expectedHex: string): Promise<boolean> {
  const hashBuffer = await crypto.subtle.digest('SHA-256', data)
  const hashArray = Array.from(new Uint8Array(hashBuffer))
  const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
  return hashHex === expectedHex.trim()
}

// ─── Load textures from IndexedDB ─────────────────────────────────────────────

async function loadTexturesFromIDB(db: IDBDatabase): Promise<PixiTextureMap> {
  const keys = await idbGetAllKeys(db)
  const map: PixiTextureMap = new Map()
  for (const key of keys) {
    if (key === '__tiles_config__') continue
    const data = await idbGet(db, key)
    if (data) {
      const blob = new Blob([data])
      const url = URL.createObjectURL(blob)
      map.set(key, url)
    }
  }
  return map
}

async function loadTilesConfig(db: IDBDatabase): Promise<TilesConfig | null> {
  const data = await idbGet(db, '__tiles_config__')
  if (!data) return null
  try {
    const text = new TextDecoder().decode(data)
    return JSON.parse(text) as TilesConfig
  } catch {
    return null
  }
}

// ─── Context ──────────────────────────────────────────────────────────────────

const AssetPackContext = createContext<AssetPackContextValue | null>(null)

export function AssetPackProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AssetStatus>('loading')
  const [progress, setProgress] = useState(0)
  const [textures, setTextures] = useState<PixiTextureMap>(new Map())
  const [tilesConfig, setTilesConfig] = useState<TilesConfig | null>(null)
  const loadingRef = useRef(false)

  const load = useCallback(async () => {
    if (loadingRef.current) return
    loadingRef.current = true

    let db: IDBDatabase
    try {
      db = await openDB()
    } catch (err) {
      console.error('AssetPackProvider: failed to open IndexedDB', err)
      setStatus('error')
      return
    }

    // ── Step 1: Fetch asset version ──────────────────────────────────────────
    let versionData: { version: number; download_url: string; sha256_url: string } | null = null
    let versionFetchFailed = false

    try {
      const resp = await fetch('/api/assets/version')
      if (!resp.ok) {
        versionFetchFailed = true
      } else {
        versionData = await resp.json() as { version: number; download_url: string; sha256_url: string }
      }
    } catch {
      versionFetchFailed = true
    }

    // ── Step 2: Check cache ──────────────────────────────────────────────────
    const storedVersion = localStorage.getItem(VERSION_KEY)
    const keys = await idbGetAllKeys(db)
    const hasCachedData = keys.length > 0

    if (versionFetchFailed) {
      if (hasCachedData) {
        console.warn('AssetPackProvider: /api/assets/version unavailable; using cached assets')
        const map = await loadTexturesFromIDB(db)
        const cfg = await loadTilesConfig(db)
        setTextures(map)
        setTilesConfig(cfg)
        setStatus('ready')
        return
      } else {
        setStatus('error')
        return
      }
    }

    // versionData is non-null here
    const remoteVersion = versionData!.version
    const versionMatch = storedVersion !== null && parseInt(storedVersion, 10) === remoteVersion

    if (versionMatch && hasCachedData) {
      // ── Step 3: Load from cache ────────────────────────────────────────────
      const map = await loadTexturesFromIDB(db)
      const cfg = await loadTilesConfig(db)
      setTextures(map)
      setTilesConfig(cfg)
      setStatus('ready')
      return
    }

    // ── Step 4: Download, verify, extract ─────────────────────────────────────
    setStatus('downloading')
    setProgress(0)

    let zipData: Uint8Array
    try {
      const resp = await fetch(versionData!.download_url)
      if (!resp.ok) {
        throw new Error(`Download failed: ${resp.status}`)
      }
      const contentLength = resp.headers.get('content-length')
      const total = contentLength ? parseInt(contentLength, 10) : 0
      const reader = resp.body?.getReader()
      if (!reader) throw new Error('No response body')

      const chunks: Uint8Array[] = []
      let received = 0
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        if (value) {
          chunks.push(value)
          received += value.length
          if (total > 0) {
            setProgress(Math.min(95, Math.round((received / total) * 95)))
          }
        }
      }
      const merged = new Uint8Array(received)
      let offset = 0
      for (const chunk of chunks) {
        merged.set(chunk, offset)
        offset += chunk.length
      }
      zipData = merged
    } catch (err) {
      console.error('AssetPackProvider: download failed', err)
      if (hasCachedData) {
        console.warn('AssetPackProvider: falling back to cached assets')
        const map = await loadTexturesFromIDB(db)
        const cfg = await loadTilesConfig(db)
        setTextures(map)
        setTilesConfig(cfg)
        setStatus('ready')
      } else {
        setStatus('error')
      }
      return
    }

    // Verify SHA-256
    try {
      const shaResp = await fetch(versionData!.sha256_url)
      if (shaResp.ok) {
        const expectedHex = await shaResp.text()
        const valid = await verifySha256(zipData, expectedHex)
        if (!valid) {
          console.error('AssetPackProvider: SHA-256 mismatch')
          setStatus('error')
          return
        }
      }
    } catch {
      // Non-fatal: proceed without verification if sha256 fetch fails
      console.warn('AssetPackProvider: could not fetch sha256, skipping verification')
    }

    setProgress(96)

    // Extract ZIP
    let extracted: Record<string, Uint8Array>
    try {
      extracted = await new Promise<Record<string, Uint8Array>>((resolve, reject) => {
        unzip(zipData, (err, data) => {
          if (err) reject(err)
          else resolve(data)
        })
      })
    } catch (err) {
      console.error('AssetPackProvider: zip extraction failed', err)
      setStatus('error')
      return
    }

    setProgress(98)

    // Store to IndexedDB
    const map: PixiTextureMap = new Map()
    let tilesConfigResult: TilesConfig | null = null

    for (const [filename, data] of Object.entries(extracted)) {
      await idbPut(db, filename, data)
      if (filename === 'tiles.json' || filename === 'tiles_config.json') {
        try {
          const text = new TextDecoder().decode(data)
          const parsed = JSON.parse(text) as TilesConfig
          tilesConfigResult = parsed
          await idbPut(db, '__tiles_config__', data)
        } catch {
          // Not valid JSON tiles config
        }
      } else {
        const blob = new Blob([data])
        const url = URL.createObjectURL(blob)
        map.set(filename, url)
      }
    }

    localStorage.setItem(VERSION_KEY, String(remoteVersion))
    setProgress(100)
    setTextures(map)
    setTilesConfig(tilesConfigResult)
    setStatus('ready')
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const value = useMemo<AssetPackContextValue>(
    () => ({ status, progress, textures, tilesConfig }),
    [status, progress, textures, tilesConfig],
  )

  return (
    <AssetPackContext.Provider value={value}>
      {children}
    </AssetPackContext.Provider>
  )
}

export function useAssetPack(): AssetPackContextValue {
  const ctx = useContext(AssetPackContext)
  if (!ctx) {
    throw new Error('useAssetPack must be used within an AssetPackProvider')
  }
  return ctx
}
