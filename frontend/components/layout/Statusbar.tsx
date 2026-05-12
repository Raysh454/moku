import React, { useEffect, useMemo, useRef, useState } from "react";
import { useProject } from "../../context/ProjectContext";

export const Statusbar: React.FC = () => {
  const { activeProject, selectedDomain, selectedSnapshot, jobs, refreshJobs, isBusy } = useProject();
  const [showJobs, setShowJobs] = useState(false);
  const [projectFilter, setProjectFilter] = useState("");
  const [siteFilter, setSiteFilter] = useState("");
  const jobsMenuRef = useRef<HTMLDivElement | null>(null);

  const projectOptions = useMemo(
    () => Array.from(new Set(jobs.map((job) => job.project).filter(Boolean))).sort(),
    [jobs],
  );
  const siteOptions = useMemo(
    () => Array.from(new Set(jobs.map((job) => job.website).filter(Boolean))).sort(),
    [jobs],
  );

  const filteredJobs = useMemo(
    () =>
      jobs
        .filter((job) => {
        if (projectFilter && job.project !== projectFilter) return false;
        if (siteFilter && job.website !== siteFilter) return false;
        return true;
      })
        .sort((left, right) => new Date(right.started_at).getTime() - new Date(left.started_at).getTime()),
    [jobs, projectFilter, siteFilter],
  );

  useEffect(() => {
    if (!showJobs) return;
    const onDocumentMouseDown = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (target && jobsMenuRef.current && !jobsMenuRef.current.contains(target)) {
        setShowJobs(false);
      }
    };
    document.addEventListener("mousedown", onDocumentMouseDown);
    return () => {
      document.removeEventListener("mousedown", onDocumentMouseDown);
    };
  }, [showJobs]);

  if (!activeProject) return null;

  return (
    <footer className="h-10 border-t border-border bg-black flex items-center justify-between px-8 text-[10px] z-30 fixed bottom-0 left-0 right-0 font-medium uppercase tracking-widest">
      <div className="flex items-center gap-6">
        <div className="flex items-center gap-2">
          <div
            className={`w-1.5 h-1.5 rounded-full ${activeProject.status === "active" ? "bg-success shadow-[0_0_8px_rgba(0,212,170,0.4)]" : "bg-slate-700"}`}
          ></div>
          <span className="text-primary font-semibold">{activeProject.status}</span>
        </div>
        <div className="h-3 w-px bg-border/50"></div>
        <span className="text-helper font-semibold">
          Engine: <span className="text-slate-400 italic font-normal">{isBusy ? "Busy" : "Idle"}</span>
        </span>
      </div>

      <div className="flex items-center gap-6 relative">
        {selectedSnapshot && (
          <>
            <div className="flex items-center gap-2">
              <span className="text-helper">Domain:</span>
              <span className="text-slate-300 tabular-nums">{selectedDomain?.hostname || "n/a"}</span>
            </div>
            <div className="h-3 w-px bg-border/50"></div>
            <div className="flex items-center gap-2">
              <span className="text-helper">Last Scan:</span>
              <span className="text-slate-300 tabular-nums">{new Date(selectedSnapshot.createdAt).toLocaleTimeString()}</span>
            </div>
          </>
        )}

        <div ref={jobsMenuRef}>
          <button
            className="px-3 py-1 rounded border border-border text-helper hover:text-white hover:border-slate-500 transition"
            onClick={() => setShowJobs((open) => !open)}
          >
            Jobs ({jobs.length})
          </button>

          {showJobs && (
            <div className="absolute bottom-11 right-0 w-[560px] max-h-[420px] bg-card border border-border rounded-xl shadow-[0_20px_60px_rgba(0,0,0,0.8)] overflow-hidden normal-case tracking-normal">
              <div className="px-4 py-3 border-b border-border bg-bg/40 flex items-center justify-between">
                <span className="text-[11px] uppercase tracking-[0.2em] text-helper font-black">Job Queue</span>
                <button
                  className="text-[10px] uppercase tracking-widest text-accent hover:text-white"
                  onClick={() => void refreshJobs()}
                >
                  Refresh
                </button>
              </div>

              <div className="px-4 py-2 border-b border-border bg-bg/30 grid grid-cols-2 gap-2">
                <select
                  value={projectFilter}
                  onChange={(event) => setProjectFilter(event.target.value)}
                  className="w-full bg-bg border border-border rounded px-2 py-1 text-[11px] text-slate-200"
                >
                  <option value="">All projects</option>
                  {projectOptions.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
                <select
                  value={siteFilter}
                  onChange={(event) => setSiteFilter(event.target.value)}
                  className="w-full bg-bg border border-border rounded px-2 py-1 text-[11px] text-slate-200"
                >
                  <option value="">All websites</option>
                  {siteOptions.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </div>

              <div className="max-h-[300px] overflow-y-auto custom-scrollbar">
                {filteredJobs.length === 0 ? (
                  <p className="px-4 py-8 text-center text-xs text-slate-500">No jobs found for selected filters.</p>
                ) : (
                  <div className="divide-y divide-border/40">
                    {filteredJobs.map((job) => (
                      <div key={job.id} className="px-4 py-3 text-xs">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <span className="text-slate-100 font-semibold">{job.type}</span>
                            <span
                              className={`px-2 py-0.5 rounded uppercase text-[10px] tracking-wider ${
                                job.status === "done"
                                  ? "bg-success/20 text-success"
                                  : job.status === "failed" || job.status === "canceled"
                                    ? "bg-danger/20 text-danger"
                                    : "bg-warning/20 text-warning"
                              }`}
                            >
                              {job.status}
                            </span>
                          </div>
                          <span className="text-slate-500 font-mono">{job.id.slice(0, 8)}</span>
                        </div>
                        <div className="mt-1 text-slate-400">
                          <span className="mr-3">project: {job.project}</span>
                          <span>website: {job.website}</span>
                        </div>
                        <div className="mt-1 text-slate-500">
                          started: {new Date(job.started_at).toLocaleString()}
                          {job.ended_at ? ` • ended: ${new Date(job.ended_at).toLocaleString()}` : ""}
                        </div>
                        {job.error && <div className="mt-1 text-danger">error: {job.error}</div>}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </footer>
  );
};
