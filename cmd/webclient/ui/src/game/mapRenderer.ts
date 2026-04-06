// mapRenderer.ts — ASCII map renderer matching the telnet RenderMap output.
//
// Returns structured segments (text + CSS color) so MapPanel can render
// colored spans inside <pre> blocks.
import type { MapTile, ZoneExitInfo } from '../proto'

const CELL_W = 4

// Danger level → CSS color (matches DangerColor in text_renderer.go)
export const DANGER_COLOR: Record<string, string> = {
  safe:        '#4a8',   // green
  sketchy:     '#cc0',   // yellow
  dangerous:   '#f80',   // orange
  all_out_war: '#f44',   // red
}
const DEFAULT_ROOM_COLOR = '#8ab'  // light blue-gray (unexplored / unknown)
const CURRENT_ROOM_COLOR = '#fff'  // bright white for current room
export const ZONE_EXIT_COLOR = '#c8f'  // purple — zone crossing exit

// POI type table — matches poi.go
export const POI_TYPES: Array<{ id: string; symbol: string; color: string; label: string }> = [
  { id: 'merchant',  symbol: '$', color: '#0bc', label: 'Merchant'  },
  { id: 'healer',    symbol: '+', color: '#4a8', label: 'Healer'    },
  { id: 'trainer',   symbol: 'T', color: '#48f', label: 'Trainer'   },
  { id: 'guard',     symbol: 'G', color: '#cc0', label: 'Guard'     },
  { id: 'npc',       symbol: 'N', color: '#aaa', label: 'NPC'       },
  { id: 'map',       symbol: 'M', color: '#0cc', label: 'Map'       },
  { id: 'cover',     symbol: 'C', color: '#cc0', label: 'Cover'     },
  { id: 'equipment', symbol: 'E', color: '#c8f', label: 'Equipment' },
]

export interface Segment {
  text: string
  color?: string  // CSS color; undefined = inherit from .map-ascii
  tile?: MapTile  // set on room cell segments only; used by MapPanel for hover tooltips
}

export type ColoredLine = Segment[]

export interface MapRenderResult {
  gridLines: ColoredLine[]
  legendLines: ColoredLine[]
}

function seg(text: string, color?: string, tile?: MapTile): Segment {
  const s: Segment = color ? { text, color } : { text }
  if (tile !== undefined) s.tile = tile
  return s
}

function dangerColor(t: MapTile): string {
  const level = t.dangerLevel ?? t.danger_level
  return (level && DANGER_COLOR[level]) ?? DEFAULT_ROOM_COLOR
}

function tileZoneExits(t: MapTile): ZoneExitInfo[] {
  return t.zoneExits ?? t.zone_exits ?? []
}

function hasZoneExit(t: MapTile): boolean {
  return tileZoneExits(t).length > 0
}

function poiSymbols(pois: string[]): ColoredLine {
  const out: Segment[] = []
  let cols = 0
  for (let i = 0; i < pois.length; i++) {
    if (cols >= CELL_W) break
    if (i === 3 && pois.length > 4) {
      out.push(seg('…'))
      cols++
      break
    }
    const pt = POI_TYPES.find(p => p.id === pois[i])
    out.push(seg(pt ? pt.symbol : '?', pt?.color))
    cols++
  }
  while (cols < CELL_W) { out.push(seg(' ')); cols++ }
  return out
}

function plainLine(text: string): ColoredLine {
  return [seg(text)]
}

function coordKey(x: number, y: number): string {
  return `${x},${y}`
}

