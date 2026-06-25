import type { Snapshot } from "../../../types/project";
import { Panel, SectionHeading } from "../../ui";

interface TextDiffViewProps {
  headSnapshot: Snapshot;
}

// NOTE: Phase 5 replaces the chunk rendering below with @pierre/diffs fed the
// full base/head text. This is the interim port of the original chunk view.
export function TextDiffView({ headSnapshot }: TextDiffViewProps) {
  const diff = headSnapshot.diff;
  const chunks = diff?.body_diff?.chunks || [];
  const headerDiff = diff?.headers_diff;

  return (
    <div className="space-y-4">
      <Panel tone="sunken" className="space-y-4">
        <div className="flex items-center justify-between">
          <SectionHeading title={`Body diff${diff?.file_path ? ` · ${diff.file_path}` : ""}`} size="sub" />
          <div className="flex gap-4 text-[11px] font-medium">
            <span className="text-success">Added</span>
            <span className="text-danger">Removed</span>
            <span className="text-warning">Changed</span>
          </div>
        </div>
        {chunks.length > 0 ? (
          <div className="space-y-1 overflow-x-auto font-mono text-xs tabular-nums">
            {chunks.map((chunk, index) => (
              <div
                key={`${chunk.type}-${chunk.base_start || 0}-${chunk.head_start || 0}-${index}`}
                className={`flex gap-4 rounded px-3 py-1.5 ${
                  chunk.type === "added"
                    ? "bg-success/10 text-success"
                    : chunk.type === "removed"
                      ? "bg-danger/10 text-danger"
                      : chunk.type === "changed"
                        ? "bg-warning/10 text-warning"
                        : "text-helper"
                }`}
              >
                <span className="w-8 text-right opacity-40">{index + 1}</span>
                <span className="whitespace-pre-wrap">{chunk.content || ""}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="py-12 text-center text-sm text-muted">No body diff for this version pair.</p>
        )}
      </Panel>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <HeaderDiffColumn title="Added headers" tone="text-success" entries={Object.entries(headerDiff?.added || {})} render={(values) => values.join(", ")} />
        <HeaderDiffColumn title="Removed headers" tone="text-danger" entries={Object.entries(headerDiff?.removed || {})} render={(values) => values.join(", ")} />
        <HeaderDiffColumn
          title="Changed headers"
          tone="text-warning"
          entries={Object.entries(headerDiff?.changed || {})}
          render={(change) => `${(change.from ?? []).join(", ")} → ${(change.to ?? []).join(", ")}`}
        />
      </div>
    </div>
  );
}

interface HeaderDiffColumnProps<T> {
  title: string;
  tone: string;
  entries: Array<[string, T]>;
  render: (value: T) => string;
}

function HeaderDiffColumn<T>({ title, tone, entries, render }: HeaderDiffColumnProps<T>) {
  return (
    <Panel tone="sunken" padding="sm">
      <h4 className={`mb-2 text-xs font-semibold ${tone}`}>{title}</h4>
      <ul className="space-y-1 text-xs text-helper">
        {entries.length === 0 ? <li className="text-muted">None</li> : null}
        {entries.map(([key, value]) => (
          <li key={key}>
            <span className="font-semibold text-primary">{key}:</span> {render(value)}
          </li>
        ))}
      </ul>
    </Panel>
  );
}
