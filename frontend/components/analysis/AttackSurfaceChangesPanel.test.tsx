import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/react'
import type { AttackSurfaceChange } from '../../src/api/types'
import { AttackSurfaceChangesPanel } from './AttackSurfaceChangesPanel'

function buildChange(overrides: Partial<AttackSurfaceChange> = {}): AttackSurfaceChange {
  return {
    kind: 'form_added_admin',
    detail: 'New admin form at /admin',
    category: 'admin_surface',
    score: 0.3,
    ...overrides,
  }
}

const noop = () => {}

describe('AttackSurfaceChangesPanel', () => {
  it('exposes each change severity for its taxonomy category', () => {
    // Each change row carries a stable `data-severity` reflecting its
    // category's taxonomy severity (high/medium/low).
    const { container } = render(
      <AttackSurfaceChangesPanel
        changes={[
          buildChange({ category: 'admin_surface' }),
          buildChange({ category: 'auth_surface' }),
          buildChange({ category: 'generic' }),
        ]}
        activeChangeIndex={null}
        hoveredChange={null}
        onChangeClick={noop}
        onChangeHoverEnter={noop}
        onChangeHoverLeave={noop}
      />,
    )

    const rows = container.querySelectorAll('[data-severity]')
    expect(rows).toHaveLength(3)
    expect(rows[0].getAttribute('data-severity')).toBe('high')
    expect(rows[1].getAttribute('data-severity')).toBe('medium')
    expect(rows[2].getAttribute('data-severity')).toBe('low')
  })
})
