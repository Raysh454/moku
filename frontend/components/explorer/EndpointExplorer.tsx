import { useMemo, useState } from "react";
import type { Domain } from "../../types/project";
import type { SecurityDiffOverviewEntry } from "../../src/api/types";
import { useProject } from "../../context/ProjectContext";
import { useEditor } from "../../context/EditorContext";
import { buildEndpointTree } from "../../lib/endpointTree";
import { FileTreeView } from "../../adapters/tree";
import { DomainActionsBar } from "../domain/DomainActionsBar";
import { EmptyState } from "../ui";
import { Boxes, ChevronRight, Globe } from "../ui/icons";

/**
 * Sidebar body: one virtualized file tree per domain (endpoint URL paths
 * become folders/files), with endpoint status decorations and a compact
 * action bar. Replaces the flat list + hover mega-menu of the old DomainTree.
 */
export function EndpointExplorer({ isCollapsed = false }: { isCollapsed?: boolean }) {
  const { activeProject, domainOverviews } = useProject();
  const { openEndpoint, activeEndpoint } = useEditor();

  if (!activeProject || activeProject.domains.length === 0) {
    return (
      <EmptyState
        icon={Boxes}
        title="No websites yet"
        hint="Add a website, then run Enumerate and Fetch to populate the explorer."
      />
    );
  }

  if (isCollapsed) {
    return (
      <div className="flex flex-col items-center gap-2 py-3">
        {activeProject.domains.map((domain) => (
          <div
            key={domain.id}
            title={domain.hostname}
            className="flex h-7 w-7 items-center justify-center rounded border border-accent/20 bg-accent/10 text-[10px] font-bold uppercase text-accent"
          >
            {domain.hostname.charAt(0)}
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      {activeProject.domains.map((domain) => (
        <DomainSection
          key={domain.id}
          domain={domain}
          selectedEndpointId={activeEndpoint?.id ?? null}
          overview={domainOverviews.get(domain.slug)}
          onSelectEndpoint={(endpointId) => openEndpoint(domain.id, endpointId)}
        />
      ))}
    </div>
  );
}

interface DomainSectionProps {
  domain: Domain;
  selectedEndpointId: string | null;
  overview: SecurityDiffOverviewEntry[] | undefined;
  onSelectEndpoint: (endpointId: string) => void;
}

function DomainSection({ domain, selectedEndpointId, overview, onSelectEndpoint }: DomainSectionProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const nodes = useMemo(() => buildEndpointTree(domain, overview), [domain, overview]);
  const hasEndpoints = domain.endpoints.length > 0;

  return (
    <section
      className={`flex flex-col border-b border-border/60 last:border-b-0 ${isExpanded ? "min-h-0 flex-1" : "flex-none"}`}
    >
      <div className="px-3 pt-3">
        <button
          type="button"
          data-testid="website-toggle"
          aria-expanded={isExpanded}
          onClick={() => setIsExpanded((value) => !value)}
          className="flex w-full items-center gap-1.5 rounded px-1 py-1 text-left transition-colors hover:bg-white/[0.03]"
        >
          <ChevronRight
            className={`h-3.5 w-3.5 shrink-0 text-helper transition-transform ${isExpanded ? "rotate-90" : ""}`}
          />
          <Globe className="h-4 w-4 shrink-0 text-helper" />
          <span className="truncate font-mono text-[13px] font-semibold text-primary">{domain.hostname}</span>
        </button>
        {isExpanded ? (
          <div className="mt-1.5 px-1">
            <DomainActionsBar domain={domain} />
          </div>
        ) : null}
      </div>
      {isExpanded ? (
        hasEndpoints ? (
          <div className="mt-2 min-h-0 flex-1">
            <FileTreeView
              treeId={`tree-${domain.id}`}
              nodes={nodes}
              selectedEndpointId={selectedEndpointId}
              onSelectEndpoint={onSelectEndpoint}
            />
          </div>
        ) : (
          <p className="px-4 py-6 text-center text-xs text-muted">No endpoints loaded yet — run Enumerate, then Fetch.</p>
        )
      ) : null}
    </section>
  );
}
