import React from "react";
import { useProject } from "../../context/ProjectContext";
import { useNavigate } from "react-router-dom";
import { LoadingSpinner } from "../../components/common/LoadingSpinner";
import { Badge } from "../../components/common/Badge";
import { Folder, ChevronRight, Trash2 } from "lucide-react";

const ProjectSelectPage: React.FC = () => {
  const { projects, isLoading, setActiveProjectById, deleteProject } = useProject();
  const navigate = useNavigate();

  const handleOpenProject = async (id: string) => {
    await setActiveProjectById(id);
    navigate("/workspace");
  };

  const handleDeleteProject = async (e: React.MouseEvent, slug: string) => {
    e.stopPropagation();
    if (window.confirm(`Are you sure you want to delete project "${slug}"?`)) {
        await deleteProject(slug);
    }
  };

  if (isLoading && projects.length === 0)
    return (
      <div className="h-screen bg-bg flex flex-col items-center justify-center space-y-4">
        <LoadingSpinner />
        <span className="text-xs text-slate-400 uppercase font-medium tracking-widest animate-pulse">
          Loading projects...
        </span>
      </div>
    );

  return (
    <div className="flex h-screen overflow-hidden">
      <div className="flex-1 bg-bg flex flex-col items-center justify-center relative">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,rgba(81,112,255,0.15)_0%,rgba(81,112,255,0.08)_40%,transparent_70%)] pointer-events-none"></div>
        <div className="flex flex-col items-center text-center animate-in fade-in zoom-in-95 duration-700 relative z-10">
          <img
            src="/MOKU_main-nobg.png?v=1"
            alt="MOKU Logo"
            className="h-80 w-auto -mb-8"
            loading="eager"
            decoding="async"
            draggable={false}
          />

          <h2 className="text-lg font-normal text-slate-300 tracking-normal mb-6">
            Easily track and monitor your projects
          </h2>

          <button
            onClick={() => navigate("/create")}
            className="group flex items-center gap-2 px-8 py-2.5 bg-accent hover:bg-accent/90 text-white font-bold rounded-full transition-all active:scale-95"
          >
            <span className="text-2xl leading-none font-light group-hover:rotate-90 transition-transform duration-300">
              +
            </span>
            <span className="text-base tracking-tight uppercase">
              New Project
            </span>
          </button>
        </div>

        <div className="absolute bottom-8 text-[11px] font-medium text-slate-600 uppercase tracking-wider">
          v1.0.4
        </div>
      </div>

      <div className="w-[45%] bg-card border-l border-border flex flex-col overflow-hidden">
        <div className="px-8 pt-8 pb-6 border-b border-border/50">
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-3 mb-2">
                <Folder className="w-5 h-5 text-accent" strokeWidth={2.5} />
                <h1 className="text-xl font-semibold text-white">
                  Existing Projects
                </h1>
              </div>
              <p className="text-xs text-slate-500">
                {projects.length}{" "}
                {projects.length === 1 ? "project" : "projects"}
              </p>
            </div>
          </div>
        </div>

        <div
          className="flex-1 overflow-y-auto"
          style={{
            scrollbarWidth: "thin",
            scrollbarColor: "#1f1f35 transparent",
          }}
        >
          {projects.length > 0 ? (
            <div className="px-8 py-6 grid grid-cols-2 gap-4">
              {projects.map((project) => (
                <div
                  key={project.id}
                  onClick={() => handleOpenProject(project.id)}
                  className="group bg-white/[0.02] hover:bg-white/[0.05] border border-border hover:border-accent/30 rounded-xl p-4 cursor-pointer transition-all active:scale-[0.98] flex items-center justify-between"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3 mb-2">
                      <div className="flex-1 min-w-0">
                        <h3 className="text-sm font-semibold text-white truncate group-hover:text-accent transition-colors">
                          {project.name}
                        </h3>
                        <p className="text-xs text-slate-500 truncate mt-0.5">
                          {project.description || "No description"}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center gap-3 ml-9 whitespace-nowrap">
                      <Badge variant={project.status} className="flex-shrink-0">
                        {project.status}
                      </Badge>
                      <span className="text-[11px] text-slate-500 whitespace-nowrap">
                        {new Date(project.createdAt).toLocaleDateString(
                          "en-US",
                          {
                            month: "short",
                            day: "numeric",
                            year: "numeric",
                          },
                        )}
                      </span>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 ml-4">
                    <button
                      onClick={(e) => handleDeleteProject(e, project.slug)}
                      className="p-1.5 text-slate-600 hover:text-danger hover:bg-danger/10 rounded transition-all"
                      title="Delete project"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                    <ChevronRight
                      className="w-4 h-4 text-slate-600 group-hover:text-accent group-hover:translate-x-0.5 transition-all"
                      strokeWidth={2}
                    />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full py-24 text-center">
              <div className="w-16 h-16 bg-white/5 rounded-xl flex items-center justify-center mb-4 border border-border/50">
                <svg
                  className="w-8 h-8 text-slate-600"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={1.5}
                    d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
                  />
                </svg>
              </div>
              <h3 className="text-sm font-medium text-slate-400 mb-2">
                No projects yet
              </h3>
              <p className="text-xs text-slate-600 max-w-xs leading-relaxed">
                Create a new project to start monitoring your API endpoints
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default ProjectSelectPage;
