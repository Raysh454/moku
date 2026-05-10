import React from "react";
import { useProject } from "../../context/ProjectContext";

export const Statusbar: React.FC = () => {
  const { activeProject, selectedSnapshot } = useProject();

  if (!activeProject) return null;

  return (
    <footer className="h-10 border-t border-border bg-black flex items-center justify-between px-8 text-[10px] z-30 fixed bottom-0 left-0 right-0 font-medium uppercase tracking-widest">
      <div className="flex items-center gap-6">
        <div className="flex items-center gap-2">
          <div
            className={`w-1.5 h-1.5 rounded-full ${activeProject.status === "active" ? "bg-success shadow-[0_0_8px_rgba(0,212,170,0.4)]" : "bg-slate-700"}`}
          ></div>
          <span className="text-primary font-semibold">
            {activeProject.status}
          </span>
        </div>
        <div className="h-3 w-px bg-border/50"></div>
        <span className="text-helper font-semibold">
          Engine:{" "}
          <span className="text-slate-400 italic font-normal">Idle</span>
        </span>
      </div>

      <div className="flex items-center gap-6">
        {selectedSnapshot && (
          <>
            <div className="flex items-center gap-2">
              <span className="text-helper">Build:</span>
              <span className="text-accent font-semibold tabular-nums">
                v{selectedSnapshot.version}
              </span>
            </div>
            <div className="h-3 w-px bg-border/50"></div>
            <div className="flex items-center gap-2">
              <span className="text-helper">Last Scan:</span>
              <span className="text-slate-300 tabular-nums">
                {new Date(selectedSnapshot.createdAt).toLocaleTimeString()}
              </span>
            </div>
            <div className="h-3 w-px bg-border/50"></div>
            <div className="flex items-center gap-2">
              <span className="text-helper">Health:</span>
              <span
                className={`font-semibold tabular-nums ${(selectedSnapshot.scoreResult?.normalized || 0) > 80 ? "text-success" : "text-danger"}`}
              >
                {selectedSnapshot.scoreResult?.normalized || "N/A"}%
              </span>
            </div>
          </>
        )}
      </div>
    </footer>
  );
};
