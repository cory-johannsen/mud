import { useState } from 'react'
import { useAuth } from '../auth/AuthContext'
import { PlayersTab } from '../admin/PlayersTab'
import { AccountsTab } from '../admin/AccountsTab'
import { ZoneEditorTab } from '../admin/ZoneEditorTab'
import { NpcSpawnerTab } from '../admin/NpcSpawnerTab'
import { LiveLogTab } from '../admin/LiveLogTab'

type Tab = 'players' | 'accounts' | 'zones' | 'npcs' | 'log'

interface TabDef {
  id: Tab
  label: string
}

const TABS: TabDef[] = [
  { id: 'players', label: 'Online Players' },
  { id: 'accounts', label: 'Accounts' },
  { id: 'zones', label: 'Zone Editor' },
  { id: 'npcs', label: 'NPC Spawner' },
  { id: 'log', label: 'Live Log' },
]

export function AdminPage() {
  const { user } = useAuth()
  const [activeTab, setActiveTab] = useState<Tab>('players')

  if (!user || (user.role !== 'admin' && user.role !== 'moderator')) {
    return (
      <div style={{ color: '#f55', padding: '2rem', fontFamily: 'monospace' }}>
        403 Forbidden — admin or moderator role required.
      </div>
    )
  }

  const renderTab = () => {
    switch (activeTab) {
      case 'players':  return <PlayersTab />
      case 'accounts': return <AccountsTab />
      case 'zones':    return <ZoneEditorTab />
      case 'npcs':     return <NpcSpawnerTab />
      case 'log':      return <LiveLogTab />
    }
  }

  return (
    <div style={{ minHeight: '100vh', background: '#121212', display: 'flex', flexDirection: 'column' }}>
      {/* Header */}
      <div style={{ background: '#1e1e1e', borderBottom: '1px solid #333', padding: '0.6rem 1.2rem', display: 'flex', gap: '1rem', alignItems: 'center' }}>
        <span style={{ color: '#c44', fontFamily: 'monospace', fontWeight: 'bold', marginRight: '1rem' }}>
          ADMIN
        </span>
        {TABS.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            style={{
              background: activeTab === tab.id ? '#2a3a4a' : 'transparent',
              color: activeTab === tab.id ? '#7af' : '#aaa',
              border: 'none',
              borderBottom: activeTab === tab.id ? '2px solid #7af' : '2px solid transparent',
              padding: '0.4rem 0.8rem',
              cursor: 'pointer',
              fontSize: '0.9em',
              fontFamily: 'inherit',
            }}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Content */}
      <div style={{ flex: 1 }}>
        {renderTab()}
      </div>
    </div>
  )
}
