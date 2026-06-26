import { useMemo, useState } from "react";
import type { Snapshot } from "../../../types/project";
import { DiffView } from "../../../adapters/diff";
import { serializeHeaders } from "../../../lib/headerText";
import { changedSecurityHeaders, filterSecurityHeaders } from "../../../lib/headerClassification";
import { SectionHeading } from "../../ui";

interface HeaderDiffViewProps {
  headSnapshot: Snapshot;
  baseSnapshot: Snapshot | null;
}

export function HeaderDiffView({ headSnapshot, baseSnapshot }: HeaderDiffViewProps) {
  const [securityOnly, setSecurityOnly] = useState(false);
  const baseHeaders = useMemo(() => baseSnapshot?.headers ?? {}, [baseSnapshot]);
  const headHeaders = useMemo(() => headSnapshot.headers ?? {}, [headSnapshot]);

  // Always flag which security headers moved, even when the noise is shown, so a
  // CSP/HSTS change is never lost among date/etag/debug-id churn.
  const securityChanges = useMemo(
    () => changedSecurityHeaders(baseHeaders, headHeaders),
    [baseHeaders, headHeaders],
  );

  const baseText = useMemo(
    () => serializeHeaders(securityOnly ? filterSecurityHeaders(baseHeaders) : baseHeaders),
    [baseHeaders, securityOnly],
  );
  const headText = useMemo(
    () => serializeHeaders(securityOnly ? filterSecurityHeaders(headHeaders) : headHeaders),
    [headHeaders, securityOnly],
  );
  const unchanged = baseText === headText;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionHeading title="Header diff" size="sub" />
        <label
          className="inline-flex items-center gap-1.5 text-xs text-helper"
          title="Hide churn headers (date, etag, request/debug ids) and show only headers that affect security posture."
        >
          <input
            type="checkbox"
            className="accent-accent"
            checked={securityOnly}
            onChange={(event) => setSecurityOnly(event.target.checked)}
          />
          Security only
        </label>
      </div>

      {securityChanges.length > 0 ? (
        <p className="text-xs text-warning">
          {securityChanges.length} security-relevant header change{securityChanges.length === 1 ? "" : "s"}:{" "}
          <span className="font-mono text-primary">{securityChanges.join(", ")}</span>
        </p>
      ) : (
        <p className="text-xs text-muted">No security-relevant header changes.</p>
      )}

      {unchanged ? (
        <p className="text-xs text-muted">
          {securityOnly
            ? "No security header changes between these versions."
            : "No header changes between these versions."}
        </p>
      ) : (
        <DiffView base={baseText} head={headText} language="http" mode="unified" fileName="response headers" />
      )}
    </div>
  );
}
