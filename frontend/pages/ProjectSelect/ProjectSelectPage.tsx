import React from "react";
import { useProject } from "../../context/ProjectContext";
import { useNavigate } from "react-router-dom";
import { LoadingSpinner } from "../../components/common/LoadingSpinner";
import { Badge } from "../../components/common/Badge";
import { EmptyState, Logo, ThemeMenu } from "../../components/ui";
import { Folder, ChevronRight, Trash2, Plus } from "../../components/ui/icons";

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
        <span className="text-xs text-slate-400 uppercase font-medium tracking-wide animate-pulse">
          Loading projects...
        </span>
      </div>
    );

  return (
    <div className="flex h-screen overflow-hidden">
      <div className="flex-1 bg-bg flex flex-col items-center justify-center relative">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_center,color-mix(in_srgb,var(--color-accent)_8%,transparent)_0%,transparent_65%)]"></div>
        <div className="flex flex-col items-center text-center animate-in fade-in zoom-in-95 duration-700 relative z-10">
          <Logo className="h-24 w-24" />
          <h1 className="mt-3 text-5xl font-semibold tracking-tight text-primary">MOKU</h1>
          <h2 className="mt-4 mb-6 text-lg font-normal text-helper">
            Easily track and monitor your projects
          </h2>

          <button
            onClick={() => navigate("/create")}
            className="inline-flex items-center gap-2 rounded-lg bg-accent px-6 py-2.5 text-sm font-medium text-on-accent transition-all hover:brightness-110 active:scale-95"
          >
            <Plus className="h-4 w-4" />
            New project
          </button>
        </div>

        <div className="absolute bottom-8 text-[11px] font-medium text-slate-600 uppercase tracking-wide">
          v1.0.4
        </div>
      </div>

      <div className="w-[45%] bg-card border-l border-border flex flex-col overflow-hidden">
        <div className="px-8 pt-8 pb-6 border-b border-border/50">
          <div className="flex items-center justify-between">
            <div>
              <div className="flex items-center gap-3 mb-2">
                <Folder className="w-5 h-5 text-accent" strokeWidth={2.5} />
                <h1 className="text-xl font-semibold tracking-tight text-white">Existing projects</h1>
              </div>
              <p className="text-xs text-slate-500">
                {projects.length}{" "}
                {projects.length === 1 ? "project" : "projects"}
              </p>
            </div>
            <ThemeMenu />
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
            <EmptyState
              icon={Folder}
              title="No projects yet"
              hint="Create a new project to start monitoring websites for security changes."
              className="h-full"
            />
          )}
        </div>
      </div>
    </div>
  );
};

export default ProjectSelectPage;
