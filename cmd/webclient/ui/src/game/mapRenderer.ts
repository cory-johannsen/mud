import type { MapTile } from '../proto'

interface Bounds {
  minX: number; maxX: number; minY: number; maxY: number
}

function getBounds(tiles: MapTile[]): Bounds {
  let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
  for (const t of tiles) {
    if (t.x < minX) minX = t.x
    if (t.x > maxX) maxX = t.x
    if (t.y < minY) minY = t.y
    if (t.y > maxY) maxY = t.y
  }
  return { minX, maxX, minY, maxY }
}

// Each room occupies 3x3 chars. Rooms are spaced 4 chars apart (3 room + 1 gutter).
const CELL = 3
const GAP = 1
const STRIDE = CELL + GAP

export function renderMapTiles(tiles: MapTile[]): string {
  if (tiles.length === 0) return ''

  const { minX, maxX, minY, maxY } = getBounds(tiles)
  const cols = (maxX - minX) * STRIDE + CELL
  const rows = (maxY - minY) * STRIDE + CELL

  // Build a mutable 2D char grid.
  const grid: string[][] = Array.from({ length: rows }, () =>
    Array.from({ length: cols }, () => ' '),
  )

  for (const t of tiles) {
    const cx = (t.x - minX) * STRIDE
    const cy = (t.y - minY) * STRIDE
    const isCurrent = t.current === true
    const isBoss = t.boss === true || t.bossRoom === true
    const inner = isCurrent ? '*' : isBoss ? 'B' : ' '
    // Draw 3-char room block: [X]
    if (grid[cy]) {
      grid[cy]![cx]     = '['
      grid[cy]![cx + 1] = inner
      grid[cy]![cx + 2] = ']'
    }
  }

  // Draw exit corridors.
  for (const t of tiles) {
    const cx = (t.x - minX) * STRIDE
    const cy = (t.y - minY) * STRIDE
    const exitDirs: string[] = Array.isArray(t.exits)
      ? (t.exits as string[])
      : []
    for (const dir of exitDirs) {
      switch (dir.toLowerCase()) {
        case 'east':
          if (cx + CELL < cols && grid[cy]) grid[cy]![cx + CELL] = '─'
          break
        case 'west':
          if (cx - 1 >= 0 && grid[cy]) grid[cy]![cx - 1] = '─'
          break
        case 'south':
          if (cy + GAP < rows && grid[cy + GAP]) grid[cy + GAP]![cx + 1] = '│'
          break
        case 'north':
          if (cy - 1 >= 0 && grid[cy - 1]) grid[cy - 1]![cx + 1] = '│'
          break
      }
    }
  }

  return grid.map((row) => row.join('')).join('\n')
}
