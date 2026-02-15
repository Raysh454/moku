import { useEffect, useMemo, useState } from 'react'
import { api, config, createJobSocket, demoApi } from './api/client'
import type { DemoPageVersion, Endpoint, EndpointDetails, Job, JobEvent, Project, Website } from './api/types'

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
  const [demoVersions, setDemoVersions] = useState<DemoPageVersion[]>([])

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
  const [endpointFilterStatus, setEndpointFilterStatus] = useState('*')
  const [endpointFilterLimit, setEndpointFilterLimit] = useState(200)

  const [activities, setActivities] = useState<Activity[]>([])
  const [rawItems, setRawItems] = useState<RawItem[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

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
      const job = await api.startEnumerate(selectedProject, selectedSite)
      pushRaw('startEnumerate', job)
      logActivity('Enumerate started', job.id)
      await waitForJob(job.id)
      await refreshEndpoints()
      await refreshJobs()
    })

  const startFetch = async () =>
    withAction('Start fetch job', async () => {
      if (!selectedProject || !selectedSite) throw new Error('Select project and site first')
      const job = await api.startFetch(selectedProject, selectedSite, {
        status: fetchStatus,
        limit: fetchLimit,
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
        const { socket, onMessage } = createJobSocket('fetch', selectedProject, selectedSite, {
          status: fetchStatus,
          limit: fetchLimit,
        })

        socket.onerror = () => reject(new Error('WebSocket connection failed'))

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

  const resetDemo = async () =>
    withAction('Reset demo versions', async () => {
      const data = await demoApi.reset()
      pushRaw('demoReset', data)
      await refreshDemoVersions()
      logActivity('Demo reset', data.message)
    })

  const bumpDemo = async () =>
    withAction('Bump all demo versions', async () => {
      const data = await demoApi.bumpAll()
      pushRaw('demoBump', data)
      await refreshDemoVersions()
      logActivity('Demo bumped', data.message)
    })

  const setDemoPathVersion = async (path: string, version: number) =>
    withAction(`Set ${path} -> v${version}`, async () => {
      const data = await demoApi.setVersion(path, version)
      pushRaw(`setVersion:${path}`, data)
      await refreshDemoVersions()
    })

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
      </header>

      {error && <div className="error">{error}</div>}
      {selectedWebsiteOriginInvalid && (
        <div className="error">
          Selected website origin is invalid for crawling: <code>{selectedWebsite?.origin}</code>. Create/select a
          website with an absolute origin like <code>http://localhost:9999</code>.
        </div>
      )}

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
          <div className="row">
            <button
              disabled={busy || !selectedProject || !selectedSite || selectedWebsiteOriginInvalid}
              onClick={() => void startEnumerate()}
            >
              Enumerate (REST)
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
          <h2>4) Endpoints</h2>
          <div className="row">
            <label>
              filter status
              <input value={endpointFilterStatus} onChange={(event) => setEndpointFilterStatus(event.target.value)} />
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
    </div>
  )
}
