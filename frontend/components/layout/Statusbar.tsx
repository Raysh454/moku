import { useEffect, useMemo, useRef, useState } from "react";
import { useProject } from "../../context/ProjectContext";
import { useEditor } from "../../context/EditorContext";

export const Statusbar = () => {
  const { activeProject, jobs, refreshJobs, isBusy } = useProject();
  const { activeDomain, headSnapshot } = useEditor();
  const [showJobs, setShowJobs] = useState(false);
  const [projectFilter, setProjectFilter] = useState("");
  const [siteFilter, setSiteFilter] = useState("");
  const jobsMenuRef = useRef<HTMLDivElement | null>(null);

  const projectOptions = useMemo(
    () => Array.from(new Set(jobs.map((job) => job.project).filter(Boolean))).sort(),
    [jobs],
  );
  const siteOptions = useMemo(() => Array.from(new Set(jobs.map((job) => job.website).filter(Boolean))).sort(), [jobs]);

  const filteredJobs = useMemo(
    () =>
      jobs
        .filter((job) => {
          if (projectFilter && job.project !== projectFilter) return false;
          if (siteFilter && job.website !== siteFilter) return false;
          return true;
        })
        .sort((left, right) => new Date(right.started_at ?? 0).getTime() - new Date(left.started_at ?? 0).getTime()),
    [jobs, projectFilter, siteFilter],
  );

  useEffect(() => {
    if (!showJobs) return;
    const onMouseDown = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (target && jobsMenuRef.current && !jobsMenuRef.current.contains(target)) setShowJobs(false);
    };
    document.addEventListener("mousedown", onMouseDown);
    return () => document.removeEventListener("mousedown", onMouseDown);
  }, [showJobs]);

  if (!activeProject) return null;

  return (
    <footer className="flex h-9 items-center justify-between border-t border-border bg-black/60 px-6 text-[11px] text-helper">
      <div className="flex items-center gap-5">
        <span className="flex items-center gap-2">
          <span
            className={`h-1.5 w-1.5 rounded-full ${activeProject.status === "active" ? "bg-success" : "bg-slate-700"}`}
          />
          <span className="font-medium text-primary">{activeProject.status}</span>
        </span>
        <span>
          Engine: <span className="text-helper">{isBusy ? "busy" : "idle"}</span>
        </span>
      </div>

      <div className="relative flex items-center gap-5">
        {headSnapshot ? (
          <>
            <span>
              Domain: <span className="tabular-nums text-primary">{activeDomain?.hostname || "n/a"}</span>
            </span>
            <span>
              Snapshot: <span className="tabular-nums text-primary">{new Date(headSnapshot.createdAt).toLocaleTimeString()}</span>
            </span>
          </>
        ) : null}

        <div ref={jobsMenuRef}>
          <button
            className="rounded border border-border px-3 py-1 text-helper transition hover:border-slate-500 hover:text-primary"
            onClick={() => setShowJobs((open) => !open)}
          >
            Jobs ({jobs.length})
          </button>

          {showJobs ? (
            <div className="absolute bottom-10 right-0 max-h-[420px] w-[560px] overflow-hidden rounded-xl border border-border bg-card">
              <div className="flex items-center justify-between border-b border-border bg-bg/40 px-4 py-3">
                <span className="text-xs font-semibold text-primary">Job queue</span>
                <button className="text-xs text-accent hover:text-primary" onClick={() => void refreshJobs()}>
                  Refresh
                </button>
              </div>
              <div className="grid grid-cols-2 gap-2 border-b border-border bg-bg/30 px-4 py-2">
                <select
                  value={projectFilter}
                  onChange={(event) => setProjectFilter(event.target.value)}
                  className="rounded border border-border bg-bg px-2 py-1 text-[11px] text-primary"
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
                  className="rounded border border-border bg-bg px-2 py-1 text-[11px] text-primary"
                >
                  <option value="">All websites</option>
                  {siteOptions.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </div>
              <div className="custom-scrollbar max-h-[300px] overflow-y-auto">
                {filteredJobs.length === 0 ? (
                  <p className="px-4 py-8 text-center text-xs text-muted">No jobs for the selected filters.</p>
                ) : (
                  <div className="divide-y divide-border/40">
                    {filteredJobs.map((job) => {
                      const hasProgress = job.status !== "failed" && job.processed !== undefined && job.total !== undefined && job.total > 0;
                      const pct = hasProgress ? Math.min(100, Math.max(0, ((job.processed ?? 0) / (job.total ?? 1)) * 100)) : 0;
                      return (
                        <div key={job.id} className="px-4 py-3 text-xs">
                          <div className="flex items-center justify-between">
                            <span className="flex items-center gap-2">
                              <span className="font-semibold text-primary">{job.type}</span>
                              <span
                                className={`rounded px-2 py-0.5 text-[10px] uppercase ${
                                  job.status === "done"
                                    ? "bg-success/20 text-success"
                                    : job.status === "failed" || job.status === "canceled"
                                      ? "bg-danger/20 text-danger"
                                      : "bg-warning/20 text-warning"
                                }`}
                              >
                                {job.status}
                              </span>
                            </span>
                            <span className="font-mono text-muted">{(job.id ?? "").slice(0, 8)}</span>
                          </div>
                          {hasProgress ? (
                            <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-border">
                              <div className="h-full bg-accent transition-all" style={{ width: `${pct}%` }} />
                            </div>
                          ) : null}
                          <div className="mt-1 text-helper">
                            {job.project} / {job.website}
                          </div>
                          {job.error ? <div className="mt-1 text-danger">error: {job.error}</div> : null}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </footer>
  );
};
