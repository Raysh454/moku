import React, { useEffect, useRef, useState } from "react";
import { useProject } from "../../context/ProjectContext";
import type { Domain, Endpoint, EnumerateRequest, FetchRequest, JobTransport } from "../../types/project";
import type { EnumerationConfig } from "../../src/api/types";

interface DomainTreeProps {
  isCollapsed?: boolean;
}

type DomainMenuSection = "enumerate" | "fetch" | null;

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
  fetchStatus: "new",
  fetchLimit: 100,
  fetchConcurrency: 4,
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

export const DomainTree: React.FC<DomainTreeProps> = ({ isCollapsed = false }) => {
  const {
    activeProject,
    selectedEndpoint,
    selectEndpoint,
    runEnumerateForDomain,
    runFetchForDomain,
    isBusy,
  } = useProject();

  const [expandedDomains, setExpandedDomains] = useState<Record<string, boolean>>({});
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [activeSection, setActiveSection] = useState<DomainMenuSection>(null);
  const [menuPos, setMenuPos] = useState({ top: 0, left: 0 });
  const [domainMenuState, setDomainMenuState] = useState<Record<string, DomainMenuState>>({});
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
    await runEnumerateForDomain(domain.id, request);
  };

  const runFetch = async (domain: Domain) => {
    const state = domainMenuState[domain.id] || defaultMenuState();
    const request: FetchRequest = {
      mode: state.fetchMode,
      status: state.fetchStatus || "new",
      limit: state.fetchLimit || 100,
      config: {
        concurrency: state.fetchConcurrency || 4,
      },
    };
    await runFetchForDomain(domain.id, request);
  };

  const endpointLabel = (endpoint: Endpoint): string => {
    if (endpoint.path) return endpoint.path;
    try {
      return new URL(endpoint.url).pathname || "/";
    } catch {
      return endpoint.url;
    }
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
                {domain.endpoints.map((endpoint, idx) => {
                  const isSelected = selectedEndpoint?.id === endpoint.id;
                  const isLast = idx === domain.endpoints.length - 1;
                  return (
                    <div key={endpoint.id} className="relative">
                      <div
                        onClick={() => void selectEndpoint(domain.id, endpoint.id)}
                        className={`group flex items-center px-10 py-1.5 cursor-pointer transition-all ${isSelected ? "text-accent" : "text-slate-500 hover:text-slate-300"}`}
                      >
                        <span className="font-mono text-slate-800 mr-2 select-none">{isLast ? "└" : "├"}</span>
                        <span className={`text-[12px] font-medium tracking-tight truncate ${isSelected ? "font-bold" : ""}`}>
                          {endpointLabel(endpoint)}
                        </span>
                      </div>
                    </div>
                  );
                })}
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
          className="w-[600px] bg-card border border-border rounded-xl shadow-[0_15px_40px_rgba(0,0,0,0.6)] animate-in fade-in zoom-in-95 duration-100 overflow-hidden"
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
                        <option value="new">new</option>
                        <option value="pending">pending</option>
                        <option value="fetched">fetched</option>
                        <option value="failed">failed</option>
                        <option value="filtered">filtered</option>
                        <option value="*">all</option>
                      </select>
                    </label>
                    <label className="text-[11px] text-slate-300 flex flex-col gap-1">
                      Limit
                      <input
                        type="number"
                        min={1}
                        max={2000}
                        value={currentMenuState.fetchLimit}
                        onChange={(event) =>
                          updateDomainMenuState(openMenuId, (state) => ({
                            ...state,
                            fetchLimit: Number(event.target.value) || 100,
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
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
