import { useEditor } from "../../context/EditorContext";
import { EditorTabBar } from "./EditorTabBar";
import { CompareToolbar } from "./CompareToolbar";
import { PreviewView } from "./views/PreviewView";
import { BodyDiffView } from "./views/BodyDiffView";
import { HeaderDiffView } from "./views/HeaderDiffView";
import { AnalysisView } from "./views/AnalysisView";
import { EmptyState, Tabs, type TabItem } from "../ui";
import { Eye, FileText, GitCompare, ShieldAlert } from "../ui/icons";

const VIEW_TABS: TabItem[] = [
  { id: "preview", label: "Preview", icon: Eye },
  { id: "diff", label: "Diff", icon: GitCompare },
  { id: "analysis", label: "Analysis", icon: ShieldAlert },
];

export function EditorShell() {
  const { openEditors, activeEndpoint, viewMode, setViewMode, headSnapshot, baseSnapshot, loading, error } = useEditor();

  if (openEditors.length === 0) {
    return (
      <main className="flex flex-1 flex-col items-center justify-center border-l border-border">
        <EmptyState
          icon={FileText}
          title="No endpoint open"
          hint="Pick an endpoint from the explorer to inspect its versions, diffs, and security analysis."
        />
      </main>
    );
  }

  return (
    <main className="flex flex-1 flex-col overflow-hidden border-l border-border">
      <EditorTabBar />

      <div className="flex items-center justify-between gap-4 border-b border-border bg-card/10 px-4 py-2">
        <span className="truncate font-mono text-sm text-primary">{activeEndpoint?.path || "/"}</span>
        <Tabs items={VIEW_TABS} value={viewMode} onChange={setViewMode} ariaLabel="Editor views" size="sm" />
      </div>

      <CompareToolbar />

      <div className="custom-scrollbar flex-1 overflow-y-auto p-4">
        {error ? <p className="mb-3 text-sm text-danger">{error}</p> : null}
        {headSnapshot ? (
          <div className="mx-auto w-full max-w-6xl pb-16">
            {viewMode === "preview" ? <PreviewView headSnapshot={headSnapshot} baseSnapshot={baseSnapshot} /> : null}
            {viewMode === "diff" ? (
              <div className="space-y-4">
                <BodyDiffView headSnapshot={headSnapshot} baseSnapshot={baseSnapshot} fileName={activeEndpoint?.path || "page"} />
                <HeaderDiffView headSnapshot={headSnapshot} baseSnapshot={baseSnapshot} />
              </div>
            ) : null}
            {viewMode === "analysis" ? <AnalysisView headSnapshot={headSnapshot} /> : null}
          </div>
        ) : (
          <EmptyState title={loading ? "Loading snapshot…" : "No snapshot for the selected version"} />
        )}
      </div>
    </main>
  );
}
