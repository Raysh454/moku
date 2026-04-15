import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import type { ScoreResult } from '../api/types'
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

describe('ScoreBreakdownPanel', () => {
  it('rendersCompositeScoreWithTwoDecimals', () => {
    render(<ScoreBreakdownPanel result={buildResult({ score: 1.23456 })} />)
    expect(screen.getByTestId('score-composite')).toHaveTextContent('1.23')
  })

  it('rendersExposureScore', () => {
    render(<ScoreBreakdownPanel result={buildResult({ exposure_score: 3.14 })} />)
    expect(screen.getByTestId('score-exposure')).toHaveTextContent('3.14')
  })

  it('rendersHardeningScoreAsPercentage', () => {
    render(<ScoreBreakdownPanel result={buildResult({ hardening_score: 0.85 })} />)
    expect(screen.getByTestId('score-hardening')).toHaveTextContent('85%')
  })

  it('rendersConfidence', () => {
    render(<ScoreBreakdownPanel result={buildResult({ confidence: 0.9 })} />)
    expect(screen.getByTestId('score-confidence')).toHaveTextContent('90%')
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

    expect(screen.getByTestId('evidence-list')).toBeInTheDocument()
    expect(screen.getByText('Admin form exposed')).toBeInTheDocument()
    expect(screen.getByText('File upload input detected')).toBeInTheDocument()
  })

  it('omitsEvidenceListWhenEmpty', () => {
    render(<ScoreBreakdownPanel result={buildResult({ evidence: [] })} />)
    expect(screen.queryByTestId('evidence-list')).not.toBeInTheDocument()
  })
})
