import { useState } from "react";
import type { Domain } from "../../types/project";
import { useProject } from "../../context/ProjectContext";
import { useNotifications } from "../../context/NotificationContext";
import { Button, Checkbox, Field, Input } from "../ui";
import { buildEnumerationConfig, defaultEnumerateState, type EnumerateFormState } from "./domainActions";

interface DomainFormProps {
  domain: Domain;
  onClose: () => void;
}

export function EnumerateForm({ domain, onClose }: DomainFormProps) {
  const { runEnumerateForDomain, isBusy } = useProject();
  const { notify } = useNotifications();
  const [state, setState] = useState<EnumerateFormState>(defaultEnumerateState);

  const patch = (next: Partial<EnumerateFormState>) => setState((current) => ({ ...current, ...next }));

  const submit = async () => {
    notify({ kind: "info", title: `Starting enumeration for ${domain.hostname}`, message: "Job queued" });
    await runEnumerateForDomain(domain.id, { config: buildEnumerationConfig(state) });
    onClose();
  };

  return (
    <div className="space-y-4">
      <Checkbox label="Spider" checked={state.spiderEnabled} onChange={(event) => patch({ spiderEnabled: event.target.checked })} />
      <div className="grid grid-cols-2 gap-3">
        <Field label="Spider depth">
          <Input
            type="number"
            min={1}
            max={20}
            value={state.spiderDepth}
            disabled={!state.spiderEnabled}
            onChange={(event) => patch({ spiderDepth: Number(event.target.value) || 4 })}
          />
        </Field>
        <Field label="Max pages">
          <Input
            type="number"
            min={1}
            value={state.spiderMaxPages}
            disabled={!state.spiderEnabled}
            onChange={(event) => patch({ spiderMaxPages: Number(event.target.value) || 1000 })}
          />
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Checkbox label="Sitemap" checked={state.sitemapEnabled} onChange={(event) => patch({ sitemapEnabled: event.target.checked })} />
        <Checkbox label="Robots" checked={state.robotsEnabled} onChange={(event) => patch({ robotsEnabled: event.target.checked })} />
        <Checkbox label="Wayback" checked={state.waybackEnabled} onChange={(event) => patch({ waybackEnabled: event.target.checked })} />
        <Checkbox
          label="Archive.org"
          checked={state.waybackMachine}
          disabled={!state.waybackEnabled}
          onChange={(event) => patch({ waybackMachine: event.target.checked })}
        />
        <Checkbox
          label="Common Crawl"
          className="col-span-2"
          checked={state.commonCrawl}
          disabled={!state.waybackEnabled}
          onChange={(event) => patch({ commonCrawl: event.target.checked })}
        />
      </div>
      <div className="flex justify-end">
        <Button onClick={() => void submit()} disabled={isBusy}>
          Start enumeration
        </Button>
      </div>
    </div>
  );
}
