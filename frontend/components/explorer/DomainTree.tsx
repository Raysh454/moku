import React, { useEffect, useRef, useState } from "react";
import { useProject } from "../../context/ProjectContext";
import { useNotifications } from "../../context/NotificationContext";
import type { Domain, Endpoint, EnumerateRequest, FetchRequest, JobTransport } from "../../types/project";
import type { EnumerationConfig, SecurityDiffOverviewEntry } from "../../src/api/types";

interface DomainTreeProps {
  isCollapsed?: boolean;
  domainOverviews?: Map<string, SecurityDiffOverviewEntry[]>;
}

type DomainMenuSection = "enumerate" | "fetch" | "endpoints" | null;

type DomainMenuState = {
  enumMode: JobTransport;
  spiderEnabled: boolean;
  spiderDepth: number;
  spiderConcurrency: number;
  sitemapEnabled: boolean;
  robotsEnabled: boolean;
  waybackEnabled: boolean;
  waybackMachine: boolean;
  commonCrawl: boolean;
  fetchMode: JobTransport;
  fetchStatus: string;
  fetchLimit: number;
  fetchConcurrency: number;
  endpointStatus: string;
  endpointLimit: number;
};

const defaultMenuState = (): DomainMenuState => ({
  enumMode: "rest",
  spiderEnabled: true,
  spiderDepth: 4,
  spiderConcurrency: 5,
  sitemapEnabled: false,
  robotsEnabled: false,
  waybackEnabled: false,
  waybackMachine: true,
  commonCrawl: true,
  fetchMode: "rest",
  fetchStatus: "*",
  fetchLimit: 0,
  fetchConcurrency: 4,
  endpointStatus: "",
  endpointLimit: 0,
});

const buildEnumerationConfig = (state: DomainMenuState): EnumerationConfig => {
  const config: EnumerationConfig = {};
  if (state.spiderEnabled) {
    config.spider = {
      max_depth: state.spiderDepth,
      concurrency: state.spiderConcurrency,
    };
  }
  if (state.sitemapEnabled) config.sitemap = {};
  if (state.robotsEnabled) config.robots = {};
  if (state.waybackEnabled) {
    config.wayback = {
      use_wayback_machine: state.waybackMachine,
      use_common_crawl: state.commonCrawl,
    };
  }
  return config;
};

