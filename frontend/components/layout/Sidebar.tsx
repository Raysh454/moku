import React, { useState } from "react";
import { DomainTree } from "../explorer/DomainTree";
import { useProject } from "../../context/ProjectContext";

export const Sidebar: React.FC = () => {
  const { activeProject, createWebsiteForActiveProject, refreshActiveProject, isBusy, domainOverviews } = useProject();
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [showWebsiteModal, setShowWebsiteModal] = useState(false);
  const [slug, setSlug] = useState("");
  const [origin, setOrigin] = useState("");
  const [formError, setFormError] = useState("");

  const closeModal = () => {
    setShowWebsiteModal(false);
    setSlug("");
    setOrigin("");
    setFormError("");
  };

  const openModal = () => {
    setFormError("");
    setShowWebsiteModal(true);
  };

  const submitWebsite = async (event: React.FormEvent) => {
    event.preventDefault();
    const nextSlug = slug.trim();
    const nextOrigin = origin.trim();

    if (!nextSlug) {
      setFormError("Slug is required");
      return;
    }

    if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(nextSlug)) {
      setFormError("Slug must use lowercase letters, numbers, and hyphens only");
      return;
    }

    try {
      const parsed = new URL(nextOrigin);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
        setFormError("Origin must start with http:// or https://");
        return;
      }
    } catch {
      setFormError("Origin must be a valid URL");
      return;
    }

    try {
      await createWebsiteForActiveProject({ slug: nextSlug, origin: nextOrigin });
      closeModal();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to create website");
    }
  };

  return (
    <>
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
                onClick={openModal}
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
          <DomainTree isCollapsed={isCollapsed} domainOverviews={domainOverviews} />
        </div>
      </aside>

      {showWebsiteModal && (
        <div className="fixed inset-0 z-[120] bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <form
            className="w-full max-w-md bg-card border border-border rounded-2xl shadow-2xl p-6 space-y-4"
            onSubmit={(event) => void submitWebsite(event)}
          >
            <div>
              <h3 className="text-sm font-black uppercase tracking-[0.25em] text-helper">Add Website</h3>
              <p className="text-xs text-slate-400 mt-1">Create a website target for this project.</p>
            </div>

            <label className="block">
              <span className="text-[10px] font-bold text-helper uppercase tracking-widest">Slug</span>
              <input
                value={slug}
                onChange={(event) => setSlug(event.target.value)}
                placeholder="main-site"
                className="mt-1 w-full bg-bg border border-border rounded-lg px-3 py-2.5 text-sm text-white focus:outline-none focus:ring-1 focus:ring-accent/40"
              />
            </label>

            <label className="block">
              <span className="text-[10px] font-bold text-helper uppercase tracking-widest">Origin</span>
              <input
                value={origin}
                onChange={(event) => setOrigin(event.target.value)}
                placeholder="https://example.com"
                className="mt-1 w-full bg-bg border border-border rounded-lg px-3 py-2.5 text-sm text-white focus:outline-none focus:ring-1 focus:ring-accent/40"
              />
            </label>

            {formError && <p className="text-xs text-danger">{formError}</p>}

            <div className="flex items-center justify-end gap-2 pt-1">
              <button
                type="button"
                onClick={closeModal}
                className="px-4 py-2 rounded-lg border border-border text-xs font-bold uppercase tracking-widest text-slate-300 hover:text-white"
                disabled={isBusy}
              >
                Cancel
              </button>
              <button
                type="submit"
                className="px-4 py-2 rounded-lg bg-accent text-white text-xs font-black uppercase tracking-widest hover:brightness-110 disabled:opacity-50"
                disabled={isBusy || !activeProject}
              >
                {isBusy ? "Creating..." : "Create Website"}
              </button>
            </div>
          </form>
        </div>
      )}
    </>
  );
};
