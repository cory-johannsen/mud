import { useState } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { GameProvider, useGame } from '../game/GameContext'
import { RoomPanel } from '../game/panels/RoomPanel'
import { MapPanel } from '../game/panels/MapPanel'
import { FeedPanel } from '../game/panels/FeedPanel'
import { CharacterPanel } from '../game/panels/CharacterPanel'
import { InputPanel } from '../game/panels/InputPanel'
import { CombatBanner } from '../game/CombatBanner'
import { DrawerContainer } from '../game/drawers/DrawerContainer'
import '../styles/game.css'

type DrawerType = 'inventory' | 'equipment' | 'skills' | 'feats'

// Inner component that has access to GameContext.
function GameLayout() {
  const { state } = useGame()
  const { logout } = useAuth()
  const [openDrawer, setOpenDrawer] = useState<DrawerType | null>(null)

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
          {(['inventory', 'equipment', 'skills', 'feats'] as DrawerType[]).map((d) => (
            <button
              key={d}
              className={`toolbar-btn${openDrawer === d ? ' active' : ''}`}
              onClick={() => toggleDrawer(d)}
            >
              {d.charAt(0).toUpperCase() + d.slice(1)}
            </button>
          ))}
          <button className="toolbar-btn toolbar-btn-logout" onClick={logout}>Logout</button>
        </div>
        {state.combatRound !== null && <CombatBanner />}
      </div>

      {/* Main panels */}
      <div className="panel-room"><RoomPanel /></div>
      <div className="panel-map"><MapPanel /></div>
      <div className="panel-character"><CharacterPanel /></div>

      {/* Feed + drawer overlay */}
      <div className="panel-feed">
        <FeedPanel />
        <DrawerContainer openDrawer={openDrawer} onClose={() => setOpenDrawer(null)} />
      </div>

      {/* Input */}
      <div className="panel-input"><InputPanel /></div>
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
