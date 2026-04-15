import { describe, it, expect } from 'vitest'
import type { AttackSurfaceChange } from '../api/types'
import {
  formatScore,
  scoreDirection,
  severityForCategory,
  groupChangesByCategory,
} from './score'

function change(
  kind: string,
  category: AttackSurfaceChange['category'],
): AttackSurfaceChange {
  return { kind, detail: kind, category, score: 0.1 }
}

describe('formatScore', () => {
  it('displaysTwoDecimalsForFiniteNumber', () => {
    expect(formatScore(1.23456)).toBe('1.23')
  })

  it('rendersDashForUndefined', () => {
    expect(formatScore(undefined)).toBe('—')
  })

  it('rendersDashForNaN', () => {
    expect(formatScore(Number.NaN)).toBe('—')
  })
})

describe('scoreDirection', () => {
  // Higher score = more exposure = worse posture.
  // Go defines Regressed = (scoreDelta > 0) in assessor_models.go:193.
  it('positiveDeltaIsRegressed', () => {
    expect(scoreDirection(0.5)).toBe('regressed')
  })

  it('negativeDeltaIsImproved', () => {
    expect(scoreDirection(-0.5)).toBe('improved')
  })

  it('zeroDeltaIsNeutral', () => {
    expect(scoreDirection(0)).toBe('neutral')
  })

  // Hardening is inverted: higher value = stronger defenses = better posture,
  // so a positive delta is an improvement, not a regression.
  it('higherIsBetter_positiveDeltaIsImproved', () => {
    expect(scoreDirection(0.5, { higherIsWorse: false })).toBe('improved')
  })

  it('higherIsBetter_negativeDeltaIsRegressed', () => {
    expect(scoreDirection(-0.5, { higherIsWorse: false })).toBe('regressed')
  })

  it('higherIsBetter_zeroDeltaIsNeutral', () => {
    expect(scoreDirection(0, { higherIsWorse: false })).toBe('neutral')
  })
})

describe('severityForCategory', () => {
  // Mirrors Go's change_taxonomy.go:101 SeverityForCategory.
  it('returnsHighForUploadSurface', () => {
    expect(severityForCategory('upload_surface')).toBe('high')
  })

  it('returnsHighForAdminSurface', () => {
    expect(severityForCategory('admin_surface')).toBe('high')
  })

  it('returnsHighForSecurityRegression', () => {
    expect(severityForCategory('security_regression')).toBe('high')
  })

  it('returnsHighForCookieRegression', () => {
    expect(severityForCategory('cookie_regression')).toBe('high')
  })

  it('returnsMediumForAuthSurface', () => {
    expect(severityForCategory('auth_surface')).toBe('medium')
  })

  it('returnsMediumForCookieRisk', () => {
    expect(severityForCategory('cookie_risk')).toBe('medium')
  })

  it('returnsLowForUnknownCategory', () => {
    expect(severityForCategory('generic')).toBe('low')
  })
})

describe('groupChangesByCategory', () => {
  it('bucketsChangesByCategory', () => {
    const changes: AttackSurfaceChange[] = [
      change('form_added_admin', 'admin_surface'),
      change('input_added_file', 'upload_surface'),
      change('form_added_auth', 'auth_surface'),
      change('input_added_password', 'auth_surface'),
    ]

    const groups = groupChangesByCategory(changes)

    expect(groups.get('admin_surface')).toHaveLength(1)
    expect(groups.get('upload_surface')).toHaveLength(1)
    expect(groups.get('auth_surface')).toHaveLength(2)
  })

  it('preservesInsertionOrderWithinCategory', () => {
    const first = change('cookie_added', 'cookie_risk')
    const second = change('cookie_changed', 'cookie_risk')
    const third = change('cookie_added_no_httponly', 'cookie_risk')

    const groups = groupChangesByCategory([first, second, third])

    expect(groups.get('cookie_risk')).toEqual([first, second, third])
  })
})
