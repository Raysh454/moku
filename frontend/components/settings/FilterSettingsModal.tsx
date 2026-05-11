import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "../../src/api/client";
import type {
  Endpoint,
  EndpointStatsResponse,
  FilterConfig,
  FilterRule,
  RuleType,
} from "../../src/api/types";

type Tab = "rules" | "filtered" | "config";

type FilterSettingsModalProps = {
  open: boolean;
  projectSlug?: string;
  siteSlug?: string;
  onClose: () => void;
  onChanged?: () => Promise<void> | void;
};

function parseCsv(value: string): string[] {
  return value
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function parseStatusCodes(value: string): number[] {
  return parseCsv(value)
    .map((entry) => Number(entry))
    .filter((entry) => Number.isFinite(entry) && entry >= 100 && entry <= 599);
}

function parseFilterReason(meta: string): string {
  if (!meta) return "Unknown";
  try {
    const decoded = JSON.parse(meta) as { filter_reason?: string };
    return decoded.filter_reason || "Unknown";
  } catch {
    return "Unknown";
  }
}

export function FilterSettingsModal({
  open,
  projectSlug,
  siteSlug,
  onClose,
  onChanged,
}: FilterSettingsModalProps) {
  const [activeTab, setActiveTab] = useState<Tab>("rules");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const [rules, setRules] = useState<FilterRule[]>([]);
  const [stats, setStats] = useState<EndpointStatsResponse | null>(null);
  const [filteredEndpoints, setFilteredEndpoints] = useState<Endpoint[]>([]);
  const [config, setConfig] = useState<FilterConfig | null>(null);
  const [ruleEdits, setRuleEdits] = useState<Record<string, { rule_type: RuleType; rule_value: string }>>({});

  const [newRuleType, setNewRuleType] = useState<RuleType>("extension");
  const [newRuleValue, setNewRuleValue] = useState("");

  const [skipExtensionsInput, setSkipExtensionsInput] = useState("");
  const [skipPatternsInput, setSkipPatternsInput] = useState("");
  const [skipStatusCodesInput, setSkipStatusCodesInput] = useState("");

  const canLoad = open && Boolean(projectSlug) && Boolean(siteSlug);

  const syncEditState = useCallback((nextRules: FilterRule[]) => {
    setRuleEdits((previous) => {
      const updated: Record<string, { rule_type: RuleType; rule_value: string }> = {};
      for (const rule of nextRules) {
        updated[rule.id] = previous[rule.id] || {
          rule_type: rule.rule_type,
          rule_value: rule.rule_value,
        };
      }
      return updated;
    });
  }, []);

  const loadAll = useCallback(async () => {
    if (!projectSlug || !siteSlug) return;
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const [rulesData, configData, statsData, filteredData] = await Promise.all([
        api.listFilterRules(projectSlug, siteSlug),
        api.getFilterConfig(projectSlug, siteSlug),
        api.getEndpointStats(projectSlug, siteSlug),
        api.listEndpoints(projectSlug, siteSlug, "filtered", 200),
      ]);

      setRules(rulesData);
      syncEditState(rulesData);
      setConfig(configData);
      setStats(statsData);
      setFilteredEndpoints(filteredData);
      setSkipExtensionsInput((configData.skip_extensions || []).join(", "));
      setSkipPatternsInput((configData.skip_patterns || []).join(", "));
      setSkipStatusCodesInput((configData.skip_status_codes || []).join(", "));
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Failed to load filter settings");
    } finally {
      setLoading(false);
    }
  }, [projectSlug, siteSlug, syncEditState]);

  useEffect(() => {
    if (!canLoad) return;
    void loadAll();
  }, [canLoad, loadAll]);

  useEffect(() => {
    if (!open) {
      setActiveTab("rules");
      setError("");
      setMessage("");
    }
  }, [open]);

  const refreshFilteredEndpoints = useCallback(async () => {
    if (!projectSlug || !siteSlug) return;
    const filteredData = await api.listEndpoints(projectSlug, siteSlug, "filtered", 200);
    setFilteredEndpoints(filteredData);
  }, [projectSlug, siteSlug]);

  const createRule = useCallback(async () => {
    if (!projectSlug || !siteSlug) return;
    if (!newRuleValue.trim()) {
      setError("Rule value is required");
      return;
    }

    setLoading(true);
    setError("");
    setMessage("");
    try {
      const created = await api.createFilterRule(projectSlug, siteSlug, {
        rule_type: newRuleType,
        rule_value: newRuleValue.trim(),
      });
      const nextRules = [...rules, created];
      setRules(nextRules);
      syncEditState(nextRules);
      setNewRuleValue("");
      setMessage("Rule created");
      await loadAll();
    } catch (createError) {
      setError(createError instanceof Error ? createError.message : "Failed to create rule");
    } finally {
      setLoading(false);
    }
  }, [
    projectSlug,
    siteSlug,
    newRuleType,
    newRuleValue,
    rules,
    syncEditState,
    loadAll,
  ]);

  const toggleRule = useCallback(
    async (rule: FilterRule) => {
      if (!projectSlug || !siteSlug) return;
      setError("");
      setMessage("");
      try {
        const updated = await api.toggleFilterRule(projectSlug, siteSlug, rule.id);
        const nextRules = rules.map((item) => (item.id === updated.id ? updated : item));
        setRules(nextRules);
        syncEditState(nextRules);
        setMessage(`Rule ${updated.enabled ? "enabled" : "disabled"}`);
      } catch (toggleError) {
        setError(toggleError instanceof Error ? toggleError.message : "Failed to toggle rule");
      }
    },
    [projectSlug, siteSlug, rules, syncEditState],
  );

  const saveRule = useCallback(
    async (rule: FilterRule) => {
      if (!projectSlug || !siteSlug) return;
      const edits = ruleEdits[rule.id];
      if (!edits) return;

      setError("");
      setMessage("");
      try {
        const updated = await api.updateFilterRule(projectSlug, siteSlug, rule.id, edits);
        const nextRules = rules.map((item) => (item.id === updated.id ? updated : item));
        setRules(nextRules);
        syncEditState(nextRules);
        setMessage("Rule updated");
      } catch (saveError) {
        setError(saveError instanceof Error ? saveError.message : "Failed to update rule");
      }
    },
    [projectSlug, siteSlug, ruleEdits, rules, syncEditState],
  );

  const deleteRule = useCallback(
    async (rule: FilterRule) => {
      if (!projectSlug || !siteSlug) return;
      if (!window.confirm(`Delete rule "${rule.rule_value}"?`)) return;

      setError("");
      setMessage("");
      try {
        await api.deleteFilterRule(projectSlug, siteSlug, rule.id);
        const nextRules = rules.filter((item) => item.id !== rule.id);
        setRules(nextRules);
        syncEditState(nextRules);
        setMessage("Rule deleted");
      } catch (deleteError) {
        setError(deleteError instanceof Error ? deleteError.message : "Failed to delete rule");
      }
    },
    [projectSlug, siteSlug, rules, syncEditState],
  );

  const saveConfig = useCallback(async () => {
    if (!projectSlug || !siteSlug) return;
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const updated = await api.updateFilterConfig(projectSlug, siteSlug, {
        skip_extensions: parseCsv(skipExtensionsInput),
        skip_patterns: parseCsv(skipPatternsInput),
        skip_status_codes: parseStatusCodes(skipStatusCodesInput),
      });
      setConfig(updated);
      setMessage("Config updated");
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Failed to update config");
    } finally {
      setLoading(false);
    }
  }, [projectSlug, siteSlug, skipExtensionsInput, skipPatternsInput, skipStatusCodesInput]);

  const applyFilters = useCallback(async () => {
    if (!projectSlug || !siteSlug) return;
    if (!window.confirm("Apply filter rules to non-filtered endpoints?")) return;
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const result = await api.applyFilters(projectSlug, siteSlug);
      setMessage(result.message || `${result.filtered} endpoints filtered`);
      await loadAll();
      await onChanged?.();
    } catch (applyError) {
      setError(applyError instanceof Error ? applyError.message : "Failed to apply filters");
    } finally {
      setLoading(false);
    }
  }, [projectSlug, siteSlug, loadAll, onChanged]);

  const unfilter = useCallback(
    async (canonicalUrls: string[], all = false) => {
      if (!projectSlug || !siteSlug) return;
      setLoading(true);
      setError("");
      setMessage("");
      try {
        const result = await api.unfilterEndpoints(projectSlug, siteSlug, canonicalUrls, all);
        setMessage(`${result.unfiltered} endpoints unfiltered`);
        await Promise.all([refreshFilteredEndpoints(), loadAll()]);
        await onChanged?.();
      } catch (unfilterError) {
        setError(unfilterError instanceof Error ? unfilterError.message : "Failed to unfilter endpoints");
      } finally {
        setLoading(false);
      }
    },
    [projectSlug, siteSlug, refreshFilteredEndpoints, loadAll, onChanged],
  );

  const statEntries = useMemo(() => Object.entries(stats?.by_status || {}), [stats]);
  const hasSelection = Boolean(projectSlug && siteSlug);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[120] bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="w-full max-w-6xl max-h-[90vh] overflow-hidden bg-card border border-border rounded-2xl shadow-2xl">
        <div className="px-6 py-4 border-b border-border flex items-center justify-between">
          <div>
            <h2 className="text-sm font-black uppercase tracking-[0.25em] text-helper">Endpoint Filtering</h2>
            <p className="text-xs text-slate-400 mt-1">
              {hasSelection
                ? `${projectSlug} / ${siteSlug}`
                : "Select a website in the explorer to manage filtering"}
            </p>
          </div>
          <button
            className="h-9 px-3 text-xs font-bold uppercase tracking-wider text-slate-300 hover:text-white rounded-lg border border-border hover:border-slate-500"
            onClick={onClose}
          >
            Close
          </button>
        </div>

        <div className="px-6 py-4 border-b border-border flex items-center gap-2">
          {(["rules", "filtered", "config"] as Tab[]).map((tab) => (
            <button
              key={tab}
              className={`px-4 py-2 rounded-lg text-[11px] font-bold uppercase tracking-widest transition ${
                activeTab === tab
                  ? "bg-accent text-white shadow-lg shadow-accent/20"
                  : "bg-bg border border-border text-helper hover:text-slate-200"
              }`}
              onClick={() => setActiveTab(tab)}
            >
              {tab}
            </button>
          ))}
          <div className="ml-auto flex items-center gap-2">
            <button
              className="px-3 py-2 rounded-lg text-[11px] font-bold uppercase tracking-widest bg-bg border border-border text-slate-300 hover:text-white"
              onClick={() => void loadAll()}
              disabled={loading || !hasSelection}
            >
              Refresh
            </button>
            <button
              className="px-3 py-2 rounded-lg text-[11px] font-bold uppercase tracking-widest bg-warning text-black hover:brightness-110 disabled:opacity-50"
              onClick={() => void applyFilters()}
              disabled={loading || !hasSelection}
            >
              Apply Filters
            </button>
          </div>
        </div>

        <div className="px-6 py-3 border-b border-border bg-bg/30 flex flex-wrap items-center gap-4 text-xs">
          <span className="text-helper uppercase tracking-wider font-bold">Stats</span>
          {stats ? (
            <>
              <span className="text-slate-300">Total: {stats.total}</span>
              {statEntries.map(([status, count]) => (
                <span key={status} className="text-slate-400">
                  {status}: {count}
                </span>
              ))}
            </>
          ) : (
            <span className="text-slate-500">No stats loaded</span>
          )}
        </div>

        {(error || message) && (
          <div className="px-6 py-3 border-b border-border">
            {error && <p className="text-sm text-danger">{error}</p>}
            {!error && message && <p className="text-sm text-success">{message}</p>}
          </div>
        )}

        <div className="p-6 overflow-auto max-h-[56vh]">
          {!hasSelection && (
            <div className="text-sm text-helper bg-bg/40 border border-border rounded-xl p-6">
              No website selected. Pick one from explorer, then open settings again.
            </div>
          )}

          {hasSelection && activeTab === "rules" && (
            <div className="space-y-4">
              <div className="grid grid-cols-12 gap-3 bg-bg/40 border border-border rounded-xl p-4">
                <div className="col-span-3">
                  <label className="text-[10px] text-helper uppercase tracking-wider font-bold mb-1 block">
                    Rule Type
                  </label>
                  <select
                    value={newRuleType}
                    onChange={(event) => setNewRuleType(event.target.value as RuleType)}
                    className="w-full bg-card border border-border rounded-lg px-3 py-2 text-sm"
                  >
                    <option value="extension">extension</option>
                    <option value="pattern">pattern</option>
                    <option value="status_code">status_code</option>
                  </select>
                </div>
                <div className="col-span-6">
                  <label className="text-[10px] text-helper uppercase tracking-wider font-bold mb-1 block">
                    Rule Value
                  </label>
                  <input
                    value={newRuleValue}
                    onChange={(event) => setNewRuleValue(event.target.value)}
                    placeholder={newRuleType === "extension" ? ".png" : newRuleType === "pattern" ? "*/assets/*" : "404"}
                    className="w-full bg-card border border-border rounded-lg px-3 py-2 text-sm"
                  />
                </div>
                <div className="col-span-3 flex items-end">
                  <button
                    className="w-full h-10 rounded-lg bg-success text-black text-xs font-black uppercase tracking-wider hover:brightness-110 disabled:opacity-50"
                    onClick={() => void createRule()}
                    disabled={loading}
                  >
                    Add
                  </button>
                </div>
              </div>

              <div className="space-y-2">
                {rules.length === 0 && (
                  <div className="text-sm text-helper bg-bg/40 border border-border rounded-xl p-6">No rules configured.</div>
                )}
                {rules.map((rule) => {
                  const edit = ruleEdits[rule.id] || {
                    rule_type: rule.rule_type,
                    rule_value: rule.rule_value,
                  };
                  return (
                    <div
                      key={rule.id}
                      className={`grid grid-cols-12 gap-2 items-center border rounded-xl p-3 ${rule.enabled ? "border-border bg-bg/40" : "border-border/60 bg-bg/20 opacity-70"}`}
                    >
                      <div className="col-span-2">
                        <select
                          value={edit.rule_type}
                          onChange={(event) =>
                            setRuleEdits((previous) => ({
                              ...previous,
                              [rule.id]: {
                                ...edit,
                                rule_type: event.target.value as RuleType,
                              },
                            }))
                          }
                          className="w-full bg-card border border-border rounded-lg px-2 py-2 text-xs"
                        >
                          <option value="extension">extension</option>
                          <option value="pattern">pattern</option>
                          <option value="status_code">status_code</option>
                        </select>
                      </div>
                      <div className="col-span-6">
                        <input
                          value={edit.rule_value}
                          onChange={(event) =>
                            setRuleEdits((previous) => ({
                              ...previous,
                              [rule.id]: {
                                ...edit,
                                rule_value: event.target.value,
                              },
                            }))
                          }
                          className="w-full bg-card border border-border rounded-lg px-3 py-2 text-sm"
                        />
                      </div>
                      <div className="col-span-1 text-center text-[10px] uppercase tracking-wider text-helper">
                        {rule.enabled ? "on" : "off"}
                      </div>
                      <div className="col-span-3 flex items-center justify-end gap-2">
                        <button
                          className="px-2.5 py-2 rounded-lg border border-border text-xs font-semibold text-slate-300 hover:text-white"
                          onClick={() => void saveRule(rule)}
                        >
                          Save
                        </button>
                        <button
                          className="px-2.5 py-2 rounded-lg border border-warning/30 text-xs font-semibold text-warning hover:bg-warning/10"
                          onClick={() => void toggleRule(rule)}
                        >
                          Toggle
                        </button>
                        <button
                          className="px-2.5 py-2 rounded-lg border border-danger/30 text-xs font-semibold text-danger hover:bg-danger/10"
                          onClick={() => void deleteRule(rule)}
                        >
                          Delete
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {hasSelection && activeTab === "filtered" && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <h3 className="text-sm font-bold text-slate-200">Filtered Endpoints ({filteredEndpoints.length})</h3>
                <button
                  className="px-3 py-2 rounded-lg text-[11px] font-bold uppercase tracking-widest bg-accent text-white disabled:opacity-50"
                  onClick={() => void unfilter([], true)}
                  disabled={loading || filteredEndpoints.length === 0}
                >
                  Unfilter All
                </button>
              </div>

              <div className="space-y-2">
                {filteredEndpoints.length === 0 && (
                  <div className="text-sm text-helper bg-bg/40 border border-border rounded-xl p-6">
                    No filtered endpoints found.
                  </div>
                )}
                {filteredEndpoints.map((endpoint) => (
                  <div key={endpoint.id} className="grid grid-cols-12 gap-2 items-center border border-border rounded-xl p-3 bg-bg/40">
                    <div className="col-span-8">
                      <p className="text-sm text-slate-200 font-mono truncate" title={endpoint.url}>
                        {endpoint.url}
                      </p>
                    </div>
                    <div className="col-span-2 text-xs text-warning truncate" title={parseFilterReason(endpoint.meta)}>
                      {parseFilterReason(endpoint.meta)}
                    </div>
                    <div className="col-span-2 flex justify-end">
                      <button
                        className="px-2.5 py-2 rounded-lg border border-accent/30 text-xs font-semibold text-accent hover:bg-accent/10"
                        onClick={() => void unfilter([endpoint.canonical_url])}
                        disabled={loading}
                      >
                        Unfilter
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {hasSelection && activeTab === "config" && (
            <div className="space-y-4">
              <div className="grid grid-cols-1 gap-4">
                <div>
                  <label className="text-[10px] text-helper uppercase tracking-wider font-bold mb-1 block">
                    Skip extensions (comma separated)
                  </label>
                  <input
                    value={skipExtensionsInput}
                    onChange={(event) => setSkipExtensionsInput(event.target.value)}
                    placeholder=".jpg, .png, .css, .js"
                    className="w-full bg-card border border-border rounded-lg px-3 py-2 text-sm"
                  />
                </div>
                <div>
                  <label className="text-[10px] text-helper uppercase tracking-wider font-bold mb-1 block">
                    Skip patterns (comma separated)
                  </label>
                  <input
                    value={skipPatternsInput}
                    onChange={(event) => setSkipPatternsInput(event.target.value)}
                    placeholder="/assets/*, */vendor/*"
                    className="w-full bg-card border border-border rounded-lg px-3 py-2 text-sm"
                  />
                </div>
                <div>
                  <label className="text-[10px] text-helper uppercase tracking-wider font-bold mb-1 block">
                    Skip status codes (comma separated)
                  </label>
                  <input
                    value={skipStatusCodesInput}
                    onChange={(event) => setSkipStatusCodesInput(event.target.value)}
                    placeholder="301, 302, 404"
                    className="w-full bg-card border border-border rounded-lg px-3 py-2 text-sm"
                  />
                </div>
              </div>

              <div className="flex items-center justify-between">
                <button
                  className="px-3 py-2 rounded-lg text-[11px] font-bold uppercase tracking-widest bg-success text-black hover:brightness-110 disabled:opacity-50"
                  onClick={() => void saveConfig()}
                  disabled={loading}
                >
                  Save Config
                </button>
                <p className="text-xs text-slate-500">
                  Current config loaded: {config ? "yes" : "no"}
                </p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
