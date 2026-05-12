import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { api, createEnumerateSocket, createFetchSocket } from "../src/api/client";
import type { Endpoint as ApiEndpoint, Job, JobEvent, Version, SecurityDiffOverviewEntry } from "../src/api/types";
import type {
  Domain,
  Endpoint,
  EnumerateRequest,
  FetchRequest,
  Project,
  Snapshot,
} from "../types/project";
import { projectService } from "../services/projectService";

interface ProjectContextType {
  projects: Project[];
  activeProject: Project | null;
  selectedDomain: Domain | null;
  selectedEndpoint: Endpoint | null;
  selectedSnapshot: Snapshot | null;
  jobs: Job[];
  isLoading: boolean;
  isBusy: boolean;
  errorMessage: string;
  noticeMessage: string;
  settingsOpen: boolean;
  compareBaseVersionId: string;
  compareHeadVersionId: string;
  compareSecurityOverview: SecurityDiffOverviewEntry[] | null;
  compareIsLoading: boolean;
  domainOverviews: Map<string, SecurityDiffOverviewEntry[]>;

  refreshProjects: () => Promise<void>;
  refreshActiveProject: () => Promise<void>;
  refreshJobs: () => Promise<void>;
  setActiveProjectById: (id: string) => Promise<void>;
  setSelectedDomain: (domain: Domain | null) => void;
  setSelectedEndpoint: (endpoint: Endpoint | null) => void;
  setSelectedSnapshot: (snapshot: Snapshot | null) => void;
  selectEndpoint: (domainId: string, endpointId: string) => Promise<void>;
  selectSnapshotVersion: (versionId: string) => Promise<void>;
  createNewProject: (data: Partial<Project>) => Promise<Project>;
  createWebsiteForActiveProject: (payload: { slug: string; origin: string }) => Promise<void>;
  loadDomainEndpoints: (domainId: string, status: string, limit: number) => Promise<void>;
  addEndpointsForDomain: (domainId: string, urls: string[], source?: string) => Promise<number>;
  runEnumerateForDomain: (domainId: string, request: EnumerateRequest) => Promise<void>;
  runFetchForDomain: (domainId: string, request: FetchRequest) => Promise<void>;
  setCompareVersions: (baseVersionId: string, headVersionId: string) => Promise<void>;
  clearMessage: () => void;
  openSettings: () => void;
  closeSettings: () => void;
}

const ProjectContext = createContext<ProjectContextType | undefined>(undefined);
const ACTIVE_PROJECT_STORAGE_KEY = "moku.activeProjectId";

const setEndpointInProject = (
  project: Project,
  domainId: string,
  endpointId: string,
  updater: (endpoint: Endpoint) => Endpoint,
): Project => ({
  ...project,
  domains: project.domains.map((domain) =>
    domain.id !== domainId
      ? domain
      : {
          ...domain,
          endpoints: domain.endpoints.map((endpoint) =>
            endpoint.id === endpointId ? updater(endpoint) : endpoint,
          ),
        },
  ),
});

const createSnapshotStubs = (endpoint: Endpoint, versions: Version[]): Endpoint["snapshots"] =>
  versions.map((version, index) => ({
    id: `${endpoint.id}:${version.id}`,
    versionId: version.id,
    version: versions.length - index,
    versionLabel: version.id,
    statusCode: 0,
    url: endpoint.url,
    body: "",
    headers: {},
    createdAt: version.timestamp,
    metadata: {
      contentLength: 0,
      loadTime: 0,
    },
  }));

const toProjectEndpoint = (raw: ApiEndpoint, versions: Version[]): Endpoint => {
  const endpoint: Endpoint = {
    id: raw.id,
    url: raw.url,
    canonicalUrl: raw.canonical_url,
    path: raw.path,
    status: raw.status,
    source: raw.source,
    meta: raw.meta,
    lastFetchedVersion: raw.last_fetched_version,
    snapshots: [],
  };
  endpoint.snapshots = createSnapshotStubs(endpoint, versions);
  return endpoint;
};

