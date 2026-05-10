import React from "react";
import { useProject } from "../../context/ProjectContext";
import { useNavigate } from "react-router-dom";

export const Topbar: React.FC = () => {
  const { activeProject, openSettings } = useProject();
  const navigate = useNavigate();

  return (
    <header className="h-16 border-b border-border bg-card/60 backdrop-blur-md flex items-center justify-between px-2 z-30 sticky top-0">
      <div className="flex items-center gap-0">
        <button
          onClick={() => navigate("/")}
          className="p-2 text-slate-500 hover:text-white hover:bg-white/5 rounded-lg transition-all"
          title="Back to Projects"
        >
          <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 19l-7-7 7-7"
            />
          </svg>
        </button>

        <div className="flex items-center gap-3">
          <img src="/MOKU_icon.png" alt="MOKU Logo" className="h-14 w-auto" />

          {activeProject && (
            <div className="flex flex-col leading-tight">
              <span className="text-[10px] text-helper font-semibold uppercase tracking-[0.2em] mb-1">
                Active Workspace
              </span>
              <span className="text-base text-primary font-semibold tracking-tight">
                {activeProject.name}
              </span>
            </div>
          )}
        </div>
      </div>

      <div className="flex items-center gap-3 pr-2">
        <button
          className="h-9 w-9 flex items-center justify-center text-helper hover:text-primary hover:bg-white/5 rounded-lg transition-all active:scale-95"
          title="Settings"
          onClick={openSettings}
        >
          <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
            />
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
            />
          </svg>
        </button>

        <button className="h-9 px-5 flex items-center gap-2.5 text-[11px] font-semibold uppercase tracking-widest text-white bg-success rounded-lg hover:brightness-110 transition-all active:scale-95 shadow-lg shadow-success/20 group">
          <svg
            className="w-3 h-3 fill-current group-hover:scale-110 transition-transform"
            viewBox="0 0 24 24"
          >
            <path d="M5 3l14 9-14 9V3z" />
          </svg>
          Live Scan
        </button>
      </div>
    </header>
  );
};
