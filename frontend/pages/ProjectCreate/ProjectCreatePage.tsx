import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useProject } from "../../context/ProjectContext";
import { Button, Field, Input, Textarea } from "../../components/ui";

const ProjectCreatePage = () => {
  const navigate = useNavigate();
  const { createNewProject, setActiveProjectById } = useProject();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [domains, setDomains] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const handleCreate = async (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim()) return;

    setIsCreating(true);
    try {
      const project = await createNewProject({ name: name.trim(), description });
      await setActiveProjectById(project.id);
      navigate("/workspace");
    } catch {
      setIsCreating(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg p-4">
      <div className="w-full max-w-md">
        <div className="mb-8">
          <h1 className="mb-2 text-2xl font-semibold tracking-tight text-primary">Create project</h1>
          <p className="text-sm text-helper">Set up a new project to monitor websites for security changes.</p>
        </div>

        <form onSubmit={(event) => void handleCreate(event)} className="space-y-5">
          <Field label="Name">
            <Input required autoFocus disabled={isCreating} value={name} onChange={(event) => setName(event.target.value)} placeholder="My Project" />
          </Field>
          <Field label="Description">
            <Input disabled={isCreating} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Optional project description" />
          </Field>
          <Field
            label="Endpoints"
            hint="Bulk import is kept for later integration — add websites from the explorer once in the workspace."
          >
            <Textarea
              rows={4}
              disabled={isCreating}
              value={domains}
              onChange={(event) => setDomains(event.target.value)}
              placeholder={"https://example.com/users\nhttps://example.com/login"}
              className="custom-scrollbar"
            />
          </Field>

          <div className="flex gap-3 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => navigate("/")} disabled={isCreating}>
              Cancel
            </Button>
            <Button type="submit" className="flex-1" disabled={isCreating}>
              {isCreating ? "Creating…" : "Create project"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default ProjectCreatePage;
