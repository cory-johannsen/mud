import { useState, useEffect } from 'react'
import { Navigate } from 'react-router-dom'
import { Panel, Group as PanelGroup, Separator as PanelResizeHandle, useDefaultLayout } from 'react-resizable-panels'
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
import { NpcInteractModal } from '../game/NpcInteractModal'
import { FeatureChoiceModal } from '../game/drawers/FeatureChoiceModal'
import { LogoutDropdown } from '../components/LogoutDropdown'
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

function useIsDesktop() {
  const [isDesktop, setIsDesktop] = useState(() => window.innerWidth > 1023)
  useEffect(() => {
    const mq = window.matchMedia('(min-width: 1024px)')
    const handler = (e: MediaQueryListEvent) => setIsDesktop(e.matches)
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [])
  return isDesktop
}

// Inner component that has access to GameContext.
function GameLayout() {
  const { state, sendMessage } = useGame()
  const activeWeather = state.activeWeather
  const [openDrawer, setOpenDrawer] = useState<DrawerType | null>(null)
  const { defaultLayout: verticalLayout, onLayoutChanged: onVerticalLayoutChanged } = useDefaultLayout({
    id: 'game-vertical',
    storage: localStorage,
  })
  const { defaultLayout: horizontalLayout, onLayoutChanged: onHorizontalLayoutChanged } = useDefaultLayout({
    id: 'game-horizontal',
    storage: localStorage,
  })
  const [activeMobilePanel, setActiveMobilePanel] = useState<MobilePanel>('room')
  const [showHelp, setShowHelp] = useState(false)
  const isDesktop = useIsDesktop()

  function toggleDrawer(d: DrawerType) {
    setOpenDrawer((prev) => (prev === d ? null : d))
  }

  const toolbar = (
    <div className="panel-toolbar">
      <div className="toolbar">
        <span className="toolbar-zone">
          {state.roomView?.zoneName ?? state.roomView?.zone_name ?? 'Connecting…'}
        </span>
        {state.timeOfDay && (
          <span className="toolbar-time">{formatTimeOfDay(state.timeOfDay)}</span>
        )}
        {activeWeather && (
          <span style={{
            background: 'rgba(0,0,0,0.7)',
            color: '#f0a500',
            border: '1px solid #f0a500',
            borderRadius: '12px',
            padding: '2px 12px',
            fontSize: '0.8rem',
            fontFamily: 'monospace',
            fontWeight: 'bold',
            letterSpacing: '0.05em',
            whiteSpace: 'nowrap',
          }}>
            {'\u26C8'} {activeWeather}
          </span>
        )}
        {(['inventory', 'equipment', 'skills', 'feats', 'stats', 'technology', 'job'] as DrawerType[]).map((d) => (
          <button
            key={d}
            className={`toolbar-btn${openDrawer === d ? ' active' : ''}`}
            onClick={() => toggleDrawer(d)}
          >
            {d.charAt(0).toUpperCase() + d.slice(1)}
          </button>
        ))}
        <button className="toolbar-btn" onClick={() => setShowHelp(true)}>Help</button>
        <LogoutDropdown />
      </div>
      {state.combatRound !== null && <CombatBanner />}
    </div>
  )

  const modals = (
    <>
      {showHelp && (
        <HelpModal
          onClose={() => setShowHelp(false)}
          onAssignHotbar={(slot, text) => {
            sendMessage('HotbarRequest', { action: 'set', slot, text })
            setShowHelp(false)
          }}
        />
      )}
      {state.shopView && <NpcModal />}
      <NpcInteractModal />
      {state.choicePrompt && <FeatureChoiceModal onClose={() => { /* modal self-closes on selection */ }} />}
    </>
  )

  if (isDesktop) {
    return (
      <div className="game-layout game-layout--desktop">
        {toolbar}
        <PanelGroup
          orientation="vertical"
          className="game-panels-vertical"
          defaultLayout={verticalLayout}
          onLayoutChanged={onVerticalLayoutChanged}
        >
          <Panel id="top" defaultSize={35} minSize={15}>
            <PanelGroup
              orientation="horizontal"
              className="game-panels-horizontal"
              defaultLayout={horizontalLayout}
              onLayoutChanged={onHorizontalLayoutChanged}
            >
              <Panel id="room" defaultSize={25} minSize={10}>
                <div className="panel-room"><RoomPanel /></div>
              </Panel>
              <PanelResizeHandle className="resize-handle resize-handle-h" />
              <Panel id="map" defaultSize={40} minSize={15}>
                <div className="panel-map"><MapPanel /></div>
              </Panel>
              <PanelResizeHandle className="resize-handle resize-handle-h" />
              <Panel id="character" defaultSize={35} minSize={10}>
                <div className="panel-character"><CharacterPanel /></div>
              </Panel>
            </PanelGroup>
          </Panel>
          <PanelResizeHandle className="resize-handle resize-handle-v" />
          <Panel id="feed" defaultSize={65} minSize={15}>
            <div className="panel-feed">
              <FeedPanel />
              <DrawerContainer openDrawer={openDrawer} onClose={() => setOpenDrawer(null)} />
            </div>
          </Panel>
        </PanelGroup>
        <div className="panel-hotbar"><HotbarPanel /></div>
        <div className="panel-input"><InputPanel /></div>
        {modals}
      </div>
    )
  }

  // Non-desktop (tablet / phone): original grid layout
  return (
    <div className="game-layout">
      {toolbar}
      {/* Mobile tab bar — hidden on tablet via CSS, shown on phone */}
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
      {/* panel-content is display:contents on tablet so children participate
          directly in the grid; on phone it becomes a block with tab switching */}
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
      <div className="panel-feed">
        <FeedPanel />
        <DrawerContainer openDrawer={openDrawer} onClose={() => setOpenDrawer(null)} />
      </div>
      <div className="panel-hotbar"><HotbarPanel /></div>
      <div className="panel-input"><InputPanel /></div>
      {modals}
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
