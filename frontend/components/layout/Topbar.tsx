import { useNavigate } from "react-router-dom";
import { useProject } from "../../context/ProjectContext";
import { IconButton, Logo, ThemeMenu } from "../ui";
import { ArrowLeft, Settings } from "../ui/icons";

export const Topbar = () => {
  const { activeProject, openSettings } = useProject();
  const navigate = useNavigate();

  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-border bg-card/60 px-3 backdrop-blur-md">
      <div className="flex items-center gap-2">
        <IconButton icon={ArrowLeft} label="Back to projects" onClick={() => navigate("/")} />
        <Logo className="h-8 w-8" />
        {activeProject ? (
          <div className="flex flex-col leading-tight">
            <span className="text-[11px] text-helper">Workspace</span>
            <span className="text-sm font-semibold tracking-tight text-primary">{activeProject.name}</span>
          </div>
        ) : null}
      </div>

      <div className="flex items-center gap-1">
        <ThemeMenu />
        <IconButton icon={Settings} label="Settings" onClick={openSettings} />
      </div>
    </header>
  );
};
