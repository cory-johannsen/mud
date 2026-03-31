import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

const dropdownPanelStyle: React.CSSProperties = {
  position: 'absolute',
  top: '100%',
  right: 0,
  marginTop: '2px',
  background: '#0d0d0d',
  border: '1px solid #333',
  borderRadius: '3px',
  minWidth: '140px',
  zIndex: 1000,
}

const dropdownItemStyle: React.CSSProperties = {
  display: 'block',
  width: '100%',
  padding: '0.4rem 0.75rem',
  background: 'none',
  border: 'none',
  color: '#aaa',
  fontFamily: 'monospace',
  fontSize: '0.8rem',
  cursor: 'pointer',
  textAlign: 'left',
}

export function LogoutDropdown() {
  const [open, setOpen] = useState(false)
  const { logout } = useAuth()
  const navigate = useNavigate()
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  function handleSwitchCharacter() {
    setOpen(false)
    navigate('/characters')
  }

  function handleLogout() {
    setOpen(false)
    logout()
  }

  return (
    <div ref={ref} style={{ position: 'relative', marginLeft: 'auto' }}>
      <button
        className="toolbar-btn toolbar-btn-logout"
        style={{ marginLeft: 0 }}
        onClick={() => setOpen((o) => !o)}
      >
        Logout ▾
      </button>
      {open && (
        <div style={dropdownPanelStyle}>
          <button
            style={dropdownItemStyle}
            onMouseEnter={(e) => { e.currentTarget.style.background = '#1a1a1a' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'none' }}
            onClick={handleSwitchCharacter}
          >
            Switch Character
          </button>
          <button
            style={dropdownItemStyle}
            onMouseEnter={(e) => { e.currentTarget.style.background = '#1a1a1a' }}
            onMouseLeave={(e) => { e.currentTarget.style.background = 'none' }}
            onClick={handleLogout}
          >
            Logout
          </button>
        </div>
      )}
    </div>
  )
}
