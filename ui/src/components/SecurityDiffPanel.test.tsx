import { describe, it, expect } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import type { AttackSurfaceChange, SecurityDiff } from '../api/types'
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

describe('SecurityDiffPanel', () => {
  it('showsBaseHeadAndDeltaScores', () => {
    render(
      <SecurityDiffPanel
        diff={buildDiff({ score_base: 1.2, score_head: 1.7, score_delta: 0.5 })}
      />,
    )

    expect(screen.getByTestId('diff-score-base')).toHaveTextContent('1.20')
    expect(screen.getByTestId('diff-score-head')).toHaveTextContent('1.70')
    expect(screen.getByTestId('diff-score-delta')).toHaveTextContent('0.50')
  })

  // Locks the inverted-direction bug fix: score > 0 means posture got worse.
  it('marksPositiveDeltaAsRegressed', () => {
    render(<SecurityDiffPanel diff={buildDiff({ score_delta: 0.5 })} />)
    expect(screen.getByTestId('diff-score-delta')).toHaveAttribute(
      'data-direction',
      'regressed',
    )
  })

  it('marksNegativeDeltaAsImproved', () => {
    render(<SecurityDiffPanel diff={buildDiff({ score_delta: -0.5 })} />)
    expect(screen.getByTestId('diff-score-delta')).toHaveAttribute(
      'data-direction',
      'improved',
    )
  })

  it('marksZeroDeltaAsNeutral', () => {
    render(<SecurityDiffPanel diff={buildDiff({ score_delta: 0 })} />)
    expect(screen.getByTestId('diff-score-delta')).toHaveAttribute(
      'data-direction',
      'neutral',
    )
  })

  it('rendersEmptyStateWhenNoChanges', () => {
    render(
      <SecurityDiffPanel
        diff={buildDiff({ attack_surface_changed: false, attack_surface_changes: [] })}
      />,
    )
    expect(screen.getByTestId('no-changes')).toBeInTheDocument()
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

    const adminGroup = screen.getByTestId('change-group-admin_surface')
    const uploadGroup = screen.getByTestId('change-group-upload_surface')
    const authGroup = screen.getByTestId('change-group-auth_surface')

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

    const badges = screen.getAllByTestId('change-severity')
    expect(badges[0]).toHaveTextContent('high')
    expect(badges[1]).toHaveTextContent('medium')
    expect(badges[2]).toHaveTextContent('low')
  })

  it('showsPerChangeScoreContribution', () => {
    const diff = buildDiff({
      attack_surface_changes: [buildChange({ score: 0.3 })],
    })

    render(<SecurityDiffPanel diff={diff} />)

    expect(screen.getByTestId('change-score')).toHaveTextContent('0.30')
  })

  it('showsExposureDeltaValue', () => {
    render(<SecurityDiffPanel diff={buildDiff({ exposure_delta: 0.4 })} />)
    expect(screen.getByTestId('diff-exposure-delta')).toHaveTextContent('0.40')
  })

  it('showsHardeningDeltaValue', () => {
    render(<SecurityDiffPanel diff={buildDiff({ hardening_delta: -0.2 })} />)
    expect(screen.getByTestId('diff-hardening-delta')).toHaveTextContent('-0.20')
  })

  it('marksPositiveExposureDeltaAsRegressed', () => {
    render(<SecurityDiffPanel diff={buildDiff({ exposure_delta: 0.4 })} />)
    expect(screen.getByTestId('diff-exposure-delta')).toHaveAttribute(
      'data-direction',
      'regressed',
    )
  })

  // Hardening is inverted: head > base means stronger defences = improvement.
  it('marksPositiveHardeningDeltaAsImproved', () => {
    render(<SecurityDiffPanel diff={buildDiff({ hardening_delta: 0.2 })} />)
    expect(screen.getByTestId('diff-hardening-delta')).toHaveAttribute(
      'data-direction',
      'improved',
    )
  })

  it('marksNegativeHardeningDeltaAsRegressed', () => {
    render(<SecurityDiffPanel diff={buildDiff({ hardening_delta: -0.2 })} />)
    expect(screen.getByTestId('diff-hardening-delta')).toHaveAttribute(
      'data-direction',
      'regressed',
    )
  })
})
