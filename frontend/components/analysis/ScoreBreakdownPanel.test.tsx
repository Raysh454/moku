import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { ScoreResult } from '../../src/api/types'
import { ScoreBreakdownPanel } from './ScoreBreakdownPanel'

function buildResult(overrides: Partial<ScoreResult> = {}): ScoreResult {
  return {
    score: 1.23,
    exposure_score: 2.5,
    hardening_score: 0.85,
    normalized: 42,
    confidence: 0.9,
    version: 'v1',
    snapshot_id: 'snap-1',
    version_id: 'ver-1',
    ...overrides,
  }
}

// The current panel renders each metric as a labelled card: an uppercase
// label element followed by its value sibling. Resolve a value by its label
// text rather than a data-testid (the current markup has none).
function valueForLabel(label: string): string {
  const labelNode = screen.getByText(label)
  const card = labelNode.parentElement
  expect(card).not.toBeNull()
  // The value sits in the sibling div after the label inside the same card.
  return card?.textContent?.replace(label, '').trim() ?? ''
}

describe('ScoreBreakdownPanel', () => {
  it('rendersCompositeScoreWithTwoDecimals', () => {
    render(<ScoreBreakdownPanel result={buildResult({ score: 1.23456 })} />)
    expect(valueForLabel('Posture Score')).toBe('1.23')
  })

  it('rendersExposureScore', () => {
    render(<ScoreBreakdownPanel result={buildResult({ exposure_score: 3.14 })} />)
    expect(valueForLabel('Exposure')).toBe('3.14')
  })

  it('rendersHardeningScoreAsPercentage', () => {
    render(<ScoreBreakdownPanel result={buildResult({ hardening_score: 0.85 })} />)
    expect(valueForLabel('Hardening')).toBe('85%')
  })

  it('rendersConfidence', () => {
    render(<ScoreBreakdownPanel result={buildResult({ confidence: 0.9 })} />)
    expect(valueForLabel('Confidence')).toBe('90%')
  })

  it('rendersNothingWhenResultIsNull', () => {
    const { container } = render(<ScoreBreakdownPanel result={null} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('rendersEvidenceListWhenPresent', () => {
    const result = buildResult({
      evidence: [
        {
          id: 'e1',
          key: 'form_admin',
          severity: 'high',
          description: 'Admin form exposed',
          contribution: 0.3,
        },
        {
          id: 'e2',
          key: 'input_file',
          severity: 'high',
          description: 'File upload input detected',
          contribution: 0.5,
        },
      ],
    })

    render(<ScoreBreakdownPanel result={result} />)

    expect(screen.getByText('Admin form exposed')).toBeInTheDocument()
    expect(screen.getByText('File upload input detected')).toBeInTheDocument()
    // Each evidence item is a list entry; two items => two list entries.
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
  })

  it('omitsEvidenceListWhenEmpty', () => {
    render(<ScoreBreakdownPanel result={buildResult({ evidence: [] })} />)
    expect(screen.queryAllByRole('listitem')).toHaveLength(0)
  })
})
