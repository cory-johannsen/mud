import { useState } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { GameProvider, useGame } from '../game/GameContext'
import type { TimeOfDayEvent } from '../proto'
import { RoomPanel } from '../game/panels/RoomPanel'
import { MapPanel } from '../game/panels/MapPanel'
import { FeedPanel } from '../game/panels/FeedPanel'
import { CharacterPanel } from '../game/panels/CharacterPanel'
import { InputPanel } from '../game/panels/InputPanel'
import { HotbarPanel } from '../game/panels/HotbarPanel'
import { CombatBanner } from '../game/CombatBanner'
import { DrawerContainer } from '../game/drawers/DrawerContainer'
import type { DrawerType } from '../game/drawers/DrawerContainer'
import { HelpModal } from '../game/HelpModal'
import { NpcModal } from '../game/NpcModal'
import '../styles/game.css'
type MobilePanel = 'room' | 'map' | 'character'

const MONTHS = ['January','February','March','April','May','June',
                 'July','August','September','October','November','December']

function ordinal(n: number): string {
  if (n >= 11 && n <= 13) return 'th'
  switch (n % 10) { case 1: return 'st'; case 2: return 'nd'; case 3: return 'rd'; default: return 'th' }
}

function formatTimeOfDay(tod: TimeOfDayEvent): string {
  const month = tod.month && tod.month >= 1 && tod.month <= 12
    ? MONTHS[tod.month - 1]! : ''
  const day = tod.day ? `${tod.day}${ordinal(tod.day)}` : ''
  const hour = tod.hour !== undefined ? String(tod.hour).padStart(2, '0') + ':00' : ''
  const period = tod.period ?? ''
  return [month && day ? `${month} ${day}` : '', period, hour].filter(Boolean).join(' · ')
}

// Inner component that has access to GameContext.
function GameLayout() {
  const { state } = useGame()
  const { logout } = useAuth()
  const [openDrawer, setOpenDrawer] = useState<DrawerType | null>(null)
  const [activeMobilePanel, setActiveMobilePanel] = useState<MobilePanel>('room')
  const [showHelp, setShowHelp] = useState(false)

  function toggleDrawer(d: DrawerType) {
    setOpenDrawer((prev) => (prev === d ? null : d))
  }

  return (
    <div className="game-layout">
      {/* Toolbar */}
      <div className="panel-toolbar">
        <div className="toolbar">
          <span className="toolbar-zone">
            {state.roomView?.zoneName ?? state.roomView?.zone_name ?? 'Connecting…'}
          </span>
          {state.timeOfDay && (
            <span className="toolbar-time">{formatTimeOfDay(state.timeOfDay)}</span>
          )}
          {(['inventory', 'equipment', 'skills', 'feats', 'stats', 'technology'] as DrawerType[]).map((d) => (
            <button
              key={d}
              className={`toolbar-btn${openDrawer === d ? ' active' : ''}`}
              onClick={() => toggleDrawer(d)}
            >
              {d.charAt(0).toUpperCase() + d.slice(1)}
            </button>
          ))}
          <button className="toolbar-btn" onClick={() => setShowHelp(true)}>Help</button>
          <button className="toolbar-btn toolbar-btn-logout" onClick={logout}>Logout</button>
        </div>
        {state.combatRound !== null && <CombatBanner />}
      </div>

      {/* Mobile tab bar — hidden on desktop via CSS */}
      <div className="panel-tabs">
        {(['room', 'map', 'character'] as MobilePanel[]).map((p) => (
          <button
            key={p}
            className={`panel-tab-btn${activeMobilePanel === p ? ' active' : ''}`}
            onClick={() => setActiveMobilePanel(p)}
          >
            {p.charAt(0).toUpperCase() + p.slice(1)}
          </button>
        ))}
      </div>

      {/* Main panels — panel-content is display:contents on desktop so children
          participate directly in the grid; on mobile it becomes a block container
          with only the active panel visible */}
      <div className="panel-content">
        <div className={`panel-room${activeMobilePanel === 'room' ? ' mobile-active' : ''}`}>
          <RoomPanel />
        </div>
        <div className={`panel-map${activeMobilePanel === 'map' ? ' mobile-active' : ''}`}>
          <MapPanel />
        </div>
        <div className={`panel-character${activeMobilePanel === 'character' ? ' mobile-active' : ''}`}>
          <CharacterPanel />
        </div>
      </div>

      {/* Feed + drawer overlay */}
      <div className="panel-feed">
        <FeedPanel />
        <DrawerContainer openDrawer={openDrawer} onClose={() => setOpenDrawer(null)} />
      </div>

      {/* Hotbar */}
      <div className="panel-hotbar"><HotbarPanel /></div>

      {/* Input */}
      <div className="panel-input"><InputPanel /></div>

      {showHelp && <HelpModal onClose={() => setShowHelp(false)} />}
      {state.shopView && <NpcModal />}
    </div>
  )
}

export function GamePage() {
  const { token } = useAuth()
  if (!token) return <Navigate to="/login" replace />

  return (
    <GameProvider>
      <GameLayout />
    </GameProvider>
  )
}
