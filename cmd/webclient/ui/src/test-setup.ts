import '@testing-library/jest-dom'

// ResizeObserver is not implemented in jsdom — provide a no-op stub so
// components that use it (e.g. MapPanel) don't throw in tests.
if (typeof global.ResizeObserver === 'undefined') {
  global.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}
