import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/react'
import { ReadyActionPicker } from './ReadyActionPicker'

// REQ-RDY-PICKER-1: Renders action step buttons on mount, advances to the
// trigger step, and emits `ready <action> when <trigger>` on confirm.
// REQ-RDY-PICKER-2: Attack trigger step accepts optional target text and
// appends it to the emitted command when provided.
// REQ-RDY-PICKER-3: Back button returns from trigger step to action step.
// REQ-RDY-PICKER-4: Cancel button invokes the onCancel callback.

describe('ReadyActionPicker', () => {
  it('renders the four action buttons on mount', () => {
    render(<ReadyActionPicker onSubmit={vi.fn()} onCancel={vi.fn()} />)
    expect(screen.getByText('Attack')).toBeDefined()
    expect(screen.getByText('Stride Toward')).toBeDefined()
    expect(screen.getByText('Stride Away')).toBeDefined()
    expect(screen.getByText('Reload')).toBeDefined()
  })

  it('advances to the trigger step after an action is chosen', () => {
    render(<ReadyActionPicker onSubmit={vi.fn()} onCancel={vi.fn()} />)
    fireEvent.click(screen.getByText('Stride Toward'))
    expect(screen.getByText('Choose Trigger')).toBeDefined()
    expect(screen.getByText('Enemy enters room')).toBeDefined()
    expect(screen.getByText('Enemy moves adjacent')).toBeDefined()
    expect(screen.getByText('Ally damaged')).toBeDefined()
  })

  it('emits the canonical ready command on trigger confirm', () => {
    const onSubmit = vi.fn()
    render(<ReadyActionPicker onSubmit={onSubmit} onCancel={vi.fn()} />)
    fireEvent.click(screen.getByText('Attack'))
    fireEvent.click(screen.getByText('Enemy enters room'))
    expect(onSubmit).toHaveBeenCalledTimes(1)
    expect(onSubmit).toHaveBeenCalledWith(
      'ready attack when enemy enters room',
    )
  })

  it('appends a target to the attack action when provided', () => {
    const onSubmit = vi.fn()
    render(<ReadyActionPicker onSubmit={onSubmit} onCancel={vi.fn()} />)
    fireEvent.click(screen.getByText('Attack'))
    const input = screen.getByPlaceholderText('e.g. goblin') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'goblin' } })
    fireEvent.click(screen.getByText('Enemy moves adjacent'))
    expect(onSubmit).toHaveBeenCalledWith(
      'ready attack goblin when enemy moves adjacent',
    )
  })

  it('emits the non-attack action verbatim (no target field) on trigger', () => {
    const onSubmit = vi.fn()
    render(<ReadyActionPicker onSubmit={onSubmit} onCancel={vi.fn()} />)
    fireEvent.click(screen.getByText('Reload'))
    // No target input on non-attack actions
    expect(screen.queryByPlaceholderText('e.g. goblin')).toBeNull()
    fireEvent.click(screen.getByText('Ally damaged'))
    expect(onSubmit).toHaveBeenCalledWith('ready reload when ally damaged')
  })

  it('returns from the trigger step to the action step when Back is clicked', () => {
    render(<ReadyActionPicker onSubmit={vi.fn()} onCancel={vi.fn()} />)
    fireEvent.click(screen.getByText('Attack'))
    expect(screen.getByText('Choose Trigger')).toBeDefined()
    fireEvent.click(screen.getByText('← Back'))
    expect(screen.getByText('Ready Action')).toBeDefined()
    expect(screen.getByText('Attack')).toBeDefined()
    expect(screen.getByText('Stride Toward')).toBeDefined()
  })

  it('clears the target field when navigating back from the trigger step', () => {
    const onSubmit = vi.fn()
    render(<ReadyActionPicker onSubmit={onSubmit} onCancel={vi.fn()} />)
    fireEvent.click(screen.getByText('Attack'))
    const input = screen.getByPlaceholderText('e.g. goblin') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'goblin' } })
    fireEvent.click(screen.getByText('← Back'))
    fireEvent.click(screen.getByText('Attack'))
    const reopened = screen.getByPlaceholderText('e.g. goblin') as HTMLInputElement
    expect(reopened.value).toBe('')
    fireEvent.click(screen.getByText('Enemy enters room'))
    expect(onSubmit).toHaveBeenCalledWith('ready attack when enemy enters room')
  })

  it('calls onCancel when the × button is clicked', () => {
    const onCancel = vi.fn()
    render(<ReadyActionPicker onSubmit={vi.fn()} onCancel={onCancel} />)
    fireEvent.click(screen.getByLabelText('Cancel'))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })
})