export const ProjectProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProject, setActiveProject] = useState<Project | null>(null);
  const [selectedDomain, setSelectedDomainState] = useState<Domain | null>(null);
  const [selectedEndpoint, setSelectedEndpointState] = useState<Endpoint | null>(null);
  const [selectedSnapshot, setSelectedSnapshotState] = useState<Snapshot | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isBusy, setIsBusy] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [noticeMessage, setNoticeMessage] = useState("");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [compareBaseVersionId, setCompareBaseVersionId] = useState("");
  const [compareHeadVersionId, setCompareHeadVersionId] = useState("");
  const [compareSecurityOverview, setCompareSecurityOverview] = useState<SecurityDiffOverviewEntry[] | null>(null);
  const [compareIsLoading, setCompareIsLoading] = useState(false);
  const [domainOverviews, setDomainOverviews] = useState<Map<string, SecurityDiffOverviewEntry[]>>(new Map());

  const overviewCacheRef = React.useRef<
    Map<string, { overview: SecurityDiffOverviewEntry[]; timestamp: number }>
  >(new Map());

  const clearMessage = useCallback(() => {
    setErrorMessage("");
    setNoticeMessage("");
  }, []);

  const setError = useCallback((message: string) => {
    setErrorMessage(message);
    setNoticeMessage("");
  }, []);

  const setNotice = useCallback((message: string) => {
    setNoticeMessage(message);
    setErrorMessage("");
  }, []);

  const refreshJobs = useCallback(async () => {
    try {
      setJobs(await api.listJobs());
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to refresh jobs");
    }
  }, [setError]);

  const fetchDomainOverviews = useCallback(async (project: Project) => {
    if (!project.domains.length) return;
    
    const newOverviews = new Map<string, SecurityDiffOverviewEntry[]>();
    
    for (const domain of project.domains) {
      if (!domain.versions.length) continue;
      
      const headVersionId = domain.versions[0].id;
      const baseVersionId = domain.versions[1]?.id || "";
      
      if (!headVersionId) continue;
      
      try {
        const overview = await api.getSecurityOverview(
          project.slug,
          domain.slug,
          baseVersionId,
          headVersionId,
        );
        newOverviews.set(domain.slug, overview.entries || []);
      } catch (error) {
        // Silent fail - overview just won't be available for this domain
      }
    }

    setDomainOverviews(newOverviews);
  }, []);

  const refreshProjects = useCallback(async () => {
    setIsLoading(true);
    clearMessage();
    try {
      const data = await projectService.getProjects();
      setProjects(data);
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to refresh projects");
    } finally {
      setIsLoading(false);
    }
  }, [clearMessage, setError]);

  const refreshActiveProject = useCallback(async () => {
    if (!activeProject) return;
    setIsBusy(true);
    clearMessage();
    try {
      const domains = await projectService.refreshProjectDomains(activeProject);
      const refreshedProject: Project = { ...activeProject, domains };
      setActiveProject(refreshedProject);
      setProjects((prev) => prev.map((project) => (project.id === refreshedProject.id ? refreshedProject : project)));

      // Fetch security overviews for all domains
      await fetchDomainOverviews(refreshedProject);

      const nextDomain = domains.find((domain) => domain.id === selectedDomain?.id) || domains[0] || null;
      setSelectedDomainState(nextDomain);

      const nextEndpoint =
        nextDomain?.endpoints.find((endpoint) => endpoint.id === selectedEndpoint?.id) ||
        nextDomain?.endpoints[0] ||
        null;
      setSelectedEndpointState(nextEndpoint);
      setSelectedSnapshotState(null);
    } catch (error) {
      setError(error instanceof Error ? error.message : "Failed to refresh workspace data");
    } finally {
      setIsBusy(false);
    }
  }, [activeProject, clearMessage, selectedDomain?.id, selectedEndpoint?.id, setError]);

  const loadLatestSnapshotForSelection = useCallback(
    async (project: Project, domain: Domain, endpoint: Endpoint) => {
      const latestSnapshot = await projectService.loadLatestSnapshot(
        project.slug,
        domain.slug,
        endpoint,
        domain.versions,
      );

      const withLoadedSnapshot = setEndpointInProject(project, domain.id, endpoint.id, (currentEndpoint) => {
        const existing = currentEndpoint.snapshots.filter((snapshot) => snapshot.versionId !== latestSnapshot.versionId);
        return {
          ...currentEndpoint,
          snapshots: [...existing, latestSnapshot].sort((left, right) => left.version - right.version),
        };
      });

      setActiveProject(withLoadedSnapshot);
      setProjects((prev) => prev.map((item) => (item.id === withLoadedSnapshot.id ? withLoadedSnapshot : item)));

      const loadedDomain = withLoadedSnapshot.domains.find((item) => item.id === domain.id) || null;
      const loadedEndpoint = loadedDomain?.endpoints.find((item) => item.id === endpoint.id) || null;

      setSelectedDomainState(loadedDomain);
      setSelectedEndpointState(loadedEndpoint);
      setSelectedSnapshotState(latestSnapshot);
    },
    [],
  );

  const setActiveProjectById = useCallback(
    async (id: string) => {
      setIsLoading(true);
      clearMessage();
      try {
        const project = await projectService.getProjectById(id);
        if (!project) {
          window.localStorage.removeItem(ACTIVE_PROJECT_STORAGE_KEY);
          setError("Project not found");
          return;
        }

        window.localStorage.setItem(ACTIVE_PROJECT_STORAGE_KEY, project.id);
        setActiveProject(project);
        setProjects((prev) => prev.map((item) => (item.id === project.id ? project : item)));

        const firstDomain = project.domains[0] || null;
        const firstEndpoint = firstDomain?.endpoints[0] || null;
        setSelectedDomainState(firstDomain);
        setSelectedEndpointState(firstEndpoint);
        setSelectedSnapshotState(null);

        if (firstDomain) {
          void fetchDomainOverviews(project);
        }

        if (firstDomain && firstEndpoint) {
          await loadLatestSnapshotForSelection(project, firstDomain, firstEndpoint);
        }
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to load project");
      } finally {
        setIsLoading(false);
      }
    },
    [clearMessage, loadLatestSnapshotForSelection, setError],
  );

  const selectEndpoint = useCallback(
    async (domainId: string, endpointId: string) => {
      if (!activeProject) return;
      const domain = activeProject.domains.find((item) => item.id === domainId);
      const endpoint = domain?.endpoints.find((item) => item.id === endpointId);
      if (!domain || !endpoint) return;

      setIsBusy(true);
      clearMessage();
      try {
        await loadLatestSnapshotForSelection(activeProject, domain, endpoint);
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to load endpoint details");
      } finally {
        setIsBusy(false);
      }
    },
    [activeProject, clearMessage, loadLatestSnapshotForSelection, setError],
  );

  const selectSnapshotVersion = useCallback(
    async (versionId: string) => {
      if (!activeProject || !selectedDomain || !selectedEndpoint || !versionId) return;
      setIsBusy(true);
      clearMessage();
      try {
        const snapshot = await projectService.loadSnapshotByVersion(
          activeProject.slug,
          selectedDomain.slug,
          selectedEndpoint,
          versionId,
          selectedDomain.versions,
        );

        const updatedProject = setEndpointInProject(
          activeProject,
          selectedDomain.id,
          selectedEndpoint.id,
          (endpoint) => ({
            ...endpoint,
            snapshots: [
              ...endpoint.snapshots.filter((item) => item.versionId !== snapshot.versionId),
              snapshot,
            ].sort((left, right) => left.version - right.version),
          }),
        );
        setActiveProject(updatedProject);
        setProjects((prev) => prev.map((item) => (item.id === updatedProject.id ? updatedProject : item)));

        const domain = updatedProject.domains.find((item) => item.id === selectedDomain.id) || null;
        const endpoint = domain?.endpoints.find((item) => item.id === selectedEndpoint.id) || null;
        setSelectedDomainState(domain);
        setSelectedEndpointState(endpoint);
        setSelectedSnapshotState(snapshot);
      } catch (error) {
        setError(error instanceof Error ? error.message : `Failed to load version ${versionId}`);
      } finally {
        setIsBusy(false);
      }
    },
    [activeProject, clearMessage, selectedDomain, selectedEndpoint, setError],
  );

  const createNewProject = useCallback(
    async (data: Partial<Project>) => {
      setIsLoading(true);
      clearMessage();
      try {
        const created = await projectService.createProject(data);
        setProjects((prev) => [created, ...prev]);
        setNotice(`Created project "${created.name}"`);
        return created;
      } finally {
        setIsLoading(false);
      }
    },
    [clearMessage, setNotice],
  );

  const createWebsiteForActiveProject = useCallback(
    async (payload: { slug: string; origin: string }) => {
      if (!activeProject) return;
      setIsBusy(true);
      clearMessage();
      try {
        const domain = await projectService.createWebsite(activeProject.slug, payload);
        const updatedProject = { ...activeProject, domains: [...activeProject.domains, domain] };
        setActiveProject(updatedProject);
        setProjects((prev) => prev.map((item) => (item.id === updatedProject.id ? updatedProject : item)));
        setSelectedDomainState(domain);
        setSelectedEndpointState(domain.endpoints[0] || null);
        setSelectedSnapshotState(null);
        setNotice(`Added website "${domain.hostname}"`);
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to create website");
      } finally {
        setIsBusy(false);
      }
    },
    [activeProject, clearMessage, setError, setNotice],
  );

  const loadDomainEndpoints = useCallback(
    async (domainId: string, status: string, limit: number) => {
      if (!activeProject) return;
      const domain = activeProject.domains.find((item) => item.id === domainId);
      if (!domain) return;

      setIsBusy(true);
      clearMessage();
      try {
        const endpointsRaw = await api.listEndpoints(activeProject.slug, domain.slug, status, limit);
        const endpoints = endpointsRaw.map((endpoint) => toProjectEndpoint(endpoint, domain.versions));
        const updatedProject: Project = {
          ...activeProject,
          domains: activeProject.domains.map((item) => (item.id === domain.id ? { ...item, endpoints } : item)),
        };
        setActiveProject(updatedProject);
        setProjects((prev) => prev.map((item) => (item.id === updatedProject.id ? updatedProject : item)));

        const nextDomain = updatedProject.domains.find((item) => item.id === domain.id) || null;
        if (selectedDomain?.id === domain.id) {
          const nextEndpoint =
            endpoints.find((endpoint) => endpoint.id === selectedEndpoint?.id) || endpoints[0] || null;
          setSelectedDomainState(nextDomain);
          setSelectedEndpointState(nextEndpoint);
          setSelectedSnapshotState(null);
        }
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to load endpoints");
      } finally {
        setIsBusy(false);
      }
    },
    [activeProject, clearMessage, selectedDomain?.id, selectedEndpoint?.id, setError],
  );

  const addEndpointsForDomain = useCallback(
    async (domainId: string, urls: string[], source = "manual") => {
      if (!activeProject) return 0;
      const domain = activeProject.domains.find((item) => item.id === domainId);
      if (!domain) return 0;

      setIsBusy(true);
      clearMessage();
      try {
        const result = await api.addEndpoints(activeProject.slug, domain.slug, { urls, source });
        setNotice(`${result.added} endpoint${result.added === 1 ? "" : "s"} added`);
        return result.added;
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to add endpoints");
        return 0;
      } finally {
        setIsBusy(false);
      }
    },
    [activeProject, clearMessage, setError, setNotice],
  );

  const runWebSocketJob = useCallback(
    async (
      setup: () => {
        socket: WebSocket;
        onMessage: (handler: (payload: Job | JobEvent | { error: string }) => void) => void;
        send: () => void;
      },
      successMessage: string,
    ) =>
      new Promise<void>((resolve, reject) => {
        const { socket, onMessage, send } = setup();

        socket.onerror = () => reject(new Error("WebSocket connection failed"));
        socket.onopen = () => send();

        onMessage((payload) => {
          if ("error" in payload && payload.error) {
            socket.close();
            reject(new Error(payload.error));
            return;
          }

          if ("type" in payload) {
            const event = payload as JobEvent;
            if (event.status === "done" || event.type === "result") {
              socket.close();
              resolve();
            } else if (event.status === "failed" || event.status === "canceled") {
              socket.close();
              reject(new Error(event.error || "Job failed"));
            }
            return;
          }

          const job = payload as Job;
          setJobs((prev) => [job, ...prev.filter((item) => item.id !== job.id)]);
        });
      }).then(() => {
        setNotice(successMessage);
      }),
    [setNotice],
  );

  const runEnumerateForDomain = useCallback(
    async (domainId: string, request: EnumerateRequest) => {
      if (!activeProject) return;
      const domain = activeProject.domains.find((item) => item.id === domainId);
      if (!domain) return;

      setIsBusy(true);
      clearMessage();
      try {
        if (request.mode === "ws") {
          await runWebSocketJob(
            () => {
              const { socket, onMessage, sendConfig } = createEnumerateSocket(
                activeProject.slug,
                domain.slug,
                request.config,
              );
              return { socket, onMessage, send: sendConfig };
            },
            "Enumeration job completed",
          );
        } else {
          const started = await api.startEnumerate(activeProject.slug, domain.slug, request.config);
          setJobs((prev) => [started, ...prev.filter((item) => item.id !== started.id)]);
          setNotice("Enumeration job started. Use Job Queue refresh to check updates.");
          return;
        }

        await refreshActiveProject();
        await refreshJobs();
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to start enumeration");
      } finally {
        setIsBusy(false);
      }
    },
    [
      activeProject,
      clearMessage,
      refreshActiveProject,
      refreshJobs,
      runWebSocketJob,
      setError,
      setNotice,
    ],
  );

  const runFetchForDomain = useCallback(
    async (domainId: string, request: FetchRequest) => {
      if (!activeProject) return;
      const domain = activeProject.domains.find((item) => item.id === domainId);
      if (!domain) return;

      setIsBusy(true);
      clearMessage();
      try {
        if (request.mode === "ws") {
          await runWebSocketJob(
            () => {
              const { socket, onMessage, sendRequest } = createFetchSocket(activeProject.slug, domain.slug, {
                status: request.status,
                limit: request.limit,
                config: request.config,
              });
              return { socket, onMessage, send: sendRequest };
            },
            "Fetch job completed",
          );
        } else {
          const started = await api.startFetch(activeProject.slug, domain.slug, {
            status: request.status,
            limit: request.limit,
            config: request.config,
          });
          setJobs((prev) => [started, ...prev.filter((item) => item.id !== started.id)]);
          setNotice("Fetch job started. Use Job Queue refresh to check updates.");
          return;
        }

        await refreshActiveProject();
        await refreshJobs();
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to start fetch");
      } finally {
        setIsBusy(false);
      }
    },
    [
      activeProject,
      clearMessage,
      refreshActiveProject,
      refreshJobs,
      runWebSocketJob,
      setError,
      setNotice,
    ],
  );

  const setCompareVersions = useCallback(
    async (baseVersionId: string, headVersionId: string) => {
      if (!activeProject || !selectedDomain) {
        return;
      }

      setCompareBaseVersionId(baseVersionId);
      setCompareHeadVersionId(headVersionId);

      if (!headVersionId) {
        setCompareSecurityOverview(null);
        setCompareIsLoading(false);
        return;
      }

      setCompareIsLoading(true);

      const cacheKey = `${selectedDomain.slug}:${baseVersionId}:${headVersionId}`;
      const cached = overviewCacheRef.current.get(cacheKey);
      if (cached) {
        setCompareSecurityOverview(cached.overview);
        setCompareIsLoading(false);
        return;
      }

      try {
        const overview = await api.getSecurityOverview(
          activeProject.slug,
          selectedDomain.slug,
          baseVersionId || "",
          headVersionId,
        );
        overviewCacheRef.current.set(cacheKey, { overview, timestamp: Date.now() });
        setCompareSecurityOverview(overview);
      } catch (error) {
        setError(error instanceof Error ? error.message : "Failed to fetch security overview");
        setCompareSecurityOverview(null);
      } finally {
        setCompareIsLoading(false);
      }
    },
    [activeProject, selectedDomain, setError, overviewCacheRef],
  );

  useEffect(() => {
    let cancelled = false;
    const initialize = async () => {
      setIsLoading(true);
      clearMessage();
      try {
        const [projectsData, jobsData] = await Promise.all([projectService.getProjects(), api.listJobs()]);
        if (cancelled) return;
        setProjects(projectsData);
        setJobs(jobsData);

        const shouldRestoreWorkspace = window.location.hash.startsWith("#/workspace");
        const storedProjectId = window.localStorage.getItem(ACTIVE_PROJECT_STORAGE_KEY);
        if (!shouldRestoreWorkspace || !storedProjectId) return;

        const restoredProject = await projectService.getProjectById(storedProjectId);
        if (cancelled) return;

        if (!restoredProject) {
          window.localStorage.removeItem(ACTIVE_PROJECT_STORAGE_KEY);
          return;
        }

        setActiveProject(restoredProject);
        setProjects((prev) => prev.map((item) => (item.id === restoredProject.id ? restoredProject : item)));

        const firstDomain = restoredProject.domains[0] || null;
        const firstEndpoint = firstDomain?.endpoints[0] || null;
        setSelectedDomainState(firstDomain);
        setSelectedEndpointState(firstEndpoint);
        setSelectedSnapshotState(null);

        if (firstDomain) {
          void fetchDomainOverviews(restoredProject);
        }

        if (firstDomain && firstEndpoint) {
          await loadLatestSnapshotForSelection(restoredProject, firstDomain, firstEndpoint);
        }
      } catch (error) {
        if (!cancelled) {
          setError(error instanceof Error ? error.message : "Failed to initialize workspace data");
        }
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    };

    void initialize();
    return () => {
      cancelled = true;
    };
  }, [clearMessage, loadLatestSnapshotForSelection, setError]);

  const value = useMemo<ProjectContextType>(
    () => ({
      projects,
      activeProject,
      selectedDomain,
      selectedEndpoint,
      selectedSnapshot,
      jobs,
      isLoading,
      isBusy,
      errorMessage,
      noticeMessage,
      settingsOpen,
      compareBaseVersionId,
      compareHeadVersionId,
      compareSecurityOverview,
      compareIsLoading,
      refreshProjects,
      refreshActiveProject,
      refreshJobs,
      setActiveProjectById,
      setSelectedDomain: setSelectedDomainState,
      setSelectedEndpoint: setSelectedEndpointState,
      setSelectedSnapshot: setSelectedSnapshotState,
      selectEndpoint,
      selectSnapshotVersion,
      createNewProject,
      createWebsiteForActiveProject,
      loadDomainEndpoints,
      addEndpointsForDomain,
      runEnumerateForDomain,
      runFetchForDomain,
      setCompareVersions,
      clearMessage,
      domainOverviews,
      openSettings: () => setSettingsOpen(true),
      closeSettings: () => setSettingsOpen(false),
    }),
    [
      projects,
      activeProject,
      selectedDomain,
      selectedEndpoint,
      selectedSnapshot,
      jobs,
      isLoading,
      isBusy,
      errorMessage,
      noticeMessage,
      settingsOpen,
      compareBaseVersionId,
      compareHeadVersionId,
      compareSecurityOverview,
      compareIsLoading,
      refreshProjects,
      refreshActiveProject,
      refreshJobs,
      setActiveProjectById,
      selectEndpoint,
      selectSnapshotVersion,
      createNewProject,
      createWebsiteForActiveProject,
      loadDomainEndpoints,
      addEndpointsForDomain,
      runEnumerateForDomain,
      runFetchForDomain,
      setCompareVersions,
      clearMessage,
      domainOverviews,
    ],
  );

  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>;
};

export const useProject = () => {
  const context = useContext(ProjectContext);
  if (!context) {
    throw new Error("useProject must be used within a ProjectProvider");
  }
  return context;
};
