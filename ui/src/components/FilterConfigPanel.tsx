import { useState, useEffect } from 'react'
import { api } from '../api/client'
import type { FilterRule, FilterConfig, FilteredEndpoint, EndpointStats, RuleType } from '../api/types'

type FilterConfigPanelProps = {
  project: string
  site: string
  onActivity?: (title: string, detail?: string) => void
}

export default function FilterConfigPanel({ project, site, onActivity }: FilterConfigPanelProps) {
  const [rules, setRules] = useState<FilterRule[]>([])
  const [config, setConfig] = useState<FilterConfig | null>(null)
  const [filteredEndpoints, setFilteredEndpoints] = useState<FilteredEndpoint[]>([])
  const [stats, setStats] = useState<EndpointStats | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // New rule form state
  const [newRuleType, setNewRuleType] = useState<RuleType>('extension')
  const [newRuleValue, setNewRuleValue] = useState('')

  // Tab state
  const [activeTab, setActiveTab] = useState<'rules' | 'filtered' | 'preview'>('rules')

  const log = (title: string, detail?: string) => {
    onActivity?.(title, detail)
  }

  const loadData = async () => {
    if (!project || !site) return
    setLoading(true)
    setError('')
    try {
      const [rulesData, configData, statsData] = await Promise.all([
        api.listFilterRules(project, site),
        api.getFilterConfig(project, site),
        api.getEndpointStats(project, site),
      ])
      setRules(rulesData)
      setConfig(configData)
      setStats(statsData)
      log('Loaded filter configuration')
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
      log('Failed to load filter config', msg)
    } finally {
      setLoading(false)
    }
  }

  const loadFilteredEndpoints = async () => {
    if (!project || !site) return
    try {
      const data = await api.listEndpoints(project, site, 'filtered', 200)
      // Convert Endpoint to FilteredEndpoint format for UI
      const filteredData = data.map(ep => {
        let filterReason = 'Unknown'
        try {
          const meta = JSON.parse(ep.meta || '{}')
          filterReason = meta.filter_reason || 'Unknown'
        } catch {
          // ignore parse errors
        }
        return {
          url: ep.url,
          canonical_url: ep.canonical_url,
          status: ep.status,
          filter_reason: filterReason,
          filtered_at: ep.last_discovered_at
        }
      })
      setFilteredEndpoints(filteredData)
      log('Loaded filtered endpoints', `${filteredData.length} endpoints`)
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
    }
  }

  useEffect(() => {
    loadData()
  }, [project, site])

  useEffect(() => {
    if (activeTab === 'filtered') {
      loadFilteredEndpoints()
    }
  }, [activeTab, project, site])

  const handleAddRule = async () => {
    if (!newRuleValue.trim()) {
      setError('Rule value is required')
      return
    }
    setLoading(true)
    setError('')
    try {
      const rule = await api.createFilterRule(project, site, {
        rule_type: newRuleType,
        rule_value: newRuleValue.trim(),
      })
      setRules((prev) => [...prev, rule])
      setNewRuleValue('')
      log('Created filter rule', `${newRuleType}: ${newRuleValue}`)
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
      log('Failed to create rule', msg)
    } finally {
      setLoading(false)
    }
  }

  const handleToggleRule = async (rule: FilterRule) => {
    try {
      const updated = await api.toggleFilterRule(project, site, rule.id)
      setRules((prev) =>
        prev.map((r) => (r.id === rule.id ? updated : r))
      )
      log(`${rule.enabled ? 'Disabled' : 'Enabled'} rule`, rule.rule_value)
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
    }
  }

  const handleDeleteRule = async (rule: FilterRule) => {
    if (!confirm(`Delete rule "${rule.rule_value}"?`)) return
    try {
      await api.deleteFilterRule(project, site, rule.id)
      setRules((prev) => prev.filter((r) => r.id !== rule.id))
      log('Deleted filter rule', rule.rule_value)
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
    }
  }

  const handleUnfilter = async (canonicalUrls: string[]) => {
    try {
      const result = await api.unfilterEndpoints(project, site, canonicalUrls)
      log('Unfiltered endpoints', `${result.count} endpoints`)
      await loadFilteredEndpoints()
      await loadData()
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
    }
  }

  const handleApplyFilters = async () => {
    if (!confirm('Apply current filter rules to all non-filtered endpoints?')) return
    setLoading(true)
    setError('')
    try {
      const result = await api.applyFilters(project, site)
      log('Applied filters', `${result.filtered} endpoints filtered`)
      await loadData()
      await loadFilteredEndpoints()
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
      log('Failed to apply filters', msg)
    } finally {
      setLoading(false)
    }
  }

  if (!project || !site) {
    return <div style={styles.container}>Select a project and website to manage filters.</div>
  }

  return (
    <div style={styles.container}>
      <div style={styles.header}>
        <h3 style={styles.title}>Filter Configuration</h3>
        <button onClick={loadData} disabled={loading} style={styles.refreshBtn}>
          {loading ? 'Loading...' : '↻ Refresh'}
        </button>
      </div>

      {error && <div style={styles.error}>{error}</div>}

      {/* Stats */}
      {stats && (
        <div style={styles.stats}>
          <span style={styles.stat}>Pending: {stats.pending}</span>
          <span style={styles.stat}>Fetched: {stats.fetched}</span>
          <span style={styles.stat}>Failed: {stats.failed}</span>
          <span style={{ ...styles.stat, color: '#f59e0b' }}>Filtered: {stats.filtered}</span>
          <span style={styles.stat}>Total: {stats.total}</span>
        </div>
      )}

      {/* Tabs */}
      <div style={styles.tabs}>
        <button
          onClick={() => setActiveTab('rules')}
          style={{ ...styles.tab, ...(activeTab === 'rules' ? styles.tabActive : {}) }}
        >
          Rules ({rules.length})
        </button>
        <button
          onClick={() => setActiveTab('filtered')}
          style={{ ...styles.tab, ...(activeTab === 'filtered' ? styles.tabActive : {}) }}
        >
          Filtered Endpoints
        </button>
        <button
          onClick={() => setActiveTab('preview')}
          style={{ ...styles.tab, ...(activeTab === 'preview' ? styles.tabActive : {}) }}
        >
          Config Preview
        </button>
      </div>

      {/* Rules Tab */}
      {activeTab === 'rules' && (
        <div style={styles.tabContent}>
          {/* Add Rule Form */}
          <div style={styles.addForm}>
            <select
              value={newRuleType}
              onChange={(e) => setNewRuleType(e.target.value as RuleType)}
              style={styles.select}
            >
              <option value="extension">Extension</option>
              <option value="pattern">Pattern</option>
              <option value="status_code">Status Code</option>
            </select>
            <input
              type="text"
              value={newRuleValue}
              onChange={(e) => setNewRuleValue(e.target.value)}
              placeholder={newRuleType === 'extension' ? '.jpg' : newRuleType === 'pattern' ? '*/assets/*' : '404'}
              style={styles.input}
            />
            <button onClick={handleAddRule} disabled={loading} style={styles.addBtn}>
              Add Rule
            </button>
            <button onClick={handleApplyFilters} disabled={loading || rules.length === 0} style={styles.applyBtn}>
              ⚡ Apply Filters
            </button>
          </div>

          {/* Rules List */}
          <div style={styles.rulesList}>
            {rules.length === 0 ? (
              <div style={styles.empty}>No filter rules configured. Add rules above or wait for default rules to load.</div>
            ) : (
              rules.map((rule) => (
                <div key={rule.id} style={{ ...styles.ruleItem, opacity: rule.enabled ? 1 : 0.5 }}>
                  <span style={styles.ruleType}>{rule.rule_type}</span>
                  <span style={styles.ruleValue}>{rule.rule_value}</span>
                  <button onClick={() => handleToggleRule(rule)} style={styles.toggleBtn}>
                    {rule.enabled ? 'Disable' : 'Enable'}
                  </button>
                  <button onClick={() => handleDeleteRule(rule)} style={styles.deleteBtn}>
                    ×
                  </button>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* Filtered Endpoints Tab */}
      {activeTab === 'filtered' && (
        <div style={styles.tabContent}>
          {filteredEndpoints.length === 0 ? (
            <div style={styles.empty}>No filtered endpoints yet.</div>
          ) : (
            <>
              <button
                onClick={() => handleUnfilter(filteredEndpoints.map((e) => e.canonical_url))}
                style={styles.unfilterAllBtn}
              >
                Unfilter All ({filteredEndpoints.length})
              </button>
              <div style={styles.filteredList}>
                {filteredEndpoints.map((ep) => (
                  <div key={ep.canonical_url} style={styles.filteredItem}>
                    <span style={styles.filteredUrl} title={ep.url}>
                      {ep.url.length > 60 ? ep.url.slice(0, 60) + '...' : ep.url}
                    </span>
                    <span style={styles.filteredReason}>{ep.filter_reason}</span>
                    <button
                      onClick={() => handleUnfilter([ep.canonical_url])}
                      style={styles.unfilterBtn}
                    >
                      Unfilter
                    </button>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      {/* Config Preview Tab */}
      {activeTab === 'preview' && (
        <div style={styles.tabContent}>
          <pre style={styles.configPreview}>
            {config ? JSON.stringify(config, null, 2) : 'No configuration loaded'}
          </pre>
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    padding: '12px',
    backgroundColor: '#1e1e1e',
    borderRadius: '8px',
    color: '#e0e0e0',
    fontSize: '13px',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: '12px',
  },
  title: {
    margin: 0,
    fontSize: '16px',
    fontWeight: 600,
  },
  refreshBtn: {
    padding: '4px 12px',
    backgroundColor: '#3b82f6',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
  },
  error: {
    padding: '8px 12px',
    backgroundColor: '#ef4444',
    color: 'white',
    borderRadius: '4px',
    marginBottom: '12px',
  },
  stats: {
    display: 'flex',
    gap: '16px',
    padding: '8px 12px',
    backgroundColor: '#2d2d2d',
    borderRadius: '4px',
    marginBottom: '12px',
  },
  stat: {
    fontSize: '12px',
  },
  tabs: {
    display: 'flex',
    gap: '4px',
    marginBottom: '12px',
    borderBottom: '1px solid #444',
    paddingBottom: '8px',
  },
  tab: {
    padding: '6px 12px',
    backgroundColor: 'transparent',
    color: '#888',
    border: 'none',
    borderRadius: '4px 4px 0 0',
    cursor: 'pointer',
    fontSize: '12px',
  },
  tabActive: {
    backgroundColor: '#3b82f6',
    color: 'white',
  },
  tabContent: {
    minHeight: '200px',
  },
  addForm: {
    display: 'flex',
    gap: '8px',
    marginBottom: '12px',
  },
  select: {
    padding: '6px 8px',
    backgroundColor: '#2d2d2d',
    color: '#e0e0e0',
    border: '1px solid #444',
    borderRadius: '4px',
    fontSize: '12px',
  },
  input: {
    padding: '6px 8px',
    backgroundColor: '#2d2d2d',
    color: '#e0e0e0',
    border: '1px solid #444',
    borderRadius: '4px',
    fontSize: '12px',
    flex: 1,
  },
  addBtn: {
    padding: '6px 12px',
    backgroundColor: '#22c55e',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
  },
  applyBtn: {
    padding: '6px 12px',
    backgroundColor: '#f59e0b',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
    fontWeight: 600,
  },
  rulesList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
  },
  ruleItem: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '6px 8px',
    backgroundColor: '#2d2d2d',
    borderRadius: '4px',
  },
  ruleType: {
    padding: '2px 6px',
    backgroundColor: '#6366f1',
    color: 'white',
    borderRadius: '3px',
    fontSize: '10px',
    fontWeight: 600,
    textTransform: 'uppercase',
  },
  ruleValue: {
    flex: 1,
    fontFamily: 'monospace',
    fontSize: '12px',
  },
  toggleBtn: {
    padding: '2px 8px',
    backgroundColor: '#f59e0b',
    color: 'white',
    border: 'none',
    borderRadius: '3px',
    cursor: 'pointer',
    fontSize: '10px',
  },
  deleteBtn: {
    padding: '2px 8px',
    backgroundColor: '#ef4444',
    color: 'white',
    border: 'none',
    borderRadius: '3px',
    cursor: 'pointer',
    fontSize: '12px',
    fontWeight: 'bold',
  },
  empty: {
    padding: '20px',
    textAlign: 'center',
    color: '#888',
  },
  filteredList: {
    display: 'flex',
    flexDirection: 'column',
    gap: '4px',
    maxHeight: '300px',
    overflowY: 'auto',
  },
  filteredItem: {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    padding: '6px 8px',
    backgroundColor: '#2d2d2d',
    borderRadius: '4px',
  },
  filteredUrl: {
    flex: 1,
    fontFamily: 'monospace',
    fontSize: '11px',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  filteredReason: {
    padding: '2px 6px',
    backgroundColor: '#f59e0b',
    color: 'white',
    borderRadius: '3px',
    fontSize: '10px',
  },
  unfilterBtn: {
    padding: '2px 8px',
    backgroundColor: '#3b82f6',
    color: 'white',
    border: 'none',
    borderRadius: '3px',
    cursor: 'pointer',
    fontSize: '10px',
  },
  unfilterAllBtn: {
    padding: '6px 12px',
    backgroundColor: '#3b82f6',
    color: 'white',
    border: 'none',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '12px',
    marginBottom: '12px',
  },
  configPreview: {
    padding: '12px',
    backgroundColor: '#2d2d2d',
    borderRadius: '4px',
    fontSize: '11px',
    fontFamily: 'monospace',
    overflow: 'auto',
    maxHeight: '300px',
    margin: 0,
  },
}
