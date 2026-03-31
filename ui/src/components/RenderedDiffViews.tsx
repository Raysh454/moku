import { useState, useMemo, useCallback } from 'react'
import type { AttackSurfaceChange, Snapshot, SecurityDiff, CombinedFileDiff } from '../api/types'
import RenderedFrame, { type HighlightedElement, type TextChange } from './RenderedFrame'
import DOMTreeView from './DOMTreeView'
import { parseHtmlToTree, diffDomTrees, getChangeSummary } from './DOMParser'

export type RenderedViewMode = 
  | 'preview'           // Single rendered view with highlights
  | 'side-by-side'      // Two rendered views
  | 'overlay'           // Opacity slider comparison
  | 'dom-tree'          // DOM tree view
  | 'security-focus'    // Security elements only
  | 'timeline'          // Step through changes

type RenderedDiffViewsProps = {
  baseSnapshot?: Snapshot | null
  headSnapshot: Snapshot
  securityDiff?: SecurityDiff | null
  diff?: CombinedFileDiff | null
  viewMode: RenderedViewMode
  onViewModeChange: (mode: RenderedViewMode) => void
}

// Map AttackSurfaceChange to HighlightedElement
function changeToHighlight(change: AttackSurfaceChange): HighlightedElement[] {
  if (!change.evidence_locations) return []
  
  return change.evidence_locations
    .filter(loc => loc.dom_index !== undefined)
    .map(loc => ({
      type: loc.type,
      domIndex: loc.dom_index,
      parentDomIndex: loc.parent_dom_index,
      change
    }))
}

// Filter highlights by change type for contextual display
function filterHighlightsByType(highlights: HighlightedElement[], includeAdded: boolean, includeRemoved: boolean, includeChanged: boolean): HighlightedElement[] {
  return highlights.filter(h => {
    if (!h.change) return false
    
    const kind = h.change.kind.toLowerCase()
    
    if (includeAdded && kind.includes('_added')) return true
    if (includeRemoved && kind.includes('_removed')) return true  
    if (includeChanged && kind.includes('_changed')) return true
    
    return false
  })
}

// Get highlights for "head" context (added + changed)
function getAddedChangedHighlights(highlights: HighlightedElement[]): HighlightedElement[] {
  return filterHighlightsByType(highlights, true, false, true)
}

// Get highlights for "base" context (removed only)
function getRemovedHighlights(highlights: HighlightedElement[]): HighlightedElement[] {
  return filterHighlightsByType(highlights, false, true, false)
}

// Filter text changes by type for contextual display
function filterTextChangesByType(textChanges: TextChange[], includeAdded: boolean, includeRemoved: boolean, includeModified: boolean): TextChange[] {
  return textChanges.filter(change => {
    if (includeAdded && change.type === 'added') return true
    if (includeRemoved && change.type === 'removed') return true
    if (includeModified && change.type === 'modified') return true // Future support
    
    return false
  })
}

// Get text changes for "head" context (added + modified) 
function getAddedModifiedTextChanges(textChanges: TextChange[]): TextChange[] {
  return filterTextChangesByType(textChanges, true, false, true)
}

// Get text changes for "base" context (removed only)
function getRemovedTextChanges(textChanges: TextChange[]): TextChange[] {
  return filterTextChangesByType(textChanges, false, true, false)
}

