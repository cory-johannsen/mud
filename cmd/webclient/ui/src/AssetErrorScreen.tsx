import type { JSX } from 'react'

export function AssetErrorScreen({ onRetry }: { onRetry: () => void }): JSX.Element {
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        gap: '1rem',
        fontFamily: 'monospace',
        backgroundColor: '#111',
        color: '#ccc',
      }}
    >
      <h1 style={{ color: '#ff6b6b', margin: 0 }}>Asset pack unavailable</h1>
      <p style={{ margin: 0, maxWidth: '420px', textAlign: 'center' }}>
        The PixiJS room scene requires the asset pack to render. The asset pack
        could not be loaded — check your connection or contact the server administrator.
      </p>
      <button
        onClick={onRetry}
        style={{
          padding: '0.5rem 1.5rem',
          cursor: 'pointer',
          backgroundColor: '#444',
          color: '#fff',
          border: '1px solid #666',
          borderRadius: '4px',
          fontFamily: 'monospace',
          fontSize: '1rem',
        }}
      >
        Retry
      </button>
    </div>
  )
}
