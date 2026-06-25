import { Navigate } from "react-router-dom";
import { Topbar } from "../../components/layout/Topbar";
import { Sidebar } from "../../components/layout/Sidebar";
import { Statusbar } from "../../components/layout/Statusbar";
import { EditorShell } from "../../components/editor/EditorShell";
import { FilterSettingsModal } from "../../components/settings/FilterSettingsModal";
import { useProject } from "../../context/ProjectContext";
import { useEditor } from "../../context/EditorContext";

const WorkspacePage = () => {
  const { activeProject, settingsOpen, closeSettings, refreshActiveProject, isLoading } = useProject();
  const { activeDomain } = useEditor();

  if (isLoading) {
    return <div className="flex h-screen items-center justify-center bg-bg text-sm text-helper">Loading workspace…</div>;
  }
  if (!activeProject) return <Navigate to="/" replace />;

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-bg">
      <Topbar />
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <Sidebar />
        <EditorShell />
      </div>
      <Statusbar />

      <FilterSettingsModal
        open={settingsOpen}
        projectSlug={activeProject.slug}
        siteSlug={activeDomain?.slug}
        onClose={closeSettings}
        onChanged={refreshActiveProject}
      />
    </div>
  );
};

export default WorkspacePage;
