import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { AttackSurfaceChange } from '../api/types'
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
  it('rendersSeverityBadgeForChange', () => {
    render(
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

    const badges = screen.getAllByTestId('change-severity-badge')
    expect(badges).toHaveLength(3)
    expect(badges[0]).toHaveTextContent('high')
    expect(badges[1]).toHaveTextContent('medium')
    expect(badges[2]).toHaveTextContent('low')
  })
})