export function renderMapTiles(tiles: MapTile[]): MapRenderResult {
  if (tiles.length === 0) return { gridLines: [], legendLines: [] }

  // Normalize tiles: protojson omits zero-value fields, so x/y may be undefined when 0.
  const normalized = tiles.map(t => ({ ...t, x: t.x ?? 0, y: t.y ?? 0 }))

  const byCoord = new Map<string, typeof normalized[0]>()
  for (const t of normalized) byCoord.set(coordKey(t.x, t.y), t)

  const xSet = new Set<number>()
  const ySet = new Set<number>()
  for (const t of normalized) { xSet.add(t.x); ySet.add(t.y) }
  const xs = Array.from(xSet).sort((a, b) => a - b)
  const ys = Array.from(ySet).sort((a, b) => a - b)

  const numByCoord = new Map<string, number>()
  let n = 1
  for (const y of ys)
    for (const x of xs)
      if (byCoord.has(coordKey(x, y)))
        numByCoord.set(coordKey(x, y), n++)



  function exits(t: MapTile): Set<string> {
    const s = new Set<string>()
    if (Array.isArray(t.exits)) for (const e of t.exits) s.add(e.toLowerCase())
    return s
  }

  const minX = xs[0]!
  let hasWestStub = false
  for (const y of ys) {
    const t = byCoord.get(coordKey(minX, y))
    if (t && exits(t).has('west')) { hasWestStub = true; break }
  }

  const gridLines: ColoredLine[] = []

  for (let yi = 0; yi < ys.length; yi++) {
    const y = ys[yi]!
    const row: ColoredLine = []

    for (let xi = 0; xi < xs.length; xi++) {
      const x = xs[xi]!

      if (xi === 0 && hasWestStub) {
        const t0 = byCoord.get(coordKey(x, y))
        row.push(seg((t0 && exits(t0).has('west')) ? '<' : ' '))
      }

      const t = byCoord.get(coordKey(x, y))
      if (!t) {
        row.push(seg('    '))
      } else {
        const num = numByCoord.get(coordKey(x, y))!
        if (t.current) {
          row.push(seg(`<${String(num).padStart(2)}>`, CURRENT_ROOM_COLOR, t))
        } else if (t.boss === true || t.bossRoom === true) {
          row.push(seg('<BB>', dangerColor(t), t))
        } else if (hasZoneExit(t)) {
          row.push(seg(`{${String(num).padStart(2)}}`, ZONE_EXIT_COLOR, t))
        } else {
          row.push(seg(`[${String(num).padStart(2)}]`, dangerColor(t), t))
        }
      }

      if (xi < xs.length - 1) {
        const nextX = xs[xi + 1]!
        const tEast = byCoord.get(coordKey(nextX, y))
        if (t && exits(t).has('east')) {
          row.push(seg(tEast ? '-' : '>'))
        } else {
          row.push(seg(' '))
        }
      } else if (t && exits(t).has('east')) {
        row.push(seg('>'))
      }
    }

    gridLines.push(row)

    // POI suffix row
    let hasPOIs = false
    for (const x of xs) {
      const t = byCoord.get(coordKey(x, y))
      if (t && Array.isArray(t.pois) && t.pois.length > 0) { hasPOIs = true; break }
    }
    if (hasPOIs) {
      const prow: ColoredLine = []
      for (let xi = 0; xi < xs.length; xi++) {
        const x = xs[xi]!
        if (xi === 0 && hasWestStub) prow.push(seg(' '))
        const t = byCoord.get(coordKey(x, y))
        const cellPOIs = (t && Array.isArray(t.pois)) ? t.pois : []
        prow.push(...poiSymbols(cellPOIs))
        if (xi < xs.length - 1) prow.push(seg(' '))
      }
      gridLines.push(prow)
    }

    // South connector row
    if (yi < ys.length - 1) {
      const nextY = ys[yi + 1]!
      let hasSouth = false
      for (const x of xs) {
        const t = byCoord.get(coordKey(x, y))
        if (t && exits(t).has('south')) { hasSouth = true; break }
      }
      if (hasSouth) {
        const srow: ColoredLine = []
        for (let xi = 0; xi < xs.length; xi++) {
          const x = xs[xi]!
          if (xi === 0 && hasWestStub) srow.push(seg(' '))
          const t = byCoord.get(coordKey(x, y))
          const tSouth = byCoord.get(coordKey(x, nextY))
          srow.push(seg(t && exits(t).has('south') ? (tSouth ? '  | ' : '  . ') : '    '))
          if (xi < xs.length - 1) {
            const nextX = xs[xi + 1]!
            let sep = ' '
            if (nextX - x === 2 && nextY - y === 2) {
              const tNE = byCoord.get(coordKey(x, nextY))
              const tSW = byCoord.get(coordKey(nextX, y))
              const tSE = byCoord.get(coordKey(nextX, nextY))
              const hasFwd = (tNE != null && exits(tNE).has('northeast')) || (tSW != null && exits(tSW).has('southwest'))
              const hasBack = (t != null && exits(t).has('southeast')) || (tSE != null && exits(tSE).has('northwest'))
              if (hasFwd && hasBack) sep = 'X'
              else if (hasFwd) sep = '/'
              else if (hasBack) sep = '\\'
            }
            srow.push(seg(sep))
          }
        }
        gridLines.push(srow)
      }
    } else {
      // Last row south stubs
      let hasSouthStub = false
      for (const x of xs) {
        const t = byCoord.get(coordKey(x, y))
        if (t && exits(t).has('south')) { hasSouthStub = true; break }
      }
      if (hasSouthStub) {
        const srow: ColoredLine = []
        for (let xi = 0; xi < xs.length; xi++) {
          const x = xs[xi]!
          if (xi === 0 && hasWestStub) srow.push(seg(' '))
          const t = byCoord.get(coordKey(x, y))
          srow.push(seg(t && exits(t).has('south') ? '  . ' : '    '))
          if (xi < xs.length - 1) srow.push(seg(' '))
        }
        gridLines.push(srow)
      }
    }
  }

  // Legend: POI key (only present types), then multi-column room entries.
  // Format matches telnet: marker(1) + num right-aligned(2) + "."(1) + name truncated/padded(16) = 20 chars/entry
  const LEGEND_COL_WIDTH = 20
  const LEGEND_NAME_WIDTH = LEGEND_COL_WIDTH - 4  // 4 = marker(1)+num(2)+dot(1)
  const LEGEND_COLS = 3

  const presentPOIs = new Set<string>()
  for (const t of normalized) {
    if (Array.isArray(t.pois)) for (const id of t.pois) presentPOIs.add(id)
  }

  const legendLines: ColoredLine[] = [plainLine('Legend:')]

  // Map key for room markers
  legendLines.push([
    seg('[##]', DEFAULT_ROOM_COLOR),
    seg(' Room  '),
    seg('{##}', ZONE_EXIT_COLOR),
    seg(' Zone Exit  '),
    seg('<##>', CURRENT_ROOM_COLOR),
    seg(' Current  '),
    seg('<BB>', '#f44'),
    seg(' Boss'),
  ])

  if (presentPOIs.size > 0) {
    legendLines.push(plainLine('Points of Interest'))
    const poiEntries = POI_TYPES.filter(pt => presentPOIs.has(pt.id))
    const POI_LABEL_WIDTH = LEGEND_COL_WIDTH - 3  // 3 = symbol(1) + "  "(2)
    for (let i = 0; i < poiEntries.length; i += LEGEND_COLS) {
      const rowSegs: ColoredLine = []
      for (let col = 0; col < LEGEND_COLS; col++) {
        const pt = poiEntries[i + col]
        if (!pt) break
        const label = pt.label.length > POI_LABEL_WIDTH
          ? pt.label.slice(0, POI_LABEL_WIDTH)
          : pt.label.padEnd(POI_LABEL_WIDTH)
        rowSegs.push(seg(pt.symbol, pt.color))
        rowSegs.push(seg(`  ${label}`))
      }
      legendLines.push(rowSegs)
    }
  }

  // Collect all room entries in row-major order.
  const roomEntries: Array<{ num: number; name: string; current: boolean; color: string }> = []
  for (const y of ys) {
    for (const x of xs) {
      const t = byCoord.get(coordKey(x, y))
      if (!t) continue
      const num = numByCoord.get(coordKey(x, y))!
      roomEntries.push({
        num,
        name: t.roomName ?? t.name ?? `Room ${num}`,
        current: t.current === true,
        color: t.current === true ? CURRENT_ROOM_COLOR : hasZoneExit(t) ? ZONE_EXIT_COLOR : dangerColor(t),
      })
    }
  }

  // Pack LEGEND_COLS entries per line.
  for (let i = 0; i < roomEntries.length; i += LEGEND_COLS) {
    const rowSegs: ColoredLine = []
    for (let col = 0; col < LEGEND_COLS; col++) {
      const e = roomEntries[i + col]
      if (!e) break
      const marker = e.current ? '*' : ' '
      const numStr = String(e.num).padStart(2)
      const name = e.name.length > LEGEND_NAME_WIDTH
        ? e.name.slice(0, LEGEND_NAME_WIDTH)
        : e.name.padEnd(LEGEND_NAME_WIDTH)
      rowSegs.push(seg(`${marker}${numStr}.`))
      rowSegs.push(seg(name, e.color))
    }
    legendLines.push(rowSegs)
  }

  return { gridLines, legendLines }
}
