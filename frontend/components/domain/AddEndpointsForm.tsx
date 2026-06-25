import { useState } from "react";
import type { Domain } from "../../types/project";
import { useProject } from "../../context/ProjectContext";
import { useNotifications } from "../../context/NotificationContext";
import { Button, Field, Textarea } from "../ui";

interface DomainFormProps {
  domain: Domain;
  onClose: () => void;
}

export function AddEndpointsForm({ domain, onClose }: DomainFormProps) {
  const { addEndpointsForDomain, loadDomainEndpoints, isBusy } = useProject();
  const { notify } = useNotifications();
  const [value, setValue] = useState("");

  const submit = async () => {
    const urls = value
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    if (urls.length === 0) {
      notify({ kind: "warning", title: "No endpoints provided", message: "Add at least one endpoint URL per line." });
      return;
    }
    const added = await addEndpointsForDomain(domain.id, urls, "manual");
    if (added > 0) {
      await loadDomainEndpoints(domain.id, "", 0);
      notify({ kind: "success", title: `Added ${added} endpoint${added === 1 ? "" : "s"}`, message: `${domain.hostname} updated` });
      onClose();
    }
  };

  return (
    <div className="space-y-4">
      <Field label="Endpoint URLs" hint="One URL per line.">
        <Textarea
          rows={10}
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder={"https://example.com/\nhttps://example.com/login\nhttps://example.com/admin"}
          className="min-h-[200px]"
        />
      </Field>
      <div className="flex justify-end">
        <Button variant="success" onClick={() => void submit()} disabled={isBusy}>
          Add endpoints
        </Button>
      </div>
    </div>
  );
}