export default function RenderedDiffViews({
  baseSnapshot,
  headSnapshot,
  securityDiff,
  diff: _diff,
  viewMode,
  onViewModeChange
}: RenderedDiffViewsProps) {
  const [activeChangeIndex, setActiveChangeIndex] = useState<number | null>(null)
  const [hoveredChange, setHoveredChange] = useState<AttackSurfaceChange | null>(null)
  const [overlayOpacity, setOverlayOpacity] = useState(0.5)
  const [timelineStep, setTimelineStep] = useState(0)
  const [showOnlyChanged, setShowOnlyChanged] = useState(false)
  const [showHighlights, setShowHighlights] = useState(true)
  const [showTextHighlights, setShowTextHighlights] = useState(true)

  // Decode body from base64 if needed
  const decodeBody = useCallback((body?: string): string => {
    if (!body) return '<p>No content</p>'
    // Check if it's base64 (simple heuristic)
    try {
      // If it starts with HTML-like content, return as-is
      if (body.startsWith('<') || body.startsWith('!') || body.includes('<!DOCTYPE')) {
        return body
      }
      // Try base64 decode
      return atob(body)
    } catch {
      return body
    }
  }, [])

  const headHtml = useMemo(() => decodeBody(headSnapshot.body), [headSnapshot.body, decodeBody])
  const baseHtml = useMemo(() => baseSnapshot ? decodeBody(baseSnapshot.body) : '', [baseSnapshot?.body, decodeBody])
  
  // Extract text changes from body_diff
  const textChanges = useMemo((): TextChange[] => {
    if (!_diff?.body_diff?.chunks) return []
    
    const chunks = _diff.body_diff.chunks
    const changes: TextChange[] = []
    const processedIndexes = new Set<number>()
    
    // First pass: detect changes (removed + added pairs)
    for (let i = 0; i < chunks.length - 1; i++) {
      if (processedIndexes.has(i) || processedIndexes.has(i + 1)) continue
      
      const current = chunks[i]
      const next = chunks[i + 1]
      
      // Look for removed followed by added at similar positions
      if (current.type === 'removed' && next.type === 'added') {
        // Check if the positions indicate they're replacements
        // Allow some flexibility in position matching
        const positionDiff = Math.abs((current.base_start || 0) - (next.base_start || 0))
        
        // Consider it a change if positions are close (within 20 characters)
        if (positionDiff <= 20) {
          const removedText = (current.content || '').replace(/<[^>]*>/g, '').trim()
          const addedText = (next.content || '').replace(/<[^>]*>/g, '').trim()
          
          // Only treat as change if both have meaningful text content
          if (removedText.length >= 1 && addedText.length >= 1) {
            changes.push({
              type: 'modified',
              content: `${removedText} → ${addedText}`,
              position: next.head_start || next.base_start || 0, // Use position where new text appears
              length: addedText.length
            })
            processedIndexes.add(i)
            processedIndexes.add(i + 1)
            continue
          }
        }
      }
    }
    
    // Second pass: handle remaining standalone adds/removes
    for (let i = 0; i < chunks.length; i++) {
      if (processedIndexes.has(i)) continue
      
      const chunk = chunks[i]
      if (!chunk.content) continue
      
      if (chunk.type === 'added' || chunk.type === 'removed') {
        const textContent = chunk.content.replace(/<[^>]*>/g, '').trim()
        if (textContent.length >= 3) { // Skip very short snippets
          changes.push({
            type: chunk.type as 'added' | 'removed' | 'modified',
            content: textContent,
            position: chunk.type === 'added' ? (chunk.head_start || 0) : (chunk.base_start || 0),
            length: textContent.length
          })
        }
      }
    }
    
    return changes
  }, [_diff])

  // Build all highlights from attack surface changes 
  const allHighlights = useMemo<HighlightedElement[]>(() => {
    if (!securityDiff?.attack_surface_changes) return []
    return securityDiff.attack_surface_changes.flatMap(changeToHighlight)
  }, [securityDiff])
  
  // Contextual highlights for different frame contexts
  const addedChangedHighlights = useMemo(() => getAddedChangedHighlights(allHighlights), [allHighlights])
  const removedHighlights = useMemo(() => getRemovedHighlights(allHighlights), [allHighlights])
  
  // Contextual text changes for different frame contexts
  const addedModifiedTextChanges = useMemo(() => getAddedModifiedTextChanges(textChanges), [textChanges])
  const removedTextChanges = useMemo(() => getRemovedTextChanges(textChanges), [textChanges])

  // Active highlight for navigation (use all highlights for navigation)
  const activeHighlight = useMemo(() => {
    if (activeChangeIndex === null || !securityDiff?.attack_surface_changes) return null
    const change = securityDiff.attack_surface_changes[activeChangeIndex]
    const elements = changeToHighlight(change)
    return elements[0] || null
  }, [activeChangeIndex, securityDiff])

  // DOM trees for tree view
  const baseTree = useMemo(() => baseHtml ? parseHtmlToTree(baseHtml, { includeText: true }) : null, [baseHtml])
  const headTree = useMemo(() => parseHtmlToTree(headHtml, { includeText: true }), [headHtml])
  const diffTree = useMemo(() => diffDomTrees(baseTree, headTree), [baseTree, headTree])
  const changeSummary = useMemo(() => diffTree ? getChangeSummary(diffTree) : null, [diffTree])

  // Handle change click - navigate to element
  const handleChangeClick = useCallback((idx: number) => {
    setActiveChangeIndex(idx === activeChangeIndex ? null : idx)
  }, [activeChangeIndex])

  // Timeline changes
  const timelineChanges = securityDiff?.attack_surface_changes || []

  const viewModes: { id: RenderedViewMode; label: string; icon: string }[] = [
    { id: 'preview', label: 'Preview', icon: '👁' },
    { id: 'side-by-side', label: 'Side by Side', icon: '⟷' },
    { id: 'overlay', label: 'Overlay', icon: '▣' },
    { id: 'dom-tree', label: 'DOM Tree', icon: '🌳' },
    { id: 'security-focus', label: 'Security', icon: '🔒' },
    { id: 'timeline', label: 'Timeline', icon: '📖' },
  ]

  return (
    <div className="renderedDiffViews">
      {/* View Mode Selector */}
      <div className="viewModeSelector">
        {viewModes.map(mode => (
          <button
            key={mode.id}
            className={`viewModeBtn ${viewMode === mode.id ? 'active' : ''}`}
            onClick={() => onViewModeChange(mode.id)}
            title={mode.label}
          >
            <span className="viewModeIcon">{mode.icon}</span>
            <span className="viewModeLabel">{mode.label}</span>
          </button>
        ))}
        
        {/* Highlight Toggles */}
        {(viewMode === 'preview' || viewMode === 'side-by-side' || viewMode === 'overlay' || viewMode === 'timeline') && (
          <>
            <label className="highlightToggle">
              <input
                type="checkbox"
                checked={showHighlights}
                onChange={(e) => setShowHighlights(e.target.checked)}
              />
              <span className="toggleLabel">Security Highlights</span>
            </label>
            <label className="highlightToggle">
              <input
                type="checkbox"
                checked={showTextHighlights}
                onChange={(e) => setShowTextHighlights(e.target.checked)}
              />
              <span className="toggleLabel">Text Changes</span>
            </label>
          </>
        )}
      </div>

      {/* Attack Surface Changes Panel */}
      {securityDiff?.attack_surface_changes && securityDiff.attack_surface_changes.length > 0 && viewMode !== 'timeline' && (
        <div className="attackChangesPanel">
          <h4>Attack Surface Changes ({securityDiff.attack_surface_changes.length})</h4>
          <div className="changesList">
            {securityDiff.attack_surface_changes.map((change, idx) => (
              <div
                key={idx}
                className={`changeItem ${activeChangeIndex === idx ? 'active' : ''} ${hoveredChange === change ? 'hovered' : ''}`}
                onClick={() => handleChangeClick(idx)}
                onMouseEnter={() => setHoveredChange(change)}
                onMouseLeave={() => setHoveredChange(null)}
              >
                <span className={`changeKindBadge kind-${change.kind.split('_')[0]}`}>
                  {change.kind.replace(/_/g, ' ')}
                </span>
                <span className="changeDetail">{change.detail}</span>
                {change.evidence_locations && change.evidence_locations.length > 0 && (
                  <span className="changeLocationCount">
                    📍 {change.evidence_locations.length}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* View Content */}
      <div className="viewContent">
        {/* Preview Mode - Single rendered view with highlights */}
        {viewMode === 'preview' && (
          <RenderedFrame
            html={headHtml}
            title="Current Version"
            highlights={addedChangedHighlights}
            activeHighlight={activeHighlight}
            showHighlights={showHighlights}
            textChanges={addedModifiedTextChanges}
            showTextHighlights={showTextHighlights}
            className="fullWidthFrame"
          />
        )}

        {/* Side by Side Mode */}
        {viewMode === 'side-by-side' && (
          <div className="sideBySideFrames">
            <RenderedFrame
              html={baseHtml || '<p>No base version available</p>'}
              title="Base Version"
              highlights={removedHighlights}
              activeHighlight={activeHighlight}
              showHighlights={showHighlights}
              textChanges={removedTextChanges}
              showTextHighlights={showTextHighlights}
              className="halfWidthFrame"
            />
            <RenderedFrame
              html={headHtml}
              title="Head Version"
              highlights={addedChangedHighlights}
              activeHighlight={activeHighlight}
              showHighlights={showHighlights}
              textChanges={addedModifiedTextChanges}
              showTextHighlights={showTextHighlights}
              className="halfWidthFrame"
            />
          </div>
        )}

        {/* Overlay Mode - Opacity slider */}
        {viewMode === 'overlay' && (
          <div className="overlayMode">
            <div className="overlayControls">
              <span>Base</span>
              <input
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={overlayOpacity}
                onChange={(e) => setOverlayOpacity(parseFloat(e.target.value))}
                className="overlaySlider"
              />
              <span>Head</span>
              <span className="overlayValue">{Math.round(overlayOpacity * 100)}%</span>
            </div>
            <div className="overlayFrames">
              <RenderedFrame
                html={baseHtml || '<p>No base version</p>'}
                title="Base"
                highlights={removedHighlights}
                activeHighlight={activeHighlight}
                showHighlights={showHighlights}
                textChanges={removedTextChanges}
                showTextHighlights={showTextHighlights}
                className="overlayFrameBase"
              />
              <div 
                className="overlayFrameHead"
                style={{ opacity: overlayOpacity }}
              >
                <RenderedFrame
                  html={headHtml}
                  title="Head"
                  highlights={addedChangedHighlights}
                  activeHighlight={activeHighlight}
                  showHighlights={showHighlights}
                  textChanges={addedModifiedTextChanges}
                  showTextHighlights={showTextHighlights}
                />
              </div>
            </div>
          </div>
        )}

        {/* DOM Tree Mode */}
        {viewMode === 'dom-tree' && (
          <div className="domTreeMode">
            <div className="domTreeControls">
              <label>
                <input
                  type="checkbox"
                  checked={showOnlyChanged}
                  onChange={(e) => setShowOnlyChanged(e.target.checked)}
                />
                Show only changed elements
              </label>
              {changeSummary && (
                <div className="changeSummary">
                  <span className="summaryAdded">+{changeSummary.added} added</span>
                  <span className="summaryRemoved">-{changeSummary.removed} removed</span>
                  <span className="summaryChanged">~{changeSummary.changed} changed</span>
                </div>
              )}
            </div>
            <DOMTreeView
              tree={diffTree}
              showOnlyChanged={showOnlyChanged}
              className="domTreeContainer"
            />
          </div>
        )}

        {/* Security Focus Mode */}
        {viewMode === 'security-focus' && (
          <div className="securityFocusMode">
            <SecurityElementsView
              baseSnapshot={baseSnapshot}
              headSnapshot={headSnapshot}
              securityDiff={securityDiff}
            />
          </div>
        )}

        {/* Timeline Mode */}
        {viewMode === 'timeline' && (
          <div className="timelineMode">
            <div className="timelineControls">
              <button 
                disabled={timelineStep === 0}
                onClick={() => setTimelineStep(s => Math.max(0, s - 1))}
              >
                ← Previous
              </button>
              <span className="timelineProgress">
                Change {timelineStep + 1} of {timelineChanges.length || 1}
              </span>
              <button 
                disabled={timelineStep >= timelineChanges.length - 1}
                onClick={() => setTimelineStep(s => Math.min(timelineChanges.length - 1, s + 1))}
              >
                Next →
              </button>
            </div>
            
            {timelineChanges.length > 0 ? (
              <div className="timelineCard">
                <div className="timelineChangeHeader">
                  <span className={`changeKindBadge kind-${timelineChanges[timelineStep].kind.split('_')[0]}`}>
                    {timelineChanges[timelineStep].kind.replace(/_/g, ' ')}
                  </span>
                </div>
                <p className="timelineChangeDetail">{timelineChanges[timelineStep].detail}</p>
                
                <RenderedFrame
                  html={headHtml}
                  highlights={changeToHighlight(timelineChanges[timelineStep])}
                  activeHighlight={changeToHighlight(timelineChanges[timelineStep])[0]}
                  showHighlights={showHighlights}
                  textChanges={textChanges}
                  showTextHighlights={showTextHighlights}
                  className="timelineFrame"
                />
              </div>
            ) : (
              <p className="noChanges">No attack surface changes to display</p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

// Security-focused elements view component
function SecurityElementsView({
  baseSnapshot: _baseSnapshot,
  headSnapshot: _headSnapshot,
  securityDiff
}: {
  baseSnapshot?: Snapshot | null
  headSnapshot: Snapshot
  securityDiff?: SecurityDiff | null
}) {
  // Group changes by type
  const groupedChanges = useMemo(() => {
    const groups: Record<string, AttackSurfaceChange[]> = {
      forms: [],
      inputs: [],
      cookies: [],
      headers: [],
      scripts: [],
      other: []
    }
    
    if (!securityDiff?.attack_surface_changes) return groups
    
    for (const change of securityDiff.attack_surface_changes) {
      if (change.kind.includes('form')) {
        groups.forms.push(change)
      } else if (change.kind.includes('input')) {
        groups.inputs.push(change)
      } else if (change.kind.includes('cookie')) {
        groups.cookies.push(change)
      } else if (change.kind.includes('header')) {
        groups.headers.push(change)
      } else if (change.kind.includes('script')) {
        groups.scripts.push(change)
      } else {
        groups.other.push(change)
      }
    }
    
    return groups
  }, [securityDiff])

  const sections = [
    { key: 'forms', label: 'Forms', icon: '📝', color: '#3b82f6' },
    { key: 'inputs', label: 'Inputs', icon: '⌨️', color: '#8b5cf6' },
    { key: 'cookies', label: 'Cookies', icon: '🍪', color: '#f59e0b' },
    { key: 'headers', label: 'Headers', icon: '📋', color: '#10b981' },
    { key: 'scripts', label: 'Scripts', icon: '📜', color: '#ef4444' },
    { key: 'other', label: 'Other', icon: '📦', color: '#6b7280' },
  ]

  return (
    <div className="securityElementsView">
      {sections.map(section => {
        const changes = groupedChanges[section.key]
        if (changes.length === 0) return null
        
        return (
          <div key={section.key} className="securitySection">
            <h4 style={{ borderLeftColor: section.color }}>
              {section.icon} {section.label} ({changes.length})
            </h4>
            <div className="securityCards">
              {changes.map((change, idx) => (
                <div key={idx} className="securityCard">
                  <div className={`cardKind kind-${change.kind.split('_')[1] || 'changed'}`}>
                    {change.kind.split('_')[1] || 'changed'}
                  </div>
                  <p className="cardDetail">{change.detail}</p>
                  {change.evidence_locations && change.evidence_locations.length > 0 && (
                    <div className="cardLocations">
                      {change.evidence_locations.map((loc, lidx) => (
                        <span key={lidx} className="locationTag">
                          {loc.type}
                          {loc.dom_index !== undefined && ` [${loc.dom_index}]`}
                          {loc.header_name && `: ${loc.header_name}`}
                          {loc.cookie_name && `: ${loc.cookie_name}`}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        )
      })}
      
      {Object.values(groupedChanges).every(g => g.length === 0) && (
        <p className="noSecurityChanges">No security-relevant changes detected</p>
      )}
    </div>
  )
}
