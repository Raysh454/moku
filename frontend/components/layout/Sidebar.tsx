import React, { useState } from "react";
import { DomainTree } from "../explorer/DomainTree";
import { useProject } from "../../context/ProjectContext";

export const Sidebar: React.FC = () => {
  const { activeProject, createWebsiteForActiveProject, refreshActiveProject, isBusy } = useProject();
  const [isCollapsed, setIsCollapsed] = useState(false);

  const addWebsite = async () => {
    if (!activeProject) return;
    const slug = window.prompt("Website slug (example: main-site)");
    if (!slug) return;
    const origin = window.prompt("Website origin (example: https://example.com)");
    if (!origin) return;
    await createWebsiteForActiveProject({
      slug: slug.trim(),
      origin: origin.trim(),
    });
  };

  return (
    <aside
      className={`bg-card flex flex-col h-full flex-shrink-0 z-20 transition-all duration-300 ease-in-out border-r border-border ${
        isCollapsed ? "w-14" : "w-72"
      }`}
    >
      <div
        className={`px-4 py-3 border-b border-border bg-card/40 flex items-center ${isCollapsed ? "justify-center px-0" : "justify-between px-6"}`}
      >
        {!isCollapsed && (
          <div className="flex items-center gap-2 flex-1 animate-in fade-in duration-300">
            <h3 className="text-[12px] font-semibold text-slate-500 uppercase tracking-[0.3em] truncate">
              Explorer
            </h3>
            <button
              className="p-1 text-slate-600 hover:text-success transition-colors"
              title="Add Website"
              onClick={() => void addWebsite()}
              disabled={isBusy || !activeProject}
            >
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
            </button>
            <button
              className="p-1 text-slate-600 hover:text-accent transition-colors"
              title="Refresh Explorer"
              onClick={() => void refreshActiveProject()}
              disabled={isBusy || !activeProject}
            >
              <svg
                className="w-3 h-3"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
            </button>
          </div>
        )}
        <button
          onClick={() => setIsCollapsed(!isCollapsed)}
          className={`text-slate-600 hover:text-accent transition-all duration-300 transform flex-shrink-0 ${isCollapsed ? "rotate-180" : ""}`}
          title={isCollapsed ? "Expand Sidebar" : "Collapse Sidebar"}
        >
          <svg
            className="w-4 h-4"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M11 19l-7-7 7-7m8 14l-7-7 7-7"
            />
          </svg>
        </button>
      </div>
      <div className="flex-1 overflow-hidden">
        <DomainTree isCollapsed={isCollapsed} />
      </div>
    </aside>
  );
};
