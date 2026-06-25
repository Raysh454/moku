import { useState } from "react";
import type { Domain } from "../../types/project";
import { useProject } from "../../context/ProjectContext";
import { useNotifications } from "../../context/NotificationContext";
import { Button, Field, Input, Select } from "../ui";

interface DomainFormProps {
  domain: Domain;
  onClose: () => void;
}

export function LoadEndpointsForm({ domain, onClose }: DomainFormProps) {
  const { loadDomainEndpoints, isBusy } = useProject();
  const { notify } = useNotifications();
  const [status, setStatus] = useState("");
  const [limit, setLimit] = useState(0);

  const submit = async () => {
    await loadDomainEndpoints(domain.id, status, Math.max(0, limit));
    notify({
      kind: "success",
      title: `Endpoints loaded for ${domain.hostname}`,
      message: `status=${status || "non-filtered"} • limit=${limit === 0 ? "no-limit" : limit}`,
    });
    onClose();
  };

  return (
    <div className="space-y-4">
      <Field label="Endpoints to load">
        <Select value={status} onChange={(event) => setStatus(event.target.value)}>
          <option value="">All (excluding filtered)</option>
          <option value="*">All (including filtered)</option>
          <option value="new">new</option>
          <option value="pending">pending</option>
          <option value="fetched">fetched</option>
          <option value="failed">failed</option>
          <option value="filtered">filtered</option>
        </Select>
      </Field>
      <Field label="Limit" hint="Limit 0 means no limit.">
        <Input type="number" min={0} max={50000} value={limit} onChange={(event) => setLimit(Math.max(0, Number(event.target.value) || 0))} />
      </Field>
      <div className="flex justify-end">
        <Button onClick={() => void submit()} disabled={isBusy}>
          Load endpoints
        </Button>
      </div>
    </div>
  );
}
