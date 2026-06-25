import { useState } from "react";
import type { Domain } from "../../types/project";
import { useProject } from "../../context/ProjectContext";
import { IconButton, Modal, Toolbar } from "../ui";
import { Boxes, Download, ListTree, Plus, ScanLine, Trash2 } from "../ui/icons";
import { EnumerateForm } from "./EnumerateForm";
import { FetchForm } from "./FetchForm";
import { LoadEndpointsForm } from "./LoadEndpointsForm";
import { AddEndpointsForm } from "./AddEndpointsForm";
import { ScanForm } from "./ScanForm";

type ActionKey = "enumerate" | "fetch" | "load" | "add" | "scan" | null;

/**
 * Compact per-domain action bar. Replaces the 620px hover "mega-menu":
 * each action opens a focused modal form rather than a nested fly-out.
 */
export function DomainActionsBar({ domain }: { domain: Domain }) {
  const { deleteWebsite, isBusy } = useProject();
  const [open, setOpen] = useState<ActionKey>(null);
  const close = () => setOpen(null);

  const onDelete = async () => {
    if (!window.confirm(`Delete website "${domain.hostname}"? This cannot be undone.`)) return;
    await deleteWebsite(domain.slug);
  };

  return (
    <>
      <Toolbar className="gap-0.5">
        <IconButton size="sm" icon={Boxes} label="Enumerate" onClick={() => setOpen("enumerate")} disabled={isBusy} />
        <IconButton size="sm" icon={Download} label="Fetch snapshots" onClick={() => setOpen("fetch")} disabled={isBusy} />
        <IconButton size="sm" icon={ListTree} label="Load endpoints" onClick={() => setOpen("load")} disabled={isBusy} />
        <IconButton size="sm" icon={Plus} label="Add endpoints" onClick={() => setOpen("add")} disabled={isBusy} />
        <IconButton size="sm" icon={ScanLine} label="Scan" onClick={() => setOpen("scan")} disabled={isBusy} />
        <IconButton size="sm" icon={Trash2} label="Delete website" tone="danger" onClick={() => void onDelete()} disabled={isBusy} />
      </Toolbar>

      <Modal open={open === "enumerate"} onClose={close} title={`Enumerate · ${domain.hostname}`} subtitle="Discover endpoints to track">
        <EnumerateForm domain={domain} onClose={close} />
      </Modal>
      <Modal open={open === "fetch"} onClose={close} title={`Fetch · ${domain.hostname}`} subtitle="Capture a new snapshot version">
        <FetchForm domain={domain} onClose={close} />
      </Modal>
      <Modal open={open === "load"} onClose={close} title={`Endpoints · ${domain.hostname}`} subtitle="Load endpoints into the explorer">
        <LoadEndpointsForm domain={domain} onClose={close} />
      </Modal>
      <Modal open={open === "add"} onClose={close} title={`Add endpoints · ${domain.hostname}`} subtitle="Track endpoints manually">
        <AddEndpointsForm domain={domain} onClose={close} />
      </Modal>
      <Modal open={open === "scan"} onClose={close} title={`Scan · ${domain.hostname}`} subtitle="Run a vulnerability scan">
        <ScanForm domain={domain} onClose={close} />
      </Modal>
    </>
  );
}
