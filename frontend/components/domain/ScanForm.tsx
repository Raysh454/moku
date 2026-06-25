import { useEffect, useState } from "react";
import type { Domain, ScanRequest } from "../../types/project";
import { useProject } from "../../context/ProjectContext";
import { useNotifications } from "../../context/NotificationContext";
import { api } from "../../src/api/client";
import { Button, Field, Input, Select } from "../ui";
import { SCAN_PROFILES, toScanProfile } from "./domainActions";

interface DomainFormProps {
  domain: Domain;
  onClose: () => void;
}

interface AnalyzerStatus {
  backend: string;
  health: string;
  supportsProfile: boolean;
}

export function ScanForm({ domain, onClose }: DomainFormProps) {
  const { activeProject, runScanForDomain, isBusy } = useProject();
  const { notify } = useNotifications();
  const [url, setUrl] = useState(domain.origin);
  const [profile, setProfile] = useState("");
  const [analyzer, setAnalyzer] = useState<AnalyzerStatus | null>(null);

  useEffect(() => {
    if (!activeProject) return;
    let cancelled = false;
    const probe = async () => {
      const [healthResult, capabilitiesResult] = await Promise.allSettled([
        api.getAnalyzerHealth(activeProject.slug, domain.slug),
        api.getAnalyzerCapabilities(activeProject.slug, domain.slug),
      ]);
      if (cancelled) return;
      const health = healthResult.status === "fulfilled" ? healthResult.value : undefined;
      const capabilities = capabilitiesResult.status === "fulfilled" ? capabilitiesResult.value : undefined;
      setAnalyzer({
        backend: capabilities?.backend || health?.backend || "unknown",
        health: health?.status || "unreachable",
        supportsProfile: capabilities?.capabilities?.supports_scan_profile ?? true,
      });
    };
    void probe();
    return () => {
      cancelled = true;
    };
  }, [activeProject, domain.slug]);

  const submit = async () => {
    const target = url.trim();
    if (!target) {
      notify({ kind: "warning", title: "Scan target required", message: "Provide a target URL to scan." });
      return;
    }
    const request: ScanRequest = { url: target, profile: toScanProfile(profile) };
    try {
      const started = await runScanForDomain(domain.id, request);
      notify({
        kind: "success",
        title: `Scan started for ${domain.hostname}`,
        message: `Job ${(started.id ?? "").slice(0, 8)} queued — findings appear in the Analysis view`,
      });
      onClose();
    } catch (error) {
      notify({
        kind: "error",
        title: `Failed to start scan for ${domain.hostname}`,
        message: error instanceof Error ? error.message : "Unknown error",
      });
    }
  };

  return (
    <div className="space-y-4">
      <Field label="Target URL">
        <Input className="font-mono" value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://example.com/" />
      </Field>
      <Field label="Profile">
        <Select
          value={profile}
          disabled={analyzer !== null && !analyzer.supportsProfile}
          onChange={(event) => setProfile(event.target.value)}
        >
          <option value="">backend default</option>
          {SCAN_PROFILES.map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </Select>
      </Field>
      <p className="text-[11px] text-muted">
        {analyzer ? `Analyzer: ${analyzer.backend} • ${analyzer.health}` : "Checking analyzer availability…"}
      </p>
      <div className="flex justify-end">
        <Button onClick={() => void submit()} disabled={isBusy}>
          Start scan
        </Button>
      </div>
    </div>
  );
}
