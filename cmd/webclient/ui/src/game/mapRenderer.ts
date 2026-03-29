// mapRenderer.ts — ASCII map renderer matching the telnet RenderMap output.
//
// Layout: compressed coordinate grid (only rows/cols that contain rooms),
// numbered cells [ N] / < N> (4 chars), east/south connectors, legend below.
import type { MapTile } from '../proto'

function coordKey(x: number, y: number): string {
  return `${x},${y}`
}

export function renderMapTiles(tiles: MapTile[]): string {
  if (tiles.length === 0) return ''

  // Index tiles by coordinate.
  const byCoord = new Map<string, MapTile>()
  for (const t of tiles) {
    byCoord.set(coordKey(t.x, t.y), t)
  }

  // Collect unique X and Y values that have rooms, sorted ascending.
  const xSet = new Set<number>()
  const ySet = new Set<number>()
  for (const t of tiles) {
    xSet.add(t.x)
    ySet.add(t.y)
  }
  const xs = Array.from(xSet).sort((a, b) => a - b)
  const ys = Array.from(ySet).sort((a, b) => a - b)

  // Assign legend numbers top-to-bottom, left-to-right.
  const numByCoord = new Map<string, number>()
  let n = 1
  for (const y of ys) {
    for (const x of xs) {
      if (byCoord.has(coordKey(x, y))) {
        numByCoord.set(coordKey(x, y), n++)
      }
    }
  }

  function exits(t: MapTile): Set<string> {
    const s = new Set<string>()
    if (Array.isArray(t.exits)) {
      for (const e of t.exits) s.add(e.toLowerCase())
    }
    return s
  }

  // Check if any leftmost-column tile has a west exit (needs stub column).
  const minX = xs[0]!
  let hasWestStub = false
  for (const y of ys) {
    const t = byCoord.get(coordKey(minX, y))
    if (t && exits(t).has('west')) { hasWestStub = true; break }
  }

  const lines: string[] = []

  for (let yi = 0; yi < ys.length; yi++) {
    const y = ys[yi]!
    let row = ''

    for (let xi = 0; xi < xs.length; xi++) {
      const x = xs[xi]!

      // West stub for first column.
      if (xi === 0 && hasWestStub) {
        const t0 = byCoord.get(coordKey(x, y))
        row += (t0 && exits(t0).has('west')) ? '<' : ' '
      }

      const t = byCoord.get(coordKey(x, y))
      if (!t) {
        row += '    '
      } else {
        const num = numByCoord.get(coordKey(x, y))!
        if (t.current) {
          row += `<${String(num).padStart(2)}>`
        } else if (t.boss === true || t.bossRoom === true) {
          row += '<BB>'
        } else {
          row += `[${String(num).padStart(2)}]`
        }
      }

      // East connector.
      if (xi < xs.length - 1) {
        const nextX = xs[xi + 1]!
        const tEast = byCoord.get(coordKey(nextX, y))
        if (t && exits(t).has('east')) {
          row += tEast ? '-' : '>'
        } else {
          row += ' '
        }
      } else if (t && exits(t).has('east')) {
        row += '>'
      }
    }

    lines.push(row.trimEnd())

    // South connector row — only emit when at least one tile in this row has a south exit.
    if (yi < ys.length - 1) {
      const nextY = ys[yi + 1]!
      let hasSouth = false
      for (const x of xs) {
        const t = byCoord.get(coordKey(x, y))
        if (t && exits(t).has('south')) { hasSouth = true; break }
      }
      if (hasSouth) {
        let srow = ''
        for (let xi = 0; xi < xs.length; xi++) {
          const x = xs[xi]!
          if (xi === 0 && hasWestStub) srow += ' '
          const t = byCoord.get(coordKey(x, y))
          const tSouth = byCoord.get(coordKey(x, nextY))
          if (t && exits(t).has('south')) {
            srow += tSouth ? '  | ' : '  . '
          } else {
            srow += '    '
          }
          if (xi < xs.length - 1) srow += ' '
        }
        lines.push(srow.trimEnd())
      }
    }
  }

  // Legend: number → room name, two entries per line where space allows.
  lines.push('')
  lines.push('Legend:')
  const legendEntries: Array<{ num: number; name: string; current: boolean }> = []
  for (const y of ys) {
    for (const x of xs) {
      const t = byCoord.get(coordKey(x, y))
      if (!t) continue
      const num = numByCoord.get(coordKey(x, y))!
      legendEntries.push({
        num,
        name: t.roomName ?? t.name ?? `Room ${num}`,
        current: t.current === true,
      })
    }
  }
  for (const entry of legendEntries) {
    const marker = entry.current ? '*' : ' '
    lines.push(` ${marker}${String(entry.num).padStart(2)}. ${entry.name}`)
  }

  return lines.join('\n')
}
