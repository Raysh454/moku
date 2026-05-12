import { useMemo } from "react";
import { getSnapshotContentInfo } from "../../lib/contentView";
import type { Snapshot } from "../../types/project";

type Props = {
  snapshot: Snapshot;
};

type DirectoryEntry = {
  href: string;
  label: string;
};

const tryFormatJson = (value: string): string => {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
};

const detectDirectoryEntries = (html: string): DirectoryEntry[] => {
  try {
    const parser = new DOMParser();
    const doc = parser.parseFromString(html, "text/html");
    const links = Array.from(doc.querySelectorAll("a[href]"));
    return links
      .map((link) => ({
        href: link.getAttribute("href") || "",
        label: (link.textContent || "").trim() || link.getAttribute("href") || "",
      }))
      .filter((entry) => entry.href || entry.label)
      .slice(0, 100);
  } catch {
    return [];
  }
};

export function SnapshotContentView({ snapshot }: Props) {
  const content = useMemo(() => getSnapshotContentInfo(snapshot), [snapshot]);
  const directoryEntries = useMemo(
    () => (content.viewKind === "directory" ? detectDirectoryEntries(content.textBody) : []),
    [content.textBody, content.viewKind],
  );

  if (content.viewKind === "image") {
    return (
      <div className="rounded-xl border border-border bg-bg/50 p-4 space-y-3">
        <div className="text-[10px] uppercase tracking-[0.16em] text-helper font-black">Image preview</div>
        {content.imageSrc ? (
          <img
            src={content.imageSrc}
            alt={snapshot.url}
            className="max-h-[480px] w-auto max-w-full rounded-lg border border-border bg-black/40"
          />
        ) : (
          <p className="text-xs text-slate-500">Image data is unavailable for preview.</p>
        )}
      </div>
    );
  }

  if (content.viewKind === "directory") {
    return (
      <div className="rounded-xl border border-border bg-bg/50 p-4 space-y-3">
        <div className="text-[10px] uppercase tracking-[0.16em] text-helper font-black">Directory listing</div>
        {directoryEntries.length > 0 ? (
          <div className="max-h-[420px] overflow-auto custom-scrollbar rounded-lg border border-border bg-bg/70">
            <div className="divide-y divide-border/40">
              {directoryEntries.map((entry, index) => (
                <div key={`${entry.href}-${index}`} className="px-3 py-2 text-xs">
                  <span className="text-slate-200">{entry.label}</span>
                  <span className="ml-2 text-slate-500">{entry.href}</span>
                </div>
              ))}
            </div>
          </div>
        ) : (
          <p className="text-xs text-slate-500">No directory entries were detected.</p>
        )}
      </div>
    );
  }

  if (content.viewKind === "json" || content.viewKind === "text") {
    const text = content.viewKind === "json" ? tryFormatJson(content.textBody) : content.textBody;
    return (
      <div className="rounded-xl border border-border bg-bg/50 p-4 space-y-3">
        <div className="text-[10px] uppercase tracking-[0.16em] text-helper font-black">
          {content.viewKind === "json" ? "JSON content" : "Text content"}
        </div>
        <pre className="max-h-[480px] overflow-auto custom-scrollbar rounded-lg border border-border bg-bg/70 p-3 text-xs text-slate-200 whitespace-pre-wrap break-words">
          {text || "(empty body)"}
        </pre>
      </div>
    );
  }

  return (
    <div className="rounded-xl border border-border bg-bg/50 p-4 space-y-2">
      <div className="text-[10px] uppercase tracking-[0.16em] text-helper font-black">Binary content</div>
      <p className="text-xs text-slate-300">
        Raw binary bodies are not rendered. Use response headers and metadata for context.
      </p>
      <div className="text-xs text-slate-500">Body bytes (encoded): {content.rawBody.length}</div>
    </div>
  );
}
