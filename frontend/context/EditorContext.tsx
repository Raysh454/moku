import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useProject } from "./ProjectContext";
import { useEndpointComparison, type VersionOption } from "../hooks/useEndpointComparison";
import { nextActiveAfterClose, openTab } from "../lib/editorTabs";
import type { Domain, Endpoint, Snapshot } from "../types/project";

/** A VS Code-style open editor tab. One tab per endpoint (id = endpointId). */
export interface OpenEditor {
  id: string;
  domainId: string;
  endpointId: string;
  label: string;
}

interface EditorCompare {
  baseVersionId: string;
  headVersionId: string;
}

export const DEFAULT_VIEW = "preview";

interface EditorContextType {
  openEditors: OpenEditor[];
  activeEditorId: string | null;
  activeDomain: Domain | null;
  activeEndpoint: Endpoint | null;
  openEndpoint: (domainId: string, endpointId: string) => void;
  closeEditor: (id: string) => void;
  setActiveEditor: (id: string) => void;
  baseVersionId: string;
  headVersionId: string;
  setCompare: (baseVersionId: string, headVersionId: string) => void;
  swapCompare: () => void;
  viewMode: string;
  setViewMode: (mode: string) => void;
  versions: VersionOption[];
  baseSnapshot: Snapshot | null;
  headSnapshot: Snapshot | null;
  loading: boolean;
  error: string;
  refreshVersions: () => Promise<void>;
}

const EditorContext = createContext<EditorContextType | undefined>(undefined);

const labelFor = (endpoint: Endpoint): string => endpoint.path || "/";

export const EditorProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { activeProject } = useProject();
  const [openEditors, setOpenEditors] = useState<OpenEditor[]>([]);
  const [activeEditorId, setActiveEditorId] = useState<string | null>(null);
  const [compareById, setCompareById] = useState<Record<string, EditorCompare>>({});
  const [viewById, setViewById] = useState<Record<string, string>>({});
  const autoOpenedProjectRef = useRef<string | null>(null);

  const openEndpoint = useCallback(
    (domainId: string, endpointId: string) => {
      const domain = activeProject?.domains.find((item) => item.id === domainId);
      const endpoint = domain?.endpoints.find((item) => item.id === endpointId);
      if (!domain || !endpoint) return;

      setOpenEditors((prev) => openTab(prev, { id: endpointId, domainId, endpointId, label: labelFor(endpoint) }));
      setCompareById((prev) =>
        prev[endpointId]
          ? prev
          : {
              ...prev,
              [endpointId]: { baseVersionId: domain.versions[1]?.id ?? "", headVersionId: domain.versions[0]?.id ?? "" },
            },
      );
      setViewById((prev) => (prev[endpointId] ? prev : { ...prev, [endpointId]: DEFAULT_VIEW }));
      setActiveEditorId(endpointId);
    },
    [activeProject],
  );

  const closeEditor = useCallback((id: string) => {
    setOpenEditors((prev) => {
      setActiveEditorId((current) => nextActiveAfterClose(prev, id, current));
      return prev.filter((editor) => editor.id !== id);
    });
  }, []);

  const setActiveEditor = useCallback((id: string) => setActiveEditorId(id), []);

  // Reset the tab set when the active project changes.
  useEffect(() => {
    setOpenEditors([]);
    setActiveEditorId(null);
    setCompareById({});
    setViewById({});
    autoOpenedProjectRef.current = null;
  }, [activeProject?.id]);

  // Auto-open the first endpoint once per project, when endpoints are available.
  useEffect(() => {
    const projectId = activeProject?.id ?? null;
    const firstDomain = activeProject?.domains[0];
    const firstEndpoint = firstDomain?.endpoints[0];
    if (!projectId || !firstDomain || !firstEndpoint) return;
    if (autoOpenedProjectRef.current === projectId) return;
    autoOpenedProjectRef.current = projectId;
    if (openEditors.length === 0) openEndpoint(firstDomain.id, firstEndpoint.id);
  }, [activeProject?.id, activeProject?.domains, openEditors.length, openEndpoint]);

  const activeEditor = openEditors.find((editor) => editor.id === activeEditorId) ?? null;
  const activeDomain = activeProject?.domains.find((item) => item.id === activeEditor?.domainId) ?? null;
  const activeEndpoint = activeDomain?.endpoints.find((item) => item.id === activeEditor?.endpointId) ?? null;
  const activeCompare = (activeEditorId && compareById[activeEditorId]) || { baseVersionId: "", headVersionId: "" };

  const comparison = useEndpointComparison(
    activeProject,
    activeDomain,
    activeEndpoint,
    activeCompare.baseVersionId,
    activeCompare.headVersionId,
  );

  // Fill in default base/head once versions for the active editor are known.
  useEffect(() => {
    if (!activeEditorId) return;
    if (compareById[activeEditorId]?.headVersionId) return;
    if (comparison.versions.length === 0) return;
    setCompareById((prev) => ({
      ...prev,
      [activeEditorId]: {
        baseVersionId: comparison.versions[1]?.versionId ?? "",
        headVersionId: comparison.versions[0]?.versionId ?? "",
      },
    }));
  }, [activeEditorId, compareById, comparison.versions]);

  const setCompare = useCallback(
    (baseVersionId: string, headVersionId: string) => {
      if (!activeEditorId) return;
      setCompareById((prev) => ({ ...prev, [activeEditorId]: { baseVersionId, headVersionId } }));
    },
    [activeEditorId],
  );

  const swapCompare = useCallback(() => {
    if (!activeEditorId) return;
    setCompareById((prev) => {
      const current = prev[activeEditorId];
      if (!current) return prev;
      return { ...prev, [activeEditorId]: { baseVersionId: current.headVersionId, headVersionId: current.baseVersionId } };
    });
  }, [activeEditorId]);

  const setViewMode = useCallback(
    (mode: string) => {
      if (!activeEditorId) return;
      setViewById((prev) => ({ ...prev, [activeEditorId]: mode }));
    },
    [activeEditorId],
  );

  const value = useMemo<EditorContextType>(
    () => ({
      openEditors,
      activeEditorId,
      activeDomain,
      activeEndpoint,
      openEndpoint,
      closeEditor,
      setActiveEditor,
      baseVersionId: activeCompare.baseVersionId,
      headVersionId: activeCompare.headVersionId,
      setCompare,
      swapCompare,
      viewMode: (activeEditorId && viewById[activeEditorId]) || DEFAULT_VIEW,
      setViewMode,
      versions: comparison.versions,
      baseSnapshot: comparison.baseSnapshot,
      headSnapshot: comparison.headSnapshot,
      loading: comparison.loading,
      error: comparison.error,
      refreshVersions: comparison.refreshVersions,
    }),
    [
      openEditors,
      activeEditorId,
      activeDomain,
      activeEndpoint,
      openEndpoint,
      closeEditor,
      setActiveEditor,
      activeCompare.baseVersionId,
      activeCompare.headVersionId,
      setCompare,
      swapCompare,
      viewById,
      setViewMode,
      comparison,
    ],
  );

  return <EditorContext.Provider value={value}>{children}</EditorContext.Provider>;
};

export const useEditor = (): EditorContextType => {
  const context = useContext(EditorContext);
  if (!context) throw new Error("useEditor must be used within an EditorProvider");
  return context;
};
