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
  it('appliesSeverityClassForEachChange', () => {
    // The current panel encodes per-change severity as a `severity--<level>`
    // class on each change row (there is no separate severity badge element),
    // so assert the class rather than badge text. Intent preserved: each
    // category maps to its taxonomy severity (high/medium/low).
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

    const rows = container.querySelectorAll('.changeItem')
    expect(rows).toHaveLength(3)
    expect(rows[0].className).toContain('severity--high')
    expect(rows[1].className).toContain('severity--medium')
    expect(rows[2].className).toContain('severity--low')
  })
})