const DomainTree: React.FC<DomainTreeProps> = ({ isCollapsed = false, domainOverviews }) => {
  const {
    activeProject,
    selectedEndpoint,
    selectEndpoint,
    loadDomainEndpoints,
    addEndpointsForDomain,
    runEnumerateForDomain,
    runFetchForDomain,
    isBusy,
  } = useProject();
  const { notify } = useNotifications();

  const [expandedDomains, setExpandedDomains] = useState<Record<string, boolean>>({});
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [activeSection, setActiveSection] = useState<DomainMenuSection>(null);
  const [menuPos, setMenuPos] = useState({ top: 0, left: 0 });
  const [domainMenuState, setDomainMenuState] = useState<Record<string, DomainMenuState>>({});
  const [showAddEndpointsModal, setShowAddEndpointsModal] = useState(false);
  const [addEndpointsDomainId, setAddEndpointsDomainId] = useState<string | null>(null);
  const [newEndpointsInput, setNewEndpointsInput] = useState("");
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      const target = event.target as HTMLElement;
      const isClickOnMenuButton = target.closest("[data-menu-trigger]");

      if (menuRef.current && !menuRef.current.contains(target) && !isClickOnMenuButton) {
        setOpenMenuId(null);
        setActiveSection(null);
      }
    };

    const handleScroll = () => {
      if (openMenuId) {
        setOpenMenuId(null);
        setActiveSection(null);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    window.addEventListener("scroll", handleScroll, true);

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      window.removeEventListener("scroll", handleScroll, true);
    };
  }, [openMenuId]);

  if (!activeProject) {
    return (
      <div className="p-8 text-gray-600 italic text-center text-xs uppercase font-bold tracking-widest opacity-50">
        Empty Workspace
      </div>
    );
  }

  const toggleDomain = (id: string) => {
    if (isCollapsed) return;
    setExpandedDomains((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  const toggleMenu = (e: React.MouseEvent, domainId: string) => {
    e.stopPropagation();
    if (openMenuId === domainId) {
      setOpenMenuId(null);
      setActiveSection(null);
      return;
    }

    const rect = e.currentTarget.getBoundingClientRect();
    setMenuPos({
      top: rect.bottom + 4,
      left: rect.left - 8,
    });
    setOpenMenuId(domainId);
    setActiveSection("enumerate");
    setDomainMenuState((prev) => ({
      ...prev,
      [domainId]: prev[domainId] || defaultMenuState(),
    }));
  };

  const updateDomainMenuState = (domainId: string, updater: (state: DomainMenuState) => DomainMenuState) => {
    setDomainMenuState((prev) => {
      const current = prev[domainId] || defaultMenuState();
      return {
        ...prev,
        [domainId]: updater(current),
      };
    });
  };

  const runEnumerate = async (domain: Domain) => {
    const state = domainMenuState[domain.id] || defaultMenuState();
    const request: EnumerateRequest = {
      mode: state.enumMode,
      config: buildEnumerationConfig(state),
    };

    notify({
      kind: "info",
      title: `Starting enumeration for ${domain.hostname}`,
      message: `${state.enumMode.toUpperCase()} job queued`,
    });

    await runEnumerateForDomain(domain.id, request);
  };

  const runFetch = async (domain: Domain) => {
    const state = domainMenuState[domain.id] || defaultMenuState();
    const request: FetchRequest = {
      mode: state.fetchMode,
      status: state.fetchStatus || "*",
      limit: Math.max(0, Number.isFinite(state.fetchLimit) ? state.fetchLimit : 0),
      config: {
        concurrency: state.fetchConcurrency || 4,
      },
    };

    notify({
      kind: "info",
      title: `Starting fetch for ${domain.hostname}`,
      message: `${state.fetchMode.toUpperCase()} • status=${request.status === "*" ? "all-non-filtered" : request.status} • limit=${request.limit === 0 ? "no-limit" : request.limit}`,
    });

    await runFetchForDomain(domain.id, request);
  };

  const loadEndpoints = async (domain: Domain) => {
    const state = domainMenuState[domain.id] || defaultMenuState();
    await loadDomainEndpoints(
      domain.id,
      state.endpointStatus,
      Math.max(0, Number.isFinite(state.endpointLimit) ? state.endpointLimit : 0),
    );
    notify({
      kind: "success",
      title: `Endpoints loaded for ${domain.hostname}`,
      message: `status=${state.endpointStatus || "non-filtered"} • limit=${state.endpointLimit === 0 ? "no-limit" : state.endpointLimit}`,
    });
  };

  const openAddEndpoints = (domainId: string) => {
    setAddEndpointsDomainId(domainId);
    setNewEndpointsInput("");
    setShowAddEndpointsModal(true);
  };

  const submitAddEndpoints = async () => {
    if (!activeProject) return;
    if (!addEndpointsDomainId) return;
    const domain = activeProject.domains.find((item) => item.id === addEndpointsDomainId);
    if (!domain) return;

    const urls = newEndpointsInput
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    if (urls.length === 0) {
      notify({
        kind: "warning",
        title: "No endpoints provided",
        message: "Add at least one endpoint URL on a new line.",
      });
      return;
    }

    const added = await addEndpointsForDomain(domain.id, urls, "manual");
    if (added > 0) {
      const state = domainMenuState[domain.id] || defaultMenuState();
      await loadDomainEndpoints(
        domain.id,
        state.endpointStatus,
        Math.max(0, Number.isFinite(state.endpointLimit) ? state.endpointLimit : 0),
      );
      setShowAddEndpointsModal(false);
      setAddEndpointsDomainId(null);
      setNewEndpointsInput("");
      notify({
        kind: "success",
        title: `Added ${added} endpoint${added === 1 ? "" : "s"}`,
        message: `${domain.hostname} updated`,
      });
    }
  };

  const endpointLabel = (endpoint: Endpoint): string => {
    if (endpoint.path) return endpoint.path;
    try {
      return new URL(endpoint.url).pathname || "/";
    } catch {
      return endpoint.url;
    }
  };

  const normalizePath = (p: string) => {
    if (!p) return "";
    let normalized = p;
    if (normalized.endsWith("/") && normalized.length > 1) {
      normalized = normalized.slice(0, -1);
    }
    // Also handle root path consistency: "/" vs ""
    if (normalized === "/") return "";
    return normalized;
  };

  const getSortedEndpoints = (domain: Domain): Endpoint[] => {
    const overview = domainOverviews?.get(domain.slug);
    if (!overview || !Array.isArray(overview)) {
      return domain.endpoints;
    }

    const endpointsByPath = new Map<string, SecurityDiffOverviewEntry>();
    for (const entry of overview) {
      endpointsByPath.set(normalizePath(entry.url), entry);
    }

    return [...domain.endpoints].sort((a, b) => {
      const overviewA = endpointsByPath.get(normalizePath(a.path));
      const overviewB = endpointsByPath.get(normalizePath(b.path));

      // Primary sort: posture score (higher first)
      const scoreA = overviewA?.score_head ?? -1;
      const scoreB = overviewB?.score_head ?? -1;
      if (scoreA !== scoreB) return scoreB - scoreA;

      // Secondary sort: exposure delta (higher first = worse regression)
      const exposureA = overviewA?.exposure_delta ?? 0;
      const exposureB = overviewB?.exposure_delta ?? 0;
      if (exposureA !== exposureB) return exposureB - exposureA;

      // Fallback: original order
      return domain.endpoints.indexOf(a) - domain.endpoints.indexOf(b);
    });
  };

  const getOverviewForEndpoint = (domain: Domain, endpoint: Endpoint): SecurityDiffOverviewEntry | undefined => {
    const overview = domainOverviews?.get(domain.slug);
    if (!overview || !Array.isArray(overview)) return undefined;
    const normalizedTarget = normalizePath(endpoint.path);
    return overview.find((entry) => normalizePath(entry.url) === normalizedTarget);
  };

  const currentMenuState = openMenuId ? domainMenuState[openMenuId] || defaultMenuState() : defaultMenuState();

  return (
    <div
      className={`overflow-y-auto h-full pb-32 custom-scrollbar py-2 transition-all duration-300 ${isCollapsed ? "items-center flex flex-col" : ""}`}
    >
      {activeProject.domains.map((domain) => {
        const isExpanded = expandedDomains[domain.id] ?? true;
        const isMenuOpen = openMenuId === domain.id;

        return (
          <div key={domain.id} className="mb-1 select-none w-full relative">
            <div
              onClick={() => toggleDomain(domain.id)}
              className={`group flex items-center gap-2 py-2 text-[13px] font-bold text-gray-400 hover:text-white cursor-pointer transition-colors relative ${isCollapsed ? "justify-center px-0" : "px-6"}`}
              title={isCollapsed ? domain.hostname : undefined}
            >
              {!isCollapsed && (
                <span
                  className={`text-[10px] text-helper transition-transform duration-200 ${isExpanded ? "rotate-90" : "rotate-0"}`}
                >
                  ▶
                </span>
              )}
              {isCollapsed ? (
                <div className="w-6 h-6 rounded bg-accent/10 border border-accent/20 flex items-center justify-center text-[10px] text-accent uppercase font-black">
                  {domain.hostname.charAt(0)}
                </div>
              ) : (
                <>
                  <span className="tracking-tight truncate flex-1 animate-in fade-in duration-300">
                    {domain.hostname}
                  </span>

                  <button
                    title="Domain menu"
                    data-menu-trigger
                    onClick={(e) => toggleMenu(e, domain.id)}
                    className={`p-1 rounded hover:bg-white/10 text-helper hover:text-white transition-all ${isMenuOpen ? "bg-white/10 text-white" : ""}`}
                  >
                    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                      <path d="M6 10c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm12 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm-6 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
                    </svg>
                  </button>
                </>
              )}
            </div>

            {!isCollapsed && isExpanded && domain.endpoints.length > 0 && (
              <div className="mt-0 animate-in slide-in-from-left-2 duration-200">
                {(() => {
                  const sortedEndpoints = getSortedEndpoints(domain);
                  return sortedEndpoints.map((endpoint, idx) => {
                    const isSelected = selectedEndpoint?.id === endpoint.id;
                    const isLast = idx === sortedEndpoints.length - 1;
                    const overview = getOverviewForEndpoint(domain, endpoint);
                    return (
                      <div key={endpoint.id} className="relative">
                        <div
                          onClick={() => void selectEndpoint(domain.id, endpoint.id)}
                          className={`group flex items-center justify-between px-10 py-1.5 cursor-pointer transition-all ${isSelected ? "text-accent" : "text-slate-500 hover:text-slate-300"}`}
                        >
                          <div className="flex items-center gap-2 flex-1 min-w-0">
                            <span className="font-mono text-slate-800 select-none">{isLast ? "└" : "├"}</span>
                            <span className={`text-[12px] font-medium tracking-tight truncate ${isSelected ? "font-bold" : ""}`}>
                              {endpointLabel(endpoint)}
                            </span>
                          </div>
                          {overview && (
                            <div className="flex items-center gap-1.5 ml-2 flex-shrink-0">
                              {overview.score_head !== undefined && (
                                <div
                                  title={`Risk: ${overview.score_base.toFixed(2)} → ${overview.score_head.toFixed(2)} (Delta: ${overview.score_delta > 0 ? "+" : ""}${overview.score_delta.toFixed(2)})`}
                                  className={`text-[10px] font-bold px-1.5 py-0.5 rounded flex items-center gap-1 ${
                                    overview.score_delta > 0
                                      ? "bg-danger/20 text-danger"
                                      : overview.score_delta < 0
                                        ? "bg-success/20 text-success"
                                        : "bg-accent/20 text-accent"
                                  }`}
                                >
                                  {overview.score_base !== undefined && (
                                    <>
                                      <span>{overview.score_base.toFixed(2)}</span>
                                      <span className="opacity-50">→</span>
                                    </>
                                  )}
                                  <span>{overview.score_head.toFixed(2)}</span>
                                </div>
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  });
                })()}
              </div>
            )}
          </div>
        );
      })}

      {openMenuId && (
        <div
          ref={menuRef}
          style={{
            position: "fixed",
            top: `${menuPos.top}px`,
            left: `${menuPos.left}px`,
            zIndex: 9999,
          }}
          className="w-[620px] bg-card border border-border rounded-xl shadow-[0_15px_40px_rgba(0,0,0,0.6)] animate-in fade-in zoom-in-95 duration-100 overflow-hidden"
        >
          <div className="grid grid-cols-2">
            <div className="p-1.5 border-r border-border">
              <button
                onMouseEnter={() => setActiveSection("enumerate")}
                className={`w-full flex items-center justify-between px-3 py-2 text-[12px] font-semibold rounded-lg transition-all ${activeSection === "enumerate" ? "bg-white/10 text-white" : "text-slate-300 hover:text-white hover:bg-white/5"}`}
              >
                <span>Enumeration</span>
                <span className="text-[10px] opacity-70">❯</span>
              </button>
              <button
                onMouseEnter={() => setActiveSection("fetch")}
                className={`w-full flex items-center justify-between px-3 py-2 text-[12px] font-semibold rounded-lg transition-all ${activeSection === "fetch" ? "bg-white/10 text-white" : "text-slate-300 hover:text-white hover:bg-white/5"}`}
              >
                <span>Fetch</span>
                <span className="text-[10px] opacity-70">❯</span>
              </button>
              <button
                onMouseEnter={() => setActiveSection("endpoints")}
                className={`w-full flex items-center justify-between px-3 py-2 text-[12px] font-semibold rounded-lg transition-all ${activeSection === "endpoints" ? "bg-white/10 text-white" : "text-slate-300 hover:text-white hover:bg-white/5"}`}
              >
                <span>Endpoints</span>
                <span className="text-[10px] opacity-70">❯</span>
              </button>
            </div>

            <div className="p-3 space-y-3">
              {activeSection === "enumerate" && (
                <>
                  <div className="grid grid-cols-2 gap-2">
                    <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                      Transport
                      <select
                        value={currentMenuState.enumMode}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            enumMode: event.target.value as JobTransport,
                          }))
                        }
                        className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                      >
                        <option value="rest">REST</option>
                        <option value="ws">WebSocket</option>
                      </select>
                    </label>
                    <label className="text-[11px] text-slate-300 flex items-center gap-2 mt-5">
                      <input
                        type="checkbox"
                        checked={currentMenuState.spiderEnabled}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            spiderEnabled: event.target.checked,
                          }))
                        }
                      />
                      Spider
                    </label>
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                      Spider depth
                      <input
                        type="number"
                        min={1}
                        max={20}
                        value={currentMenuState.spiderDepth}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            spiderDepth: Number(event.target.value) || 4,
                          }))
                        }
                        className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                      />
                    </label>
                    <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                      Spider concurrency
                      <input
                        type="number"
                        min={1}
                        max={100}
                        value={currentMenuState.spiderConcurrency}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            spiderConcurrency: Number(event.target.value) || 5,
                          }))
                        }
                        className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                      />
                    </label>
                  </div>

                  <div className="grid grid-cols-2 gap-2 text-[11px] text-slate-300">
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={currentMenuState.sitemapEnabled}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            sitemapEnabled: event.target.checked,
                          }))
                        }
                      />
                      Sitemap
                    </label>
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={currentMenuState.robotsEnabled}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            robotsEnabled: event.target.checked,
                          }))
                        }
                      />
                      Robots
                    </label>
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={currentMenuState.waybackEnabled}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            waybackEnabled: event.target.checked,
                          }))
                        }
                      />
                      Wayback
                    </label>
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={currentMenuState.waybackMachine}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            waybackMachine: event.target.checked,
                          }))
                        }
                        disabled={!currentMenuState.waybackEnabled}
                      />
                      Archive.org
                    </label>
                    <label className="flex items-center gap-2 col-span-2">
                      <input
                        type="checkbox"
                        checked={currentMenuState.commonCrawl}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            commonCrawl: event.target.checked,
                          }))
                        }
                        disabled={!currentMenuState.waybackEnabled}
                      />
                      Common Crawl
                    </label>
                  </div>

                  <button
                    className="w-full bg-accent text-white rounded px-3 py-2 text-[12px] font-semibold hover:brightness-110 disabled:opacity-50"
                    disabled={isBusy}
                    onClick={() => {
                      const domain = activeProject.domains.find((item) => item.id === openMenuId);
                      if (!domain) return;
                      void runEnumerate(domain);
                    }}
                  >
                    Start Enumeration
                  </button>
                </>
              )}

              {activeSection === "fetch" && (
                <>
                  <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                    Transport
                    <select
                      value={currentMenuState.fetchMode}
                      onChange={(event) =>
                        updateDomainMenuState(openMenuId, (state) => ({
                          ...state,
                          fetchMode: event.target.value as JobTransport,
                        }))
                      }
                      className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                    >
                      <option value="rest">REST</option>
                      <option value="ws">WebSocket</option>
                    </select>
                  </label>

                  <div className="grid grid-cols-3 gap-2">
                    <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                      Status
                      <select
                        value={currentMenuState.fetchStatus}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            fetchStatus: event.target.value,
                          }))
                        }
                        className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                      >
                        <option value="*">all (excluding filtered)</option>
                        <option value="new">new</option>
                        <option value="pending">pending</option>
                        <option value="fetched">fetched</option>
                        <option value="failed">failed</option>
                        <option value="filtered">filtered</option>
                      </select>
                    </label>
                    <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                      Limit
                      <input
                        type="number"
                        min={0}
                        max={20000}
                        value={currentMenuState.fetchLimit}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            fetchLimit: Math.max(0, Number(event.target.value) || 0),
                          }))
                        }
                        className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                      />
                    </label>
                    <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                      Concurrency
                      <input
                        type="number"
                        min={1}
                        max={100}
                        value={currentMenuState.fetchConcurrency}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            fetchConcurrency: Number(event.target.value) || 4,
                          }))
                        }
                        className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                      />
                    </label>
                  </div>

                  <p className="text-[10px] text-slate-500">Limit 0 means no limit (fetch all matching endpoints).</p>

                  <button
                    className="w-full bg-success text-black rounded px-3 py-2 text-[12px] font-semibold hover:brightness-110 disabled:opacity-50"
                    disabled={isBusy}
                    onClick={() => {
                      const domain = activeProject.domains.find((item) => item.id === openMenuId);
                      if (!domain) return;
                      void runFetch(domain);
                    }}
                  >
                    Start Fetch
                  </button>
                </>
              )}

              {activeSection === "endpoints" && (
                <>
                  <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                    Endpoints to load
                    <select
                      value={currentMenuState.endpointStatus}
                      onChange={(event) =>
                        updateDomainMenuState(openMenuId, (state) => ({
                          ...state,
                          endpointStatus: event.target.value,
                        }))
                      }
                      className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                    >
                      <option value="">All (excluding filtered)</option>
                      <option value="*">All (including filtered)</option>
                      <option value="new">new</option>
                      <option value="pending">pending</option>
                      <option value="fetched">fetched</option>
                      <option value="failed">failed</option>
                      <option value="filtered">filtered</option>
                    </select>
                  </label>

                  <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                    Limit
                    <input
                      type="number"
                      min={0}
                      max={50000}
                      value={currentMenuState.endpointLimit}
                      onChange={(event) =>
                        updateDomainMenuState(openMenuId, (state) => ({
                          ...state,
                          endpointLimit: Math.max(0, Number(event.target.value) || 0),
                        }))
                      }
                      className="w-full bg-bg border border-border rounded px-2 py-1 text-[12px]"
                    />
                  </label>

                  <p className="text-[10px] text-slate-500">
                    Default is all non-filtered endpoints. Limit 0 means no limit.
                  </p>

                  <button
                    className="w-full bg-accent text-white rounded px-3 py-2 text-[12px] font-semibold hover:brightness-110 disabled:opacity-50"
                    disabled={isBusy}
                    onClick={() => {
                      const domain = activeProject.domains.find((item) => item.id === openMenuId);
                      if (!domain) return;
                      void loadEndpoints(domain);
                    }}
                  >
                    Load Endpoints
                  </button>

                  <button
                    className="w-full bg-bg border border-border text-slate-200 rounded px-3 py-2 text-[12px] font-semibold hover:border-slate-500 disabled:opacity-50"
                    disabled={isBusy}
                    onClick={() => {
                      openAddEndpoints(openMenuId);
                    }}
                  >
                    Add Endpoints
                  </button>
                </>
              )}
            </div>
          </div>
        </div>
      )}

      {showAddEndpointsModal && addEndpointsDomainId && (
        <div className="fixed inset-0 z-[10000] bg-black/60 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="w-full max-w-2xl bg-card border border-border rounded-2xl shadow-2xl overflow-hidden">
            <div className="px-5 py-4 border-b border-border flex items-center justify-between">
              <div>
                <h3 className="text-[11px] font-black uppercase tracking-[0.22em] text-helper">Add Endpoints</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Paste endpoint URLs, one per line.
                </p>
              </div>
              <button
                className="h-8 px-3 text-[11px] uppercase tracking-widest border border-border rounded-lg text-slate-300 hover:text-white hover:border-slate-500"
                onClick={() => {
                  setShowAddEndpointsModal(false);
                  setAddEndpointsDomainId(null);
                }}
              >
                Close
              </button>
            </div>
            <div className="p-5 space-y-4">
              <textarea
                value={newEndpointsInput}
                onChange={(event) => setNewEndpointsInput(event.target.value)}
                rows={10}
                placeholder={"https://example.com/\nhttps://example.com/login\nhttps://example.com/admin"}
                className="w-full bg-bg border border-border rounded-xl px-4 py-3 text-sm font-mono text-slate-200 resize-y min-h-[220px]"
              />
              <div className="flex justify-end">
                <button
                  className="h-10 px-5 rounded-lg bg-success text-black text-xs font-black uppercase tracking-[0.18em] hover:brightness-110 disabled:opacity-50"
                  disabled={isBusy}
                  onClick={() => {
                    void submitAddEndpoints();
                  }}
                >
                  Add Endpoints
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export { DomainTree };
export default DomainTree;
