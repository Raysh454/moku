import { useEditor } from "../../context/EditorContext";
import { FileText, X } from "../ui/icons";

/** VS Code-style strip of open endpoint tabs. */
export function EditorTabBar() {
  const { openEditors, activeEditorId, setActiveEditor, closeEditor } = useEditor();
  if (openEditors.length === 0) return null;

  return (
    <div className="custom-scrollbar flex items-stretch overflow-x-auto border-b border-border bg-card/30">
      {openEditors.map((editor) => {
        const isActive = editor.id === activeEditorId;
        return (
          <div
            key={editor.id}
            data-testid="editor-tab"
            onClick={() => setActiveEditor(editor.id)}
            className={`group flex cursor-pointer items-center gap-2 border-r border-border px-3 py-2 ${
              isActive ? "bg-bg text-primary" : "text-helper hover:bg-white/5 hover:text-primary"
            }`}
          >
            <FileText className="h-3.5 w-3.5 shrink-0 opacity-70" />
            <span className="max-w-[200px] truncate font-mono text-xs">{editor.label}</span>
            <button
              type="button"
              aria-label={`Close ${editor.label}`}
              onClick={(event) => {
                event.stopPropagation();
                closeEditor(editor.id);
              }}
              className="rounded p-0.5 text-helper opacity-0 transition-opacity hover:text-danger group-hover:opacity-100"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
