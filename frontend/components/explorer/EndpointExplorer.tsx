import { useMemo } from "react";
import type { Domain } from "../../types/project";
import type { SecurityDiffOverviewEntry } from "../../src/api/types";
import { useProject } from "../../context/ProjectContext";
import { buildEndpointTree } from "../../lib/endpointTree";
import { FileTreeView } from "../../adapters/tree";
import { DomainActionsBar } from "../domain/DomainActionsBar";
import { EmptyState } from "../ui";
import { Boxes, Globe } from "../ui/icons";

/**
 * Sidebar body: one virtualized file tree per domain (endpoint URL paths
 * become folders/files), with endpoint status decorations and a compact
 * action bar. Replaces the flat list + hover mega-menu of the old DomainTree.
 */
export function EndpointExplorer({ isCollapsed = false }: { isCollapsed?: boolean }) {
  const { activeProject, selectedEndpoint, selectEndpoint, domainOverviews } = useProject();

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
          selectedEndpointId={selectedEndpoint?.id ?? null}
          overview={domainOverviews.get(domain.slug)}
          onSelectEndpoint={(endpointId) => void selectEndpoint(domain.id, endpointId)}
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
  const nodes = useMemo(() => buildEndpointTree(domain, overview), [domain, overview]);
  const hasEndpoints = domain.endpoints.length > 0;

  return (
    <section className="flex min-h-0 flex-1 flex-col border-b border-border/60 last:border-b-0">
      <div className="px-3 pt-3">
        <div className="flex items-center gap-2 px-1">
          <Globe className="h-4 w-4 shrink-0 text-helper" />
          <span className="truncate font-mono text-[13px] font-semibold text-primary">{domain.hostname}</span>
        </div>
        <div className="mt-1.5 px-1">
          <DomainActionsBar domain={domain} />
        </div>
      </div>
      {hasEndpoints ? (
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
      )}
    </section>
  );
}
