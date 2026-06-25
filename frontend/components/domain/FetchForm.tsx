import { useState } from "react";
import type { Domain } from "../../types/project";
import { useProject } from "../../context/ProjectContext";
import { useNotifications } from "../../context/NotificationContext";
import { Button, Field, Input, Select } from "../ui";

interface DomainFormProps {
  domain: Domain;
  onClose: () => void;
}

const FETCH_STATUSES = ["*", "new", "pending", "fetched", "failed", "filtered"] as const;

export function FetchForm({ domain, onClose }: DomainFormProps) {
  const { runFetchForDomain, isBusy } = useProject();
  const { notify } = useNotifications();
  const [status, setStatus] = useState("*");
  const [limit, setLimit] = useState(0);
  const [concurrency, setConcurrency] = useState(4);

  const submit = async () => {
    notify({
      kind: "info",
      title: `Starting fetch for ${domain.hostname}`,
      message: `status=${status === "*" ? "all-non-filtered" : status} • limit=${limit === 0 ? "no-limit" : limit}`,
    });
    await runFetchForDomain(domain.id, { status, limit: Math.max(0, limit), config: { concurrency: concurrency || 4 } });
    onClose();
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-3 gap-3">
        <Field label="Status">
          <Select value={status} onChange={(event) => setStatus(event.target.value)}>
            <option value="*">all (excl. filtered)</option>
            {FETCH_STATUSES.filter((value) => value !== "*").map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Limit">
          <Input type="number" min={0} max={20000} value={limit} onChange={(event) => setLimit(Math.max(0, Number(event.target.value) || 0))} />
        </Field>
        <Field label="Concurrency">
          <Input type="number" min={1} max={100} value={concurrency} onChange={(event) => setConcurrency(Number(event.target.value) || 4)} />
        </Field>
      </div>
      <p className="text-[11px] text-muted">Limit 0 fetches all matching endpoints.</p>
      <div className="flex justify-end">
        <Button variant="success" onClick={() => void submit()} disabled={isBusy}>
          Start fetch
        </Button>
      </div>
    </div>
  );
}
