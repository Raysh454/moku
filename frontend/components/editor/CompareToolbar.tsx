import { useEditor } from "../../context/EditorContext";
import { IconButton, Select, StatusPill, Toolbar, scoreTone } from "../ui";
import { ArrowLeftRight, RefreshCw } from "../ui/icons";
import { formatScore } from "../../lib/score";

/** Compact base → head compare bar. Replaces the big two-dropdown card. */
export function CompareToolbar() {
  const { versions, baseVersionId, headVersionId, setCompare, swapCompare, refreshVersions, headSnapshot } = useEditor();
  const scoreDelta = headSnapshot?.securityDiff?.score_delta;
  const headMeta = versions.find((version) => version.versionId === headVersionId);

  return (
    <Toolbar className="border-b border-border bg-card/20 px-4 py-2">
      <span className="text-xs font-medium text-helper">Compare</span>

      <div className="w-40">
        <Select
          sizeVariant="sm"
          aria-label="Base version"
          value={baseVersionId}
          onChange={(event) => setCompare(event.target.value, headVersionId)}
          onMouseDown={() => void refreshVersions()}
        >
          <option value="">base…</option>
          {versions.map((version) => (
            <option key={`base-${version.versionId}`} value={version.versionId}>
              {version.label}
            </option>
          ))}
        </Select>
      </div>

      <IconButton size="sm" icon={ArrowLeftRight} label="Swap base and head" onClick={swapCompare} />

      <div className="w-40">
        <Select
          sizeVariant="sm"
          aria-label="Head version"
          value={headVersionId}
          onChange={(event) => setCompare(baseVersionId, event.target.value)}
          onMouseDown={() => void refreshVersions()}
        >
          <option value="">head…</option>
          {versions.map((version) => (
            <option key={`head-${version.versionId}`} value={version.versionId}>
              {version.label}
            </option>
          ))}
        </Select>
      </div>

      {scoreDelta !== undefined ? (
        <StatusPill tone={scoreTone(scoreDelta)} title="Security score change (base → head)">
          Δ {formatScore(scoreDelta)}
        </StatusPill>
      ) : null}

      {headMeta?.fetchedAt ? (
        <span className="text-[11px] text-muted">fetched {new Date(headMeta.fetchedAt).toLocaleString()}</span>
      ) : null}

      <IconButton size="sm" icon={RefreshCw} label="Refresh versions" onClick={() => void refreshVersions()} className="ml-auto" />
    </Toolbar>
  );
}
