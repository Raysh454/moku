import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useProject } from "../../context/ProjectContext";

const ProjectCreatePage: React.FC = () => {
  const navigate = useNavigate();
  const { createNewProject, setActiveProjectById } = useProject();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [domains, setDomains] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setIsCreating(true);

    try {
      const project = await createNewProject({
        name: name.trim(),
        description,
      });

      await setActiveProjectById(project.id);
      navigate("/workspace");
    } catch (err) {
      setIsCreating(false);
    }
  };

  return (
    <div className="min-h-screen bg-bg flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-white mb-2">Create Project</h1>
          <p className="text-sm text-slate-400">
            Set up a new scanning project to monitor websites.
          </p>
        </div>

        <form onSubmit={handleCreate} className="space-y-5">
          <div>
            <label className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 block">
              Name
            </label>
            <input
              required
              disabled={isCreating}
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Project"
              className="w-full bg-card border border-border rounded-lg px-3 py-2.5 text-sm text-white focus:outline-none focus:ring-1 focus:ring-accent/40 transition-all placeholder:text-slate-600 disabled:opacity-50"
            />
          </div>

          <div>
            <label className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 block">
              Description
            </label>
            <input
              disabled={isCreating}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional project description"
              className="w-full bg-card border border-border rounded-lg px-3 py-2.5 text-sm text-white focus:outline-none focus:ring-1 focus:ring-accent/40 transition-all placeholder:text-slate-600 disabled:opacity-50"
            />
          </div>

          <div>
            <label className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2 block flex items-center gap-2">
              <span>Endpoints</span>
              <span className="text-slate-500 font-normal text-[10px]">
                One per line
              </span>
            </label>
            <textarea
              disabled={isCreating}
              value={domains}
              onChange={(e) => setDomains(e.target.value)}
              placeholder="https://example.com/users&#10;https://example.com/login&#10;api.example.com"
              className="w-full h-24 px-3 py-2.5 bg-card border border-border rounded-lg text-slate-300 font-mono text-xs focus:outline-none focus:ring-1 focus:ring-accent/40 transition-all resize-none placeholder:text-slate-600 disabled:opacity-50 custom-scrollbar"
            />
            <p className="mt-2 text-[11px] text-slate-500">
              Endpoint bulk import is kept in UI for later integration. Add websites from the explorer in workspace.
            </p>
          </div>

          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={() => navigate("/")}
              disabled={isCreating}
              className="flex-1 px-4 py-2.5 text-sm font-medium text-slate-400 hover:text-white transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isCreating}
              className="flex-1 px-4 py-2.5 bg-accent hover:bg-accent/90 text-white font-semibold text-sm rounded-lg transition-all disabled:opacity-50 flex items-center justify-center gap-2"
            >
              {isCreating ? (
                <>
                  <div className="w-4 h-4 border-2 border-white/20 border-t-white rounded-full animate-spin"></div>
                  Creating...
                </>
              ) : (
                "Create Project"
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default ProjectCreatePage;
