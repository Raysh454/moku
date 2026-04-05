import { useEffect, useMemo, useState } from 'react'
import { api, config, createEnumerateSocket, createFetchSocket, demoApi } from './api/client'
import type { DemoPageVersion, Endpoint, EndpointDetails, EnumerationConfig, FetchConfig, Job, JobEvent, Project, Version, Website } from './api/types'
import RenderedDiffViews, { type RenderedViewMode } from './components/RenderedDiffViews'
import FilterConfigPanel from './components/FilterConfigPanel'

type Activity = {
  at: string
  title: string
  detail?: string
}

type RawItem = {
  key: string
  value: unknown
}

const now = () => new Date().toLocaleTimeString()

export default function App() {
  const [projects, setProjects] = useState<Project[]>([])
  const [websites, setWebsites] = useState<Website[]>([])
  const [jobs, setJobs] = useState<Job[]>([])
  const [endpoints, setEndpoints] = useState<Endpoint[]>([])
  const [details, setDetails] = useState<EndpointDetails | null>(null)
  const [_demoVersions, setDemoVersions] = useState<DemoPageVersion[]>([])

  const [projectSlug, setProjectSlug] = useState('demo-ui')
  const [projectName, setProjectName] = useState('Demo UI Project')
  const [projectDescription, setProjectDescription] = useState('created from isolated react ui')

  const [siteSlug, setSiteSlug] = useState('local-demo')
  const [siteOrigin, setSiteOrigin] = useState(config.demoOrigin)

  const [selectedProject, setSelectedProject] = useState('')
  const [selectedSite, setSelectedSite] = useState('')
  const [selectedEndpointUrl, setSelectedEndpointUrl] = useState('')

  const [fetchStatus, setFetchStatus] = useState('*')
  const [fetchLimit, setFetchLimit] = useState(100)
  const [fetchConcurrency, setFetchConcurrency] = useState(4)
  const [endpointFilterStatus, setEndpointFilterStatus] = useState('')
  const [endpointFilterLimit, setEndpointFilterLimit] = useState(200)

  // Enumeration config state
  const [enumSpiderEnabled, setEnumSpiderEnabled] = useState(true)
  const [enumSpiderMaxDepth, setEnumSpiderMaxDepth] = useState(4)
  const [enumSpiderConcurrency, setEnumSpiderConcurrency] = useState(5)
  const [enumSitemapEnabled, setEnumSitemapEnabled] = useState(false)
  const [enumRobotsEnabled, setEnumRobotsEnabled] = useState(false)
  const [enumWaybackEnabled, setEnumWaybackEnabled] = useState(false)
  const [enumWaybackMachine, setEnumWaybackMachine] = useState(true)
  const [enumCommonCrawl, setEnumCommonCrawl] = useState(true)

  const [activities, setActivities] = useState<Activity[]>([])
  const [rawItems, setRawItems] = useState<RawItem[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  // Version comparison state
  const [viewMode, setViewMode] = useState<'dashboard' | 'comparison'>('dashboard')
  const [versions, setVersions] = useState<Version[]>([])
  const [selectedBaseVersion, setSelectedBaseVersion] = useState('')
  const [selectedHeadVersion, setSelectedHeadVersion] = useState('')
  const [comparisonDetails, setComparisonDetails] = useState<EndpointDetails | null>(null)
  const [comparisonLayout, setComparisonLayout] = useState<'side-by-side' | 'unified' | 'split'>('side-by-side')
  const [renderedViewMode, setRenderedViewMode] = useState<RenderedViewMode>('preview')

  // Comparison page independent state
  const [comparisonProjects, setComparisonProjects] = useState<Project[]>([])
  const [comparisonWebsites, setComparisonWebsites] = useState<Website[]>([])
  const [comparisonEndpoints, setComparisonEndpoints] = useState<Endpoint[]>([])
  const [comparisonProject, setComparisonProject] = useState('')
  const [comparisonWebsite, setComparisonWebsite] = useState('')
  const [comparisonEndpoint, setComparisonEndpoint] = useState('')

  // Base snapshot state for comparison
  const [baseSnapshot, setBaseSnapshot] = useState<EndpointDetails | null>(null)

  const logActivity = (title: string, detail?: string) => {
    setActivities((prev) => [{ at: now(), title, detail }, ...prev].slice(0, 60))
  }

  const pushRaw = (key: string, value: unknown) => {
    setRawItems((prev) => [{ key, value }, ...prev.filter((entry) => entry.key !== key)].slice(0, 30))
  }

  const withAction = async (label: string, action: () => Promise<void>) => {
    setBusy(true)
    setError('')
    try {
      logActivity(label)
      await action()
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      logActivity(`${label} failed`, message)
    } finally {
      setBusy(false)
    }
  }

  const buildEnumerationConfig = (): EnumerationConfig => {
    const cfg: EnumerationConfig = {}

    if (enumSpiderEnabled) {
      cfg.spider = {
        max_depth: enumSpiderMaxDepth,
        concurrency: enumSpiderConcurrency,
      }
    }
    if (enumSitemapEnabled) {
      cfg.sitemap = {}
    }
    if (enumRobotsEnabled) {
      cfg.robots = {}
    }
    if (enumWaybackEnabled) {
      cfg.wayback = {
        use_wayback_machine: enumWaybackMachine,
        use_common_crawl: enumCommonCrawl,
      }
    }

    return cfg
  }

  const buildFetchConfig = (): FetchConfig | undefined => {
    // Only include config if concurrency is different from default (4)
    if (fetchConcurrency !== 4) {
      return { concurrency: fetchConcurrency }
    }
    return undefined
  }

  const refreshProjects = async () => {
    const data = await api.listProjects()
    setProjects(data)
    pushRaw('projects', data)
    if (!selectedProject && data.length > 0) {
      setSelectedProject(data[0].slug)
    }
  }

  const refreshWebsites = async (project: string) => {
    if (!project) {
      setWebsites([])
      return
    }
    const data = await api.listWebsites(project)
    setWebsites(data)
    pushRaw('websites', data)
    if (!selectedSite && data.length > 0) {
      setSelectedSite(data[0].slug)
    }
  }

  const refreshJobs = async () => {
    const data = await api.listJobs()
    setJobs(data)
    pushRaw('jobs', data)
  }

  const refreshEndpoints = async () => {
    if (!selectedProject || !selectedSite) return
    const data = await api.listEndpoints(selectedProject, selectedSite, endpointFilterStatus, endpointFilterLimit)
    setEndpoints(data)
    pushRaw('endpoints', data)
    if (!selectedEndpointUrl && data.length > 0) {
      setSelectedEndpointUrl(data[0].canonical_url)
    }
  }

  const refreshDemoVersions = async () => {
    const data = await demoApi.getVersions()
    setDemoVersions(data)
    pushRaw('demoVersions', data)
  }

  const waitForJob = async (jobId: string) => {
    const started = Date.now()
    while (Date.now() - started < 180000) {
      const job = await api.getJob(jobId)
      pushRaw(`job:${jobId}`, job)
      setJobs((prev) => [job, ...prev.filter((entry) => entry.id !== job.id)])

      if (job.status === 'done') {
        logActivity(`Job ${job.type} done`, job.id)
        return job
      }
      if (job.status === 'failed' || job.status === 'canceled') {
        throw new Error(`Job ${job.status}: ${job.error || 'unknown error'}`)
      }
      await new Promise((resolve) => setTimeout(resolve, 500))
    }
    throw new Error('Timed out waiting for job completion')
  }

  const loadDetails = async (url: string) => {
    if (!selectedProject || !selectedSite || !url) return
    const data = await api.getEndpointDetails(selectedProject, selectedSite, url)
    setDetails(data)
    pushRaw('endpointDetails', data)
  }

  const loadComparisonDetails = async (url: string, baseVersionId: string, headVersionId: string) => {
    if (!comparisonProject || !comparisonWebsite || !url || !baseVersionId || !headVersionId) return
    // Load head version details with diff
    const data = await api.getEndpointDetails(comparisonProject, comparisonWebsite, url, baseVersionId, headVersionId)
    setComparisonDetails(data)
    pushRaw('comparisonDetails', data)
    // Also load base version snapshot for side-by-side rendered view
    try {
      const baseData = await api.getEndpointDetails(comparisonProject, comparisonWebsite, url, baseVersionId, baseVersionId)
      setBaseSnapshot(baseData)
    } catch {
      setBaseSnapshot(null)
    }
  }

  // Comparison page data loaders
  const loadComparisonProjects = async () => {
    const data = await api.listProjects()
    setComparisonProjects(data)
    pushRaw('comparisonProjects', data)
  }

  const loadComparisonWebsites = async (project: string) => {
    if (!project) {
      setComparisonWebsites([])
      return
    }
    const data = await api.listWebsites(project)
    setComparisonWebsites(data)
    pushRaw('comparisonWebsites', data)
  }

  const loadComparisonEndpoints = async (project: string, website: string) => {
    if (!project || !website) {
      setComparisonEndpoints([])
      return
    }
    const data = await api.listEndpoints(project, website, '*', 500)
    setComparisonEndpoints(data)
    pushRaw('comparisonEndpoints', data)
  }

  const loadComparisonVersions = async (project: string, website: string) => {
    if (!project || !website) {
      setVersions([])
      return
    }
    const data = await api.listVersions(project, website)
    setVersions(data)
    pushRaw('comparisonVersions', data)
  }

  // Navigation functions for comparison view
  const switchToComparisonFromHeader = async () => {
    await withAction('Open version comparison', async () => {
      // Load projects if not already loaded
      if (comparisonProjects.length === 0) {
        await loadComparisonProjects()
      }
      // Load data if we have remembered selections
      if (comparisonProject) {
        await loadComparisonWebsites(comparisonProject)
        if (comparisonWebsite) {
          await loadComparisonEndpoints(comparisonProject, comparisonWebsite)
          await loadComparisonVersions(comparisonProject, comparisonWebsite)
        }
      }
      setViewMode('comparison')
    })
  }

  const switchToComparisonFromEndpoint = async () => {
    await withAction('Compare versions for endpoint', async () => {
      // Pre-populate comparison state with current dashboard selections
      setComparisonProject(selectedProject)
      setComparisonWebsite(selectedSite)
      setComparisonEndpoint(selectedEndpointUrl)
      
      // Load necessary data
      await loadComparisonProjects()
      await loadComparisonWebsites(selectedProject)
      await loadComparisonEndpoints(selectedProject, selectedSite)
      await loadComparisonVersions(selectedProject, selectedSite)
      
      setViewMode('comparison')
    })
  }

  const compareVersions = async () => {
    await withAction('Compare versions', async () => {
      if (!comparisonEndpoint) throw new Error('Select an endpoint first')
      if (!selectedBaseVersion || !selectedHeadVersion) throw new Error('Select both base and head versions')
      await loadComparisonDetails(comparisonEndpoint, selectedBaseVersion, selectedHeadVersion)
    })
  }

  // Comparison page selection handlers
  const handleComparisonProjectChange = async (project: string) => {
    setComparisonProject(project)
    setComparisonWebsite('')
    setComparisonEndpoint('')
    setSelectedBaseVersion('')
    setSelectedHeadVersion('')
    setComparisonDetails(null)
    setVersions([])
    if (project) {
      await withAction(`Load websites for ${project}`, async () => {
        await loadComparisonWebsites(project)
      })
    } else {
      setComparisonWebsites([])
    }
  }

  const handleComparisonWebsiteChange = async (website: string) => {
    setComparisonWebsite(website)
    setComparisonEndpoint('')
    setSelectedBaseVersion('')
    setSelectedHeadVersion('')
    setComparisonDetails(null)
    if (website && comparisonProject) {
      await withAction(`Load endpoints for ${website}`, async () => {
        await loadComparisonEndpoints(comparisonProject, website)
        await loadComparisonVersions(comparisonProject, website)
      })
    } else {
      setComparisonEndpoints([])
      setVersions([])
    }
  }

  const handleComparisonEndpointChange = (endpoint: string) => {
    setComparisonEndpoint(endpoint)
    setSelectedBaseVersion('')
    setSelectedHeadVersion('')
    setComparisonDetails(null)
  }

  const createProject = async () =>
    withAction('Create project', async () => {
      const project = await api.createProject({
        slug: projectSlug,
        name: projectName,
        description: projectDescription,
      })
      pushRaw('createProject', project)
      setSelectedProject(project.slug)
      await refreshProjects()
      await refreshWebsites(project.slug)
      logActivity('Project created', project.slug)
    })

  const createWebsite = async () =>
    withAction('Create website', async () => {
      if (!selectedProject) throw new Error('Select a project first')

      const normalizedOrigin = /^https?:\/\//.test(siteOrigin) ? siteOrigin : config.demoOrigin

      const website = await api.createWebsite(selectedProject, {
        slug: siteSlug,
        origin: normalizedOrigin,
      })
      pushRaw('createWebsite', website)
      setSelectedSite(website.slug)
      await refreshWebsites(selectedProject)
      logActivity('Website created', website.slug)
    })

  const startEnumerate = async () =>
    withAction('Start enumerate job', async () => {
      if (!selectedProject || !selectedSite) throw new Error('Select project and site first')
      const cfg = buildEnumerationConfig()
      const job = await api.startEnumerate(selectedProject, selectedSite, cfg)
      pushRaw('startEnumerate', job)
      logActivity('Enumerate started', job.id)
      await waitForJob(job.id)
      await refreshEndpoints()
      await refreshJobs()
    })

  const startEnumerateWebSocket = async () =>
    withAction('Start enumerate via websocket', async () => {
      if (!selectedProject || !selectedSite) throw new Error('Select project and site first')

      const cfg = buildEnumerationConfig()

      await new Promise<void>((resolve, reject) => {
        const { socket, sendConfig, onMessage } = createEnumerateSocket(
          selectedProject,
          selectedSite,
          cfg,
        )

        socket.onerror = () => reject(new Error('WebSocket connection failed'))

        socket.onopen = () => {
          sendConfig()
        }

        onMessage(async (payload) => {
          pushRaw('ws:enumerate', payload)
          if ('error' in payload && payload.error) {
            socket.close()
            reject(new Error(payload.error))
            return
          }

          if ('type' in payload) {
            const event = payload as JobEvent
            logActivity(`WS ${event.type}`, `${event.status || ''} ${event.processed || ''}/${event.total || ''}`)
            if (event.type === 'result' || event.status === 'done') {
              socket.close()
              await refreshEndpoints()
              await refreshJobs()
              resolve()
            }
            if (event.status === 'failed' || event.status === 'canceled') {
              socket.close()
              reject(new Error(event.error || `Job ${event.status}`))
            }
            return
          }

          const job = payload as Job
          setJobs((prev) => [job, ...prev.filter((entry) => entry.id !== job.id)])
          logActivity('WS job created', `${job.id} (${job.type})`)
        })
      })
    })

  const startFetch = async () =>
    withAction('Start fetch job', async () => {
      if (!selectedProject || !selectedSite) throw new Error('Select project and site first')
      const job = await api.startFetch(selectedProject, selectedSite, {
        status: fetchStatus,
        limit: fetchLimit,
        config: buildFetchConfig(),
      })
      pushRaw('startFetch', job)
      logActivity('Fetch started', job.id)
      await waitForJob(job.id)
      await refreshEndpoints()
      if (selectedEndpointUrl) {
        await loadDetails(selectedEndpointUrl)
      }
      await refreshJobs()
    })

  const startFetchWebSocket = async () =>
    withAction('Start fetch via websocket', async () => {
      if (!selectedProject || !selectedSite) throw new Error('Select project and site first')

      await new Promise<void>((resolve, reject) => {
        const { socket, sendRequest, onMessage } = createFetchSocket(selectedProject, selectedSite, {
          status: fetchStatus,
          limit: fetchLimit,
          config: buildFetchConfig(),
        })

        socket.onerror = () => reject(new Error('WebSocket connection failed'))

        socket.onopen = () => {
          // Send request as first message after connection opens
          sendRequest()
        }

        onMessage(async (payload) => {
          pushRaw('ws:fetch', payload)
          if ('error' in payload && payload.error) {
            socket.close()
            reject(new Error(payload.error))
            return
          }

          if ('type' in payload) {
            const event = payload as JobEvent
            logActivity(`WS ${event.type}`, `${event.status || ''} ${event.processed || ''}/${event.total || ''}`)
            if (event.type === 'result' || event.status === 'done') {
              socket.close()
              await refreshEndpoints()
              await refreshJobs()
              resolve()
            }
            if (event.status === 'failed' || event.status === 'canceled') {
              socket.close()
              reject(new Error(event.error || `Job ${event.status}`))
            }
            return
          }

          const job = payload as Job
          setJobs((prev) => [job, ...prev.filter((entry) => entry.id !== job.id)])
          logActivity('WS job created', `${job.id} (${job.type})`)
        })
      })
    })

  // Demo-specific functions (commented out for now)
  // const resetDemo = async () =>
  //   withAction('Reset demo versions', async () => {
  //     const data = await demoApi.reset()
  //     pushRaw('demoReset', data)
  //     await refreshDemoVersions()
  //     logActivity('Demo reset', data.message)
  //   })

  // const bumpDemo = async () =>
  //   withAction('Bump all demo versions', async () => {
  //     const data = await demoApi.bumpAll()
  //     pushRaw('demoBump', data)
  //     await refreshDemoVersions()
  //     logActivity('Demo bumped', data.message)
  //   })

  // const setDemoPathVersion = async (path: string, version: number) =>
  //   withAction(`Set ${path} -> v${version}`, async () => {
  //     const data = await demoApi.setVersion(path, version)
  //     pushRaw(`setVersion:${path}`, data)
  //     await refreshDemoVersions()
  //   })

  useEffect(() => {
    void withAction('Initial load', async () => {
      await Promise.all([refreshProjects(), refreshJobs(), refreshDemoVersions()])
    })
  }, [])

  useEffect(() => {
    void refreshWebsites(selectedProject)
  }, [selectedProject])

  const selectedProjectLabel = useMemo(
    () => projects.find((project) => project.slug === selectedProject)?.name || selectedProject,
    [projects, selectedProject],
  )

  const selectedWebsite = useMemo(
    () => websites.find((site) => site.slug === selectedSite) || null,
    [websites, selectedSite],
  )
  const selectedWebsiteOriginInvalid =
    !!selectedWebsite && !/^https?:\/\//.test(selectedWebsite.origin || '')

  return (
    <div className="app">
      <header className="top">
        <div>
          <h1>Moku Demo UI</h1>
          <p>
            Isolated React GUI for the current backend API. Base API: <code>{config.apiBase}</code>, Demo server:{' '}
            <code>{config.demoOrigin}</code>
          </p>
        </div>
        <div className="row" style={{ gap: '8px' }}>
          <button className="primaryBtn" disabled={busy} onClick={() => void switchToComparisonFromHeader()}>
            Version Comparison
          </button>
          <button className="primaryBtn" disabled={busy} onClick={() => void withAction('Refresh all', async () => {
            await Promise.all([
              refreshProjects(),
              refreshWebsites(selectedProject),
              refreshJobs(),
              refreshEndpoints(),
              refreshDemoVersions(),
            ])
            if (selectedEndpointUrl) {
              await loadDetails(selectedEndpointUrl)
            }
          })}>
            Refresh all
          </button>
        </div>
      </header>

      {error && <div className="error">{error}</div>}
      {selectedWebsiteOriginInvalid && (
        <div className="error">
          Selected website origin is invalid for crawling: <code>{selectedWebsite?.origin}</code>. Create/select a
          website with an absolute origin like <code>http://localhost:9999</code>.
        </div>
      )}

      {viewMode === 'dashboard' ? (
      <main className="grid">
        <section className="card">
          <h2>1) Projects</h2>
          <div className="row">
            <input value={projectSlug} onChange={(event) => setProjectSlug(event.target.value)} placeholder="slug" />
            <input value={projectName} onChange={(event) => setProjectName(event.target.value)} placeholder="name" />
          </div>
          <input
            value={projectDescription}
            onChange={(event) => setProjectDescription(event.target.value)}
            placeholder="description"
          />
          <div className="row">
            <button disabled={busy} onClick={() => void createProject()}>
              Create project
            </button>
            <button disabled={busy} onClick={() => void refreshProjects()}>
              Reload projects
            </button>
          </div>
          <select value={selectedProject} onChange={(event) => setSelectedProject(event.target.value)}>
            <option value="">Select project</option>
            {projects.map((project) => (
              <option key={project.id} value={project.slug}>
                {project.slug} — {project.name}
              </option>
            ))}
          </select>
        </section>

        <section className="card">
          <h2>2) Websites ({selectedProjectLabel || 'none'})</h2>
          <div className="row">
            <input value={siteSlug} onChange={(event) => setSiteSlug(event.target.value)} placeholder="site slug" />
            <input value={siteOrigin} onChange={(event) => setSiteOrigin(event.target.value)} placeholder="origin" />
          </div>
          <div className="row">
            <button disabled={busy} onClick={() => void createWebsite()}>
              Create website
            </button>
            <button disabled={busy || !selectedProject} onClick={() => void refreshWebsites(selectedProject)}>
              Reload websites
            </button>
          </div>
          <select value={selectedSite} onChange={(event) => setSelectedSite(event.target.value)}>
            <option value="">Select website</option>
            {websites.map((site) => (
              <option key={site.id} value={site.slug}>
                {site.slug} — {site.origin}
              </option>
            ))}
          </select>
        </section>

        <section className="card wide">
          <h2>3) Jobs</h2>

          {/* Enumeration Config */}
          <div className="enumConfig">
            <h4>Enumeration Methods</h4>
            <div className="row">
              <label className="checkLabel">
                <input
                  type="checkbox"
                  checked={enumSpiderEnabled}
                  onChange={(e) => setEnumSpiderEnabled(e.target.checked)}
                />
                Spider
              </label>
              {enumSpiderEnabled && (
                <>
                  <label>
                    depth
                    <input
                      type="number"
                      value={enumSpiderMaxDepth}
                      min={1}
                      max={20}
                      onChange={(e) => setEnumSpiderMaxDepth(Number(e.target.value) || 4)}
                    />
                  </label>
                  <label>
                    concurrency
                    <input
                      type="number"
                      value={enumSpiderConcurrency}
                      min={1}
                      max={50}
                      onChange={(e) => setEnumSpiderConcurrency(Number(e.target.value) || 5)}
                    />
                  </label>
                </>
              )}
            </div>
            <div className="row">
              <label className="checkLabel">
                <input
                  type="checkbox"
                  checked={enumSitemapEnabled}
                  onChange={(e) => setEnumSitemapEnabled(e.target.checked)}
                />
                Sitemap
              </label>
              <label className="checkLabel">
                <input
                  type="checkbox"
                  checked={enumRobotsEnabled}
                  onChange={(e) => setEnumRobotsEnabled(e.target.checked)}
                />
                Robots
              </label>
            </div>
            <div className="row">
              <label className="checkLabel">
                <input
                  type="checkbox"
                  checked={enumWaybackEnabled}
                  onChange={(e) => setEnumWaybackEnabled(e.target.checked)}
                />
                Wayback
              </label>
              {enumWaybackEnabled && (
                <>
                  <label className="checkLabel">
                    <input
                      type="checkbox"
                      checked={enumWaybackMachine}
                      onChange={(e) => setEnumWaybackMachine(e.target.checked)}
                    />
                    Archive.org
                  </label>
                  <label className="checkLabel">
                    <input
                      type="checkbox"
                      checked={enumCommonCrawl}
                      onChange={(e) => setEnumCommonCrawl(e.target.checked)}
                    />
                    CommonCrawl
                  </label>
                </>
              )}
            </div>
          </div>

          <div className="row">
            <button
              disabled={busy || !selectedProject || !selectedSite || selectedWebsiteOriginInvalid}
              onClick={() => void startEnumerate()}
            >
              Enumerate (REST)
            </button>
            <button
              disabled={busy || !selectedProject || !selectedSite || selectedWebsiteOriginInvalid}
              onClick={() => void startEnumerateWebSocket()}
            >
              Enumerate (WebSocket)
            </button>
            <button
              disabled={busy || !selectedProject || !selectedSite || selectedWebsiteOriginInvalid}
              onClick={() => void startFetch()}
            >
              Fetch (REST)
            </button>
            <button
              disabled={busy || !selectedProject || !selectedSite || selectedWebsiteOriginInvalid}
              onClick={() => void startFetchWebSocket()}
            >
              Fetch (WebSocket)
            </button>
            <button disabled={busy} onClick={() => void refreshJobs()}>
              Reload jobs
            </button>
          </div>
          <div className="row">
            <label>
              status
              <input value={fetchStatus} onChange={(event) => setFetchStatus(event.target.value)} />
            </label>
            <label>
              limit
              <input
                type="number"
                value={fetchLimit}
                onChange={(event) => setFetchLimit(Number(event.target.value || 100))}
              />
            </label>
            <label>
              concurrency
              <input
                type="number"
                value={fetchConcurrency}
                min="1"
                max="100"
                onChange={(event) => setFetchConcurrency(Number(event.target.value || 4))}
              />
            </label>
          </div>
          <div className="tableWrap">
            <table>
              <thead>
                <tr>
                  <th>id</th>
                  <th>type</th>
                  <th>status</th>
                  <th>project</th>
                  <th>website</th>
                  <th>started</th>
                  <th>ended</th>
                </tr>
              </thead>
              <tbody>
                {jobs.map((job) => (
                  <tr key={job.id}>
                    <td title={job.id}>{job.id.slice(0, 8)}</td>
                    <td>{job.type}</td>
                    <td>{job.status}</td>
                    <td>{job.project}</td>
                    <td>{job.website}</td>
                    <td>{job.started_at?.replace('T', ' ').replace('Z', '')}</td>
                    <td>{job.ended_at?.replace('T', ' ').replace('Z', '') || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className="card wide">
          <h2>Filter Configuration</h2>
          <FilterConfigPanel
            project={selectedProject}
            site={selectedSite}
            onActivity={logActivity}
          />
        </section>

        <section className="card wide">
          <h2>4) Endpoints</h2>
          <div className="row">
            <label>
              filter status
              <select value={endpointFilterStatus} onChange={(event) => setEndpointFilterStatus(event.target.value)}>
                <option value="">Non-filtered (default)</option>
                <option value="*">All (including filtered)</option>
                <option value="pending">Pending</option>
                <option value="fetched">Fetched</option>
                <option value="failed">Failed</option>
                <option value="filtered">Filtered only</option>
                <option value="new">New</option>
              </select>
            </label>
            <label>
              limit
              <input
                type="number"
                value={endpointFilterLimit}
                onChange={(event) => setEndpointFilterLimit(Number(event.target.value || 200))}
              />
            </label>
            <button disabled={busy || !selectedProject || !selectedSite} onClick={() => void refreshEndpoints()}>
              Load endpoints
            </button>
          </div>

          <div className="tableWrap">
            <table>
              <thead>
                <tr>
                  <th>select</th>
                  <th>canonical url</th>
                  <th>status</th>
                  <th>source</th>
                  <th>last fetched version</th>
                </tr>
              </thead>
              <tbody>
                {endpoints.map((endpoint) => (
                  <tr key={endpoint.id}>
                    <td>
                      <button
                        disabled={busy}
                        onClick={() => {
                          setSelectedEndpointUrl(endpoint.canonical_url)
                          void withAction(`Load details for ${endpoint.path || endpoint.canonical_url}`, async () => {
                            await loadDetails(endpoint.canonical_url)
                          })
                        }}
                      >
                        View
                      </button>
                    </td>
                    <td title={endpoint.canonical_url}>{endpoint.canonical_url}</td>
                    <td>{endpoint.status}</td>
                    <td>{endpoint.source}</td>
                    <td>{endpoint.last_fetched_version || '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className="card wide">
          <h2>5) Endpoint details {selectedEndpointUrl ? `for ${selectedEndpointUrl}` : ''}</h2>
          {!details ? (
            <p>No endpoint details loaded yet.</p>
          ) : (
            <div className="detailsGrid">
              <div>
                <h3>Snapshot</h3>
                <ul>
                  <li>id: {details.snapshot?.id}</li>
                  <li>version: {details.snapshot?.version_id}</li>
                  <li>status: {details.snapshot?.status_code}</li>
                  <li>url: {details.snapshot?.url}</li>
                </ul>
                <button 
                  disabled={busy || !selectedProject || !selectedSite || !selectedEndpointUrl} 
                  onClick={() => void switchToComparisonFromEndpoint()}
                  className="primaryBtn"
                >
                  Compare Versions
                </button>
                <h4>Headers</h4>
                <pre>{JSON.stringify(details.snapshot?.headers ?? {}, null, 2)}</pre>
                <h4>Body (truncated)</h4>
                <pre>{(details.snapshot?.body || '').slice(0, 2000)}</pre>
              </div>

              <div>
                <h3>Score result</h3>
                <pre>{JSON.stringify(details.score_result ?? null, null, 2)}</pre>
                <h3>Security diff</h3>
                <pre>{JSON.stringify(details.security_diff ?? null, null, 2)}</pre>
                <h3>Content/Header diff</h3>
                <pre>{JSON.stringify(details.diff ?? null, null, 2)}</pre>
              </div>
            </div>
          )}
        </section>

        <section className="card">
          <h2>Activity log</h2>
          <div className="logList">
            {activities.map((item, index) => (
              <div key={`${item.at}-${item.title}-${index}`} className="logItem">
                <strong>[{item.at}]</strong> {item.title}
                {item.detail ? <div className="muted">{item.detail}</div> : null}
              </div>
            ))}
          </div>
        </section>

        <section className="card">
          <h2>Raw JSON inspector</h2>
          <div className="logList">
            {rawItems.map((item) => (
              <details key={item.key}>
                <summary>{item.key}</summary>
                <pre>{JSON.stringify(item.value, null, 2)}</pre>
              </details>
            ))}
          </div>
        </section>
      </main>
      ) : (
        <main className="comparisonView">
          <div className="comparisonHeader">
            <button onClick={() => setViewMode('dashboard')} className="primaryBtn">
              ← Back to Dashboard
            </button>
            <h2>Version Comparison</h2>
          </div>

          <section className="card comparisonSelectors">
            <h3>1) Select Endpoint</h3>
            <div className="selectorRow">
              <div className="selectorGroup">
                <label>Project</label>
                <select 
                  value={comparisonProject} 
                  onChange={(e) => void handleComparisonProjectChange(e.target.value)}
                  disabled={busy}
                >
                  <option value="">Select project</option>
                  {comparisonProjects.map((project) => (
                    <option key={project.id} value={project.slug}>
                      {project.slug} — {project.name}
                    </option>
                  ))}
                </select>
              </div>
              <div className="selectorGroup">
                <label>Website</label>
                <select 
                  value={comparisonWebsite} 
                  onChange={(e) => void handleComparisonWebsiteChange(e.target.value)}
                  disabled={busy || !comparisonProject}
                >
                  <option value="">Select website</option>
                  {comparisonWebsites.map((site) => (
                    <option key={site.id} value={site.slug}>
                      {site.slug} — {site.origin}
                    </option>
                  ))}
                </select>
              </div>
              <div className="selectorGroup">
                <label>Endpoint</label>
                <select 
                  value={comparisonEndpoint} 
                  onChange={(e) => handleComparisonEndpointChange(e.target.value)}
                  disabled={busy || !comparisonWebsite}
                >
                  <option value="">Select endpoint</option>
                  {comparisonEndpoints.map((endpoint) => (
                    <option key={endpoint.id} value={endpoint.canonical_url}>
                      {endpoint.canonical_url}
                    </option>
                  ))}
                </select>
              </div>
            </div>
            {comparisonEndpoint && (
              <div style={{ marginTop: '8px', color: '#9ca3af', fontSize: '14px' }}>
                Selected: {comparisonEndpoint}
              </div>
            )}
          </section>

          <section className="card">
            <h3>2) Select Versions to Compare</h3>
            <div className="versionSelector">
              <div className="row">
                <label>
                  Base Version (older)
                  <select 
                    value={selectedBaseVersion} 
                    onChange={(e) => setSelectedBaseVersion(e.target.value)}
                    disabled={busy || !comparisonEndpoint}
                  >
                    <option value="">Select base version</option>
                    {versions.map((v) => (
                      <option key={v.id} value={v.id}>
                        {v.message} — {v.author} — {new Date(v.timestamp).toLocaleString()}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Head Version (newer)
                  <select 
                    value={selectedHeadVersion} 
                    onChange={(e) => setSelectedHeadVersion(e.target.value)}
                    disabled={busy || !comparisonEndpoint}
                  >
                    <option value="">Select head version</option>
                    {versions.map((v) => (
                      <option key={v.id} value={v.id}>
                        {v.message} — {v.author} — {new Date(v.timestamp).toLocaleString()}
                      </option>
                    ))}
                  </select>
                </label>
                <button 
                  disabled={busy || !selectedBaseVersion || !selectedHeadVersion || !comparisonEndpoint}
                  onClick={() => void compareVersions()}
                  className="primaryBtn"
                >
                  Compare
                </button>
              </div>
            </div>
          </section>

          {comparisonDetails && (
            <>
              <section className="card">
                <h3>Layout</h3>
                <div className="layoutToggle">
                  <button 
                    className={comparisonLayout === 'side-by-side' ? 'active' : ''}
                    onClick={() => setComparisonLayout('side-by-side')}
                  >
                    Side by Side
                  </button>
                  <button 
                    className={comparisonLayout === 'unified' ? 'active' : ''}
                    onClick={() => setComparisonLayout('unified')}
                  >
                    Unified
                  </button>
                  <button 
                    className={comparisonLayout === 'split' ? 'active' : ''}
                    onClick={() => setComparisonLayout('split')}
                  >
                    Split View
                  </button>
                </div>
              </section>

              <section className="card">
                <h3>Security Diff</h3>
                {comparisonDetails.security_diff ? (
                  <div className="securityDiff">
                    <div className="scoreComparison">
                      <div>
                        <strong>Base Score:</strong> {comparisonDetails.security_diff.score_base?.toFixed(2) ?? 'N/A'}
                      </div>
                      <div>
                        <strong>Head Score:</strong> {comparisonDetails.security_diff.score_head?.toFixed(2) ?? 'N/A'}
                      </div>
                      <div className={
                        (comparisonDetails.security_diff.score_delta ?? 0) > 0 ? 'scoreDeltaPositive' :
                        (comparisonDetails.security_diff.score_delta ?? 0) < 0 ? 'scoreDeltaNegative' :
                        'scoreDeltaNeutral'
                      }>
                        <strong>Delta:</strong> {comparisonDetails.security_diff.score_delta?.toFixed(2) ?? '0.00'}
                        {(comparisonDetails.security_diff.score_delta ?? 0) > 0 && ' (improved)'}
                        {(comparisonDetails.security_diff.score_delta ?? 0) < 0 && ' (regressed)'}
                      </div>
                    </div>

                    {comparisonDetails.security_diff.attack_surface_changes && comparisonDetails.security_diff.attack_surface_changes.length > 0 && (
                      <div className="attackSurfaceChanges">
                        <h4>Attack Surface Changes</h4>
                        <ul>
                          {comparisonDetails.security_diff.attack_surface_changes.map((change, idx) => (
                            <li key={idx} className={`changeKind-${change.kind}`}>
                              <strong>{change.kind}:</strong> {change.detail}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}

                    {comparisonDetails.security_diff.feature_deltas && Object.keys(comparisonDetails.security_diff.feature_deltas).length > 0 && (
                      <div className="featureDeltas">
                        <h4>Feature Changes</h4>
                        <pre>{JSON.stringify(comparisonDetails.security_diff.feature_deltas, null, 2)}</pre>
                      </div>
                    )}
                  </div>
                ) : (
                  <p>No security diff available</p>
                )}
              </section>

              <section className="card wide">
                <h3>Rendered Views</h3>
                <p style={{ color: '#9ca3af', fontSize: '13px', marginBottom: '12px' }}>
                  Interactive HTML visualization with security highlights
                </p>
                <RenderedDiffViews
                  baseSnapshot={baseSnapshot?.snapshot}
                  headSnapshot={comparisonDetails.snapshot}
                  securityDiff={comparisonDetails.security_diff}
                  diff={comparisonDetails.diff}
                  viewMode={renderedViewMode}
                  onViewModeChange={setRenderedViewMode}
                />
              </section>

              <section className="card">
                <h3>Header Diff</h3>
                {comparisonDetails.diff?.headers_diff ? (
                  <div className="headerDiff">
                    {comparisonDetails.diff.headers_diff.added && Object.keys(comparisonDetails.diff.headers_diff.added).length > 0 && (
                      <div className="diffAdded">
                        <h4>Added Headers</h4>
                        <ul>
                          {Object.entries(comparisonDetails.diff.headers_diff.added).map(([key, values]) => (
                            <li key={key}>
                              <strong>{key}:</strong> {values.join(', ')}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}

                    {comparisonDetails.diff.headers_diff.removed && Object.keys(comparisonDetails.diff.headers_diff.removed).length > 0 && (
                      <div className="diffRemoved">
                        <h4>Removed Headers</h4>
                        <ul>
                          {Object.entries(comparisonDetails.diff.headers_diff.removed).map(([key, values]) => (
                            <li key={key}>
                              <strong>{key}:</strong> {values.join(', ')}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}

                    {comparisonDetails.diff.headers_diff.changed && Object.keys(comparisonDetails.diff.headers_diff.changed).length > 0 && (
                      <div className="diffChanged">
                        <h4>Changed Headers</h4>
                        <ul>
                          {Object.entries(comparisonDetails.diff.headers_diff.changed).map(([key, change]) => (
                            <li key={key}>
                              <strong>{key}:</strong> {change.from.join(', ')} → {change.to.join(', ')}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}

                    {(!comparisonDetails.diff.headers_diff.added || Object.keys(comparisonDetails.diff.headers_diff.added).length === 0) &&
                     (!comparisonDetails.diff.headers_diff.removed || Object.keys(comparisonDetails.diff.headers_diff.removed).length === 0) &&
                     (!comparisonDetails.diff.headers_diff.changed || Object.keys(comparisonDetails.diff.headers_diff.changed).length === 0) && (
                      <p>No header changes</p>
                    )}
                  </div>
                ) : (
                  <p>No header diff available</p>
                )}
              </section>

              <section className="card">
                <h3>Body Diff</h3>
                {comparisonDetails.diff?.body_diff?.chunks ? (
                  <div className={`bodyDiff layout-${comparisonLayout}`}>
                    {comparisonLayout === 'side-by-side' && (
                      <div className="sideBySide">
                        <div className="diffColumn">
                          <h4>Base</h4>
                          <pre>
                            {comparisonDetails.diff.body_diff.chunks.map((chunk, idx) => (
                              <div key={idx} className={`chunk chunk-${chunk.type}`}>
                                {chunk.type === 'removed' ? chunk.content : ''}
                              </div>
                            ))}
                          </pre>
                        </div>
                        <div className="diffColumn">
                          <h4>Head</h4>
                          <pre>
                            {comparisonDetails.diff.body_diff.chunks.map((chunk, idx) => (
                              <div key={idx} className={`chunk chunk-${chunk.type}`}>
                                {chunk.type === 'added' ? chunk.content : ''}
                              </div>
                            ))}
                          </pre>
                        </div>
                      </div>
                    )}

                    {comparisonLayout === 'unified' && (
                      <div className="unified">
                        <pre>
                          {comparisonDetails.diff.body_diff.chunks.map((chunk, idx) => (
                            <div key={idx} className={`chunk chunk-${chunk.type}`}>
                              {chunk.type === 'removed' && '- '}
                              {chunk.type === 'added' && '+ '}
                              {chunk.content}
                            </div>
                          ))}
                        </pre>
                      </div>
                    )}

                    {comparisonLayout === 'split' && (
                      <div className="split">
                        <div className="tabs">
                          <button 
                            className={comparisonLayout === 'split' ? 'active' : ''}
                            onClick={() => {
                              const baseTab = document.querySelector('.splitBase') as HTMLElement
                              const headTab = document.querySelector('.splitHead') as HTMLElement
                              if (baseTab && headTab) {
                                baseTab.style.display = 'block'
                                headTab.style.display = 'none'
                              }
                            }}
                          >
                            Base Version
                          </button>
                          <button 
                            onClick={() => {
                              const baseTab = document.querySelector('.splitBase') as HTMLElement
                              const headTab = document.querySelector('.splitHead') as HTMLElement
                              if (baseTab && headTab) {
                                baseTab.style.display = 'none'
                                headTab.style.display = 'block'
                              }
                            }}
                          >
                            Head Version
                          </button>
                        </div>
                        <div className="splitBase">
                          <pre>
                            {comparisonDetails.diff.body_diff.chunks
                              .filter(chunk => chunk.type === 'removed')
                              .map((chunk, idx) => (
                                <div key={idx}>{chunk.content}</div>
                              ))}
                          </pre>
                        </div>
                        <div className="splitHead" style={{ display: 'none' }}>
                          <pre>
                            {comparisonDetails.diff.body_diff.chunks
                              .filter(chunk => chunk.type === 'added')
                              .map((chunk, idx) => (
                                <div key={idx}>{chunk.content}</div>
                              ))}
                          </pre>
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <p>No body diff available</p>
                )}
              </section>
            </>
          )}
        </main>
      )}
    </div>
  )
}
