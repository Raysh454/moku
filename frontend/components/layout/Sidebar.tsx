import { useState, type FormEvent } from "react";
import { EndpointExplorer } from "../explorer/EndpointExplorer";
import { useProject } from "../../context/ProjectContext";
import { Button, Field, IconButton, Input, Modal } from "../ui";
import { ChevronsLeft, Plus, RefreshCw } from "../ui/icons";

const SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export const Sidebar = () => {
  const { activeProject, createWebsiteForActiveProject, refreshActiveProject, isBusy } = useProject();
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [showModal, setShowModal] = useState(false);
  const [slug, setSlug] = useState("");
  const [origin, setOrigin] = useState("");
  const [formError, setFormError] = useState("");

  const closeModal = () => {
    setShowModal(false);
    setSlug("");
    setOrigin("");
    setFormError("");
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const nextSlug = slug.trim();
    const nextOrigin = origin.trim();

    if (!nextSlug) {
      setFormError("Slug is required");
      return;
    }
    if (!SLUG_PATTERN.test(nextSlug)) {
      setFormError("Slug must use lowercase letters, numbers, and hyphens only");
      return;
    }
    try {
      const parsed = new URL(nextOrigin);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
        setFormError("Origin must start with http:// or https://");
        return;
      }
    } catch {
      setFormError("Origin must be a valid URL");
      return;
    }

    try {
      await createWebsiteForActiveProject({ slug: nextSlug, origin: nextOrigin });
      closeModal();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to create website");
    }
  };

  return (
    <aside
      className={`flex h-full flex-shrink-0 flex-col border-r border-border bg-card transition-all duration-300 ${isCollapsed ? "w-14" : "w-72"}`}
    >
      <div
        className={`flex items-center border-b border-border bg-card/40 px-2 py-2 ${isCollapsed ? "justify-center" : "justify-between"}`}
      >
        {!isCollapsed ? (
          <span className="pl-2 text-xs font-semibold uppercase tracking-wider text-helper">Explorer</span>
        ) : null}
        <div className="flex items-center gap-0.5">
          {!isCollapsed ? (
            <>
              <IconButton
                size="sm"
                icon={Plus}
                label="Add website"
                onClick={() => setShowModal(true)}
                disabled={isBusy || !activeProject}
              />
              <IconButton
                size="sm"
                icon={RefreshCw}
                label="Refresh explorer"
                onClick={() => void refreshActiveProject()}
                disabled={isBusy || !activeProject}
              />
            </>
          ) : null}
          <IconButton
            size="sm"
            icon={ChevronsLeft}
            label={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
            onClick={() => setIsCollapsed((value) => !value)}
            className={isCollapsed ? "rotate-180" : ""}
          />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-hidden">
        <EndpointExplorer isCollapsed={isCollapsed} />
      </div>

      <Modal open={showModal} onClose={closeModal} title="Add website" subtitle="Create a website target for this project." size="sm">
        <form onSubmit={(event) => void submit(event)} className="space-y-4">
          <Field label="Slug">
            <Input value={slug} onChange={(event) => setSlug(event.target.value)} placeholder="main-site" />
          </Field>
          <Field label="Origin">
            <Input value={origin} onChange={(event) => setOrigin(event.target.value)} placeholder="https://example.com" />
          </Field>
          {formError ? <p className="text-xs text-danger">{formError}</p> : null}
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="secondary" onClick={closeModal} disabled={isBusy}>
              Cancel
            </Button>
            <Button type="submit" disabled={isBusy || !activeProject}>
              {isBusy ? "Creating…" : "Create website"}
            </Button>
          </div>
        </form>
      </Modal>
    </aside>
  );
};
