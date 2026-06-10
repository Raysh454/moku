import { describe, it, expect } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import type { AttackSurfaceChange, SecurityDiff } from '../../src/api/types'
import { SecurityDiffPanel } from './SecurityDiffPanel'

function buildChange(overrides: Partial<AttackSurfaceChange> = {}): AttackSurfaceChange {
  return {
    kind: 'form_added_admin',
    detail: 'New admin form at /admin',
    category: 'admin_surface',
    score: 0.3,
    ...overrides,
  }
}

function buildDiff(overrides: Partial<SecurityDiff> = {}): SecurityDiff {
  return {
    url: '/admin',
    base_version_id: 'v1',
    head_version_id: 'v2',
    base_snapshot_id: 's1',
    head_snapshot_id: 's2',
    score_base: 1.0,
    score_head: 1.5,
    score_delta: 0.5,
    exposure_delta: 0,
    hardening_delta: 0,
    attack_surface_changed: true,
    attack_surface_changes: [buildChange()],
    ...overrides,
  }
}

// The current panel renders each metric as a labelled card. Direction is
// expressed through a colour class on the value element rather than a
// data-direction attribute: regressed => text-danger, improved => text-success,
// neutral => text-helper.
const DIRECTION_CLASS = {
  regressed: 'text-danger',
  improved: 'text-success',
  neutral: 'text-helper',
} as const

function cardForLabel(label: string): HTMLElement {
  const labelNode = screen.getByText(label)
  const card = labelNode.parentElement
  expect(card).not.toBeNull()
  return card as HTMLElement
}

function valueElementForLabel(label: string): HTMLElement {
  // The value is the sibling element after the label inside the card.
  const value = cardForLabel(label).querySelector(':scope > div:last-child')
  expect(value).not.toBeNull()
  return value as HTMLElement
}

function valueForLabel(label: string): string {
  return valueElementForLabel(label).textContent?.trim() ?? ''
}

function directionForLabel(label: string): keyof typeof DIRECTION_CLASS {
  const className = valueElementForLabel(label).className
  for (const [direction, cls] of Object.entries(DIRECTION_CLASS)) {
    if (className.includes(cls)) return direction as keyof typeof DIRECTION_CLASS
  }
  throw new Error(`no direction class on "${label}" (class="${className}")`)
}

describe('SecurityDiffPanel', () => {
  it('showsBaseHeadAndDeltaScores', () => {
    render(
      <SecurityDiffPanel
        diff={buildDiff({ score_base: 1.2, score_head: 1.7, score_delta: 0.5 })}
      />,
    )

    expect(valueForLabel('Base')).toBe('1.20')
    expect(valueForLabel('Head')).toBe('1.70')
    expect(valueForLabel('Posture Δ')).toBe('0.50')
  })

  // Locks the inverted-direction fix: score > 0 means posture got worse.
  it('marksPositiveDeltaAsRegressed', () => {
    render(<SecurityDiffPanel diff={buildDiff({ score_delta: 0.5 })} />)
    expect(directionForLabel('Posture Δ')).toBe('regressed')
  })

  it('marksNegativeDeltaAsImproved', () => {
    render(<SecurityDiffPanel diff={buildDiff({ score_delta: -0.5 })} />)
    expect(directionForLabel('Posture Δ')).toBe('improved')
  })

  it('marksZeroDeltaAsNeutral', () => {
    render(<SecurityDiffPanel diff={buildDiff({ score_delta: 0 })} />)
    expect(directionForLabel('Posture Δ')).toBe('neutral')
  })

  it('rendersChangesGroupedByCategory', () => {
    const diff = buildDiff({
      attack_surface_changes: [
        buildChange({ kind: 'form_added_admin', category: 'admin_surface' }),
        buildChange({ kind: 'input_added_file', category: 'upload_surface' }),
        buildChange({ kind: 'input_added_password', category: 'auth_surface' }),
        buildChange({ kind: 'form_added_auth', category: 'auth_surface' }),
      ],
    })

    render(<SecurityDiffPanel diff={diff} />)

    // Each category is a section headed by an <h4> whose text is the category.
    const adminGroup = screen.getByText('admin_surface').closest('section') as HTMLElement
    const uploadGroup = screen.getByText('upload_surface').closest('section') as HTMLElement
    const authGroup = screen.getByText('auth_surface').closest('section') as HTMLElement

    expect(within(adminGroup).getAllByRole('listitem')).toHaveLength(1)
    expect(within(uploadGroup).getAllByRole('listitem')).toHaveLength(1)
    expect(within(authGroup).getAllByRole('listitem')).toHaveLength(2)
  })

  it('rendersSeverityBadgeForEachChange', () => {
    const diff = buildDiff({
      attack_surface_changes: [
        buildChange({ category: 'admin_surface' }),
        buildChange({ category: 'auth_surface' }),
        buildChange({ category: 'generic' }),
      ],
    })

    render(<SecurityDiffPanel diff={diff} />)

    // Each change row leads with a severity badge (high/medium/low).
    const items = screen.getAllByRole('listitem')
    expect(items[0]).toHaveTextContent('high')
    expect(items[1]).toHaveTextContent('medium')
    expect(items[2]).toHaveTextContent('low')
  })

  it('showsPerChangeScoreContribution', () => {
    const diff = buildDiff({
      attack_surface_changes: [buildChange({ score: 0.3 })],
    })

    render(<SecurityDiffPanel diff={diff} />)

    expect(screen.getAllByRole('listitem')[0]).toHaveTextContent('0.30')
  })

  it('showsExposureDeltaValue', () => {
    render(<SecurityDiffPanel diff={buildDiff({ exposure_delta: 0.4 })} />)
    expect(valueForLabel('Exposure Δ')).toBe('0.40')
  })

  it('showsHardeningDeltaValue', () => {
    render(<SecurityDiffPanel diff={buildDiff({ hardening_delta: -0.2 })} />)
    expect(valueForLabel('Hardening Δ')).toBe('-0.20')
  })

  it('marksPositiveExposureDeltaAsRegressed', () => {
    render(<SecurityDiffPanel diff={buildDiff({ exposure_delta: 0.4 })} />)
    expect(directionForLabel('Exposure Δ')).toBe('regressed')
  })

  // Hardening is inverted: head > base means stronger defences = improvement.
  it('marksPositiveHardeningDeltaAsImproved', () => {
    render(<SecurityDiffPanel diff={buildDiff({ hardening_delta: 0.2 })} />)
    expect(directionForLabel('Hardening Δ')).toBe('improved')
  })

  it('marksNegativeHardeningDeltaAsRegressed', () => {
    render(<SecurityDiffPanel diff={buildDiff({ hardening_delta: -0.2 })} />)
    expect(directionForLabel('Hardening Δ')).toBe('regressed')
  })
})
