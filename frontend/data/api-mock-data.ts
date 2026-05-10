// Notes:
// - tracker.models.Snapshot.Body is []byte in Go, so JSON encodes it as base64 string.
// - EndpointDetails omits diff/security_diff when not available (omitempty).

export const MOCK_API_RESPONSES = {
  'GET /projects': [
    {
      id: 'proj_01',
      slug: 'acme',
      name: 'Acme Corp',
      description: 'Example project for UI mocks.',
      created_at: 1766354606,
      meta: '{}'
    },
    {
      id: 'proj_02',
      slug: 'targetcorp',
      name: 'TargetCorp',
      description: 'Multi-domain monitoring for TargetCorp infrastructure.',
      created_at: 1766353606,
      meta: '{}'
    },
    {
      id: 'proj_03',
      slug: 'beta',
      name: 'Beta Analytics',
      description: 'Public analytics endpoints tracking.',
      created_at: 1766352606,
      meta: '{}'
    }
  ],

  // Projects -> Websites
  'GET /projects/acme/websites': [
    {
      id: 'site_01',
      project_id: 'proj_01',
      slug: 'example',
      origin: 'https://example.com',
      storage_path: '/mock/storage/path/example.com',
      created_at: 1766354606,
      last_seen_at: 1766355806,
      config: '{}'
    }
  ],

  'GET /projects/targetcorp/websites': [
    {
      id: 'site_02',
      project_id: 'proj_02',
      slug: 'web',
      origin: 'https://targetcorp.com',
      storage_path: '/mock/storage/path/targetcorp.com',
      created_at: 1766353606,
      last_seen_at: 1766355806,
      config: '{}'
    },
    {
      id: 'site_03',
      project_id: 'proj_02',
      slug: 'api',
      origin: 'https://api.targetcorp.com',
      storage_path: '/mock/storage/path/api.targetcorp.com',
      created_at: 1766353606,
      last_seen_at: 1766355806,
      config: '{}'
    }
  ],

  'GET /projects/beta/websites': [
    {
      id: 'site_04',
      project_id: 'proj_03',
      slug: 'analytics',
      origin: 'https://analytics.beta.io',
      storage_path: '/mock/storage/path/analytics.beta.io',
      created_at: 1766352606,
      last_seen_at: 1766355806,
      config: '{}'
    }
  ],

  // Websites -> Endpoints (indexer.Endpoint)
  'GET /projects/acme/websites/example/endpoints?limit=3': [
    {
      id: 'ep_acme_01',
      url: 'https://example.com/login',
      canonical_url: 'https://example.com/login',
      host: 'example.com',
      path: '/login',
      first_discovered_at: 1766354606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_02',
      last_fetched_at: 1766355806,
      status: 'fetched',
      source: 'api',
      meta: '{"tags":["auth"],"owner":"frontend"}'
    },
    {
      id: 'ep_acme_02',
      url: 'https://example.com/admin',
      canonical_url: 'https://example.com/admin',
      host: 'example.com',
      path: '/admin',
      first_discovered_at: 1766354606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_02',
      last_fetched_at: 1766355806,
      status: 'fetched',
      source: 'enumerator',
      meta: '{}'
    },
    {
      id: 'ep_acme_03',
      url: 'https://example.com/api/v1/users',
      canonical_url: 'https://example.com/api/v1/users',
      host: 'example.com',
      path: '/api/v1/users',
      first_discovered_at: 1766354606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_01',
      last_fetched_at: 1766354706,
      status: 'new',
      source: 'enumerator',
      meta: '{}'
    }
  ],

  'GET /projects/targetcorp/websites/web/endpoints?limit=3': [
    {
      id: 'ep_tc_web_01',
      url: 'https://targetcorp.com/login',
      canonical_url: 'https://targetcorp.com/login',
      host: 'targetcorp.com',
      path: '/login',
      first_discovered_at: 1766353606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_tc_02',
      last_fetched_at: 1766355806,
      status: 'fetched',
      source: 'api',
      meta: '{}'
    },
    {
      id: 'ep_tc_web_02',
      url: 'https://targetcorp.com/admin',
      canonical_url: 'https://targetcorp.com/admin',
      host: 'targetcorp.com',
      path: '/admin',
      first_discovered_at: 1766353606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_tc_02',
      last_fetched_at: 1766355806,
      status: 'fetched',
      source: 'enumerator',
      meta: '{}'
    },
    {
      id: 'ep_tc_web_03',
      url: 'https://targetcorp.com/status',
      canonical_url: 'https://targetcorp.com/status',
      host: 'targetcorp.com',
      path: '/status',
      first_discovered_at: 1766353606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_tc_01',
      last_fetched_at: 1766354706,
      status: 'fetched',
      source: 'enumerator',
      meta: '{}'
    }
  ],

  'GET /projects/targetcorp/websites/api/endpoints?limit=3': [
    {
      id: 'ep_tc_api_01',
      url: 'https://api.targetcorp.com/v1/users',
      canonical_url: 'https://api.targetcorp.com/v1/users',
      host: 'api.targetcorp.com',
      path: '/v1/users',
      first_discovered_at: 1766353606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_api_02',
      last_fetched_at: 1766355806,
      status: 'fetched',
      source: 'api',
      meta: '{}'
    },
    {
      id: 'ep_tc_api_02',
      url: 'https://api.targetcorp.com/health',
      canonical_url: 'https://api.targetcorp.com/health',
      host: 'api.targetcorp.com',
      path: '/health',
      first_discovered_at: 1766353606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_api_01',
      last_fetched_at: 1766354706,
      status: 'fetched',
      source: 'enumerator',
      meta: '{}'
    },
    {
      id: 'ep_tc_api_03',
      url: 'https://api.targetcorp.com/v1/orders',
      canonical_url: 'https://api.targetcorp.com/v1/orders',
      host: 'api.targetcorp.com',
      path: '/v1/orders',
      first_discovered_at: 1766353606,
      last_discovered_at: 1766355606,
      last_fetched_version: '',
      last_fetched_at: 0,
      status: 'new',
      source: 'enumerator',
      meta: '{}'
    }
  ],

  'GET /projects/beta/websites/analytics/endpoints?limit=3': [
    {
      id: 'ep_beta_01',
      url: 'https://analytics.beta.io/collect',
      canonical_url: 'https://analytics.beta.io/collect',
      host: 'analytics.beta.io',
      path: '/collect',
      first_discovered_at: 1766352606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_beta_01',
      last_fetched_at: 1766354706,
      status: 'fetched',
      source: 'enumerator',
      meta: '{}'
    },
    {
      id: 'ep_beta_02',
      url: 'https://analytics.beta.io/config',
      canonical_url: 'https://analytics.beta.io/config',
      host: 'analytics.beta.io',
      path: '/config',
      first_discovered_at: 1766352606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_beta_01',
      last_fetched_at: 1766354706,
      status: 'fetched',
      source: 'enumerator',
      meta: '{}'
    },
    {
      id: 'ep_beta_03',
      url: 'https://analytics.beta.io/health',
      canonical_url: 'https://analytics.beta.io/health',
      host: 'analytics.beta.io',
      path: '/health',
      first_discovered_at: 1766352606,
      last_discovered_at: 1766355606,
      last_fetched_version: 'ver_beta_01',
      last_fetched_at: 1766354706,
      status: 'fetched',
      source: 'enumerator',
      meta: '{}'
    }
  ],

  // Endpoint details (app.EndpointDetails)
  'GET /projects/acme/websites/example/endpoints/details?url=https%3A%2F%2Fexample.com%2Flogin': {
    snapshot: {
      id: 'snap_acme_login_v2',
      version_id: 'ver_02',
      status_code: 200,
      url: 'https://example.com/login',
      body: 'PCFkb2N0eXBlIGh0bWw+PGh0bWw+PGhlYWQ+PHRpdGxlPkFjbWUgTG9naW48L3RpdGxlPjxtZXRhIGh0dHAtZXF1aXY9IkNvbnRlbnQtU2VjdXJpdHktUG9saWN5IiBjb250ZW50PSJkZWZhdWx0LXNyYyAnc2VsZiciIC8+PC9oZWFkPjxib2R5PjxoMT5TaWduIGluIHRvIEFjbWU8L2gxPjxmb3JtPjxpbnB1dCBuYW1lPSJlbWFpbCIgLz48aW5wdXQgbmFtZT0icGFzc3dvcmQiIHR5cGU9InBhc3N3b3JkIiAvPjxidXR0b24+TG9naW48L2J1dHRvbj48L2Zvcm0+PHNjcmlwdCBzcmM9Ii9zdGF0aWMvYW5hbHl0aWNzLmpzIj48L3NjcmlwdD48L2JvZHk+PC9odG1sPg==',
      headers: {
        'content-type': ['text/html; charset=utf-8'],
        server: ['nginx'],
        'set-cookie': ['session=abc123; HttpOnly; Secure'],
        'x-content-type-options': ['nosniff']
      },
      created_at: '2026-01-26T22:11:50.350Z'
    },
    score_result: {
      score: 0.78,
      snapshot_id: 'snap_acme_login_v2',
      version_id: 'ver_02',
      normalized: 78,
      confidence: 0.9,
      version: 'v0.1.0',
      evidence: [
        {
          id: 'ev_csp_missing',
          key: 'csp_missing',
          rule_id: 'csp_missing',
          severity: 'high',
          description: 'Content-Security-Policy header is missing',
          value: 1,
          locations: [
            { type: 'header', snapshot_id: 'snap_acme_login_v2', header_name: 'content-security-policy' }
          ],
          contribution: 0.5
        },
        {
          id: 'ev_external_script',
          key: 'num_external_scripts',
          rule_id: 'num_external_scripts',
          severity: 'low',
          description: 'Number of external script tags on the page',
          value: 1,
          contribution: 0.1
        }
      ],
      meta: { snapshot_id: 'snap_acme_login_v2', url: 'https://example.com/login' },
      raw_features: { csp_missing: 1, num_external_scripts: 1, is_html: 1, status_2xx: 1 },
      contrib_by_rule: { csp_missing: 0.5, num_external_scripts: 0.1, is_html: 0, status_2xx: 0 },
      timestamp: '2026-01-26T22:11:50.350Z'
    },
    security_diff: {
      url: '/login',
      base_version_id: 'ver_01',
      head_version_id: 'ver_02',
      base_snapshot_id: 'snap_acme_login_v1',
      head_snapshot_id: 'snap_acme_login_v2',
      score_base: 0.82,
      score_head: 0.78,
      score_delta: -0.04,
      attack_surface_changed: true,
      attack_surface_changes: [
        {
          kind: 'script_added',
          detail: 'Scripts increased from 0 to 1',
          evidence_locations: [
            { type: 'script', snapshot_id: 'snap_acme_login_v2', dom_index: 1 }
          ]
        },
        {
          kind: 'cookie_added',
          detail: 'Cookie added: session',
          evidence_locations: [
            { type: 'cookie', snapshot_id: 'snap_acme_login_v2', cookie_name: 'session' }
          ]
        }
      ]
    },
    diff: {
      file_path: '/login',
      body_diff: {
        base_id: 'ver_01',
        head_id: 'ver_02',
        chunks: [
          {
            type: 'added',
            content: 'meta http-equiv="Content-Security-Policy" content="default-src \'self\'" /><',
            base_start: 53,
            head_start: 53,
            head_len: 74
          },
          {
            type: 'added',
            content: ' to Acme',
            base_start: 76,
            head_start: 150,
            head_len: 8
          },
          {
            type: 'added',
            content: 'script src="/static/analytics.js"></script><',
            base_start: 180,
            head_start: 262,
            head_len: 44
          }
        ]
      },
      headers_diff: {
        added: {
          'x-content-type-options': ['nosniff'],
          'set-cookie': ['session=abc123; HttpOnly; Secure']
        }
      }
    }
  },

  'GET /projects/acme/websites/example/endpoints/details?url=https%3A%2F%2Fexample.com%2Fadmin': {
    snapshot: {
      id: 'snap_acme_admin_v2',
      version_id: 'ver_02',
      status_code: 200,
      url: 'https://example.com/admin',
      body: 'PCFkb2N0eXBlIGh0bWw+PGh0bWw+PGhlYWQ+PHRpdGxlPkFkbWluPC90aXRsZT48L2hlYWQ+PGJvZHk+PGgxPkFkbWluPC9oMT48Zm9ybSBpZD0iYWRtaW4tZGVidWctZm9ybSI+PGlucHV0IG5hbWU9InVzZXIiIC8+PGlucHV0IG5hbWU9InBhc3MiIHR5cGU9InBhc3N3b3JkIiAvPjxpbnB1dCBuYW1lPSJkZWJ1ZyIgdmFsdWU9IjEiIC8+PC9mb3JtPjwvYm9keT48L2h0bWw+',
      headers: {
        'content-type': ['text/html; charset=utf-8'],
        server: ['nginx']
      },
      created_at: '2026-01-26T22:11:50.350Z'
    },
    score_result: {
      score: 0.55,
      snapshot_id: 'snap_acme_admin_v2',
      version_id: 'ver_02',
      normalized: 55,
      confidence: 0.8,
      version: 'v0.1.0',
      evidence: [
        {
          id: 'ev_debug_form',
          key: 'form_added',
          rule_id: 'form_added',
          severity: 'high',
          description: 'Debug form detected on admin page',
          value: 'admin-debug-form',
          contribution: 0.45
        }
      ],
      meta: { snapshot_id: 'snap_acme_admin_v2', url: 'https://example.com/admin' },
      timestamp: '2026-01-26T22:11:50.350Z'
    },
    diff: {
      file_path: '/admin',
      body_diff: {
        base_id: 'ver_01',
        head_id: 'ver_02',
        chunks: [
          {
            type: 'removed',
            content: 'login',
            base_start: 84,
            base_len: 5,
            head_start: 84
          },
          {
            type: 'added',
            content: 'admin-debug',
            base_start: 89,
            head_start: 84,
            head_len: 11
          },
          {
            type: 'added',
            content: ' /><input name="debug" value="1"',
            base_start: 151,
            head_start: 157,
            head_len: 32
          }
        ]
      },
      headers_diff: {}
    }
  },

  'GET /projects/targetcorp/websites/api/endpoints/details?url=https%3A%2F%2Fapi.targetcorp.com%2Fv1%2Fusers': {
    snapshot: {
      id: 'snap_tc_users_v2',
      version_id: 'ver_api_02',
      status_code: 200,
      url: 'https://api.targetcorp.com/v1/users',
      body: 'eyJ1c2VycyI6W3siaWQiOjEsImVtYWlsIjoiYWxpY2VAZXhhbXBsZS5jb20iLCJzc24iOiIqKiotKiotMTIzNCJ9XSwicGFnZSI6MX0=',
      headers: {
        'content-type': ['application/json'],
        server: ['cloudflare']
      },
      created_at: '2026-01-26T22:11:50.350Z'
    },
    score_result: {
      score: 0.32,
      snapshot_id: 'snap_tc_users_v2',
      version_id: 'ver_api_02',
      normalized: 32,
      confidence: 0.95,
      version: 'v0.1.0',
      evidence: [
        {
          id: 'ev_idor',
          key: 'sensitive_data_exposure',
          rule_id: 'sensitive_data_exposure',
          severity: 'critical',
          description: 'Endpoint appears to return sensitive fields without auth.',
          value: 'ssn',
          contribution: 0.8
        }
      ],
      meta: { snapshot_id: 'snap_tc_users_v2', url: 'https://api.targetcorp.com/v1/users' },
      timestamp: '2026-01-26T22:11:50.350Z'
    },
    diff: {
      file_path: '/v1/users',
      body_diff: {
        base_id: 'ver_api_01',
        head_id: 'ver_api_02',
        chunks: [
          {
            type: 'added',
            content: ',"ssn":"***-**-1234"',
            base_start: 45,
            head_start: 45,
            head_len: 20
          }
        ]
      },
      headers_diff: {}
    }
  },

  'GET /projects/beta/websites/analytics/endpoints/details?url=https%3A%2F%2Fanalytics.beta.io%2Fcollect': {
    snapshot: {
      id: 'snap_beta_collect_v1',
      version_id: 'ver_beta_01',
      status_code: 204,
      url: 'https://analytics.beta.io/collect',
      body: '',
      headers: {
        'content-type': ['text/plain'],
        server: ['nginx']
      },
      created_at: '2026-01-26T21:11:50.350Z'
    },
    score_result: {
      score: 0.95,
      snapshot_id: 'snap_beta_collect_v1',
      version_id: 'ver_beta_01',
      normalized: 95,
      confidence: 0.7,
      version: 'v0.1.0',
      meta: { snapshot_id: 'snap_beta_collect_v1', url: 'https://analytics.beta.io/collect' },
      timestamp: '2026-01-26T21:11:50.350Z'
    }
  }
} as const;

export type MockApiKey = keyof typeof MOCK_API_RESPONSES;

export function getMockResponse(key: MockApiKey) {
  return MOCK_API_RESPONSES[key];
}

export const MOCK_KEYS = {
  projects: 'GET /projects',

  acmeWebsites: 'GET /projects/acme/websites',
  acmeEndpoints: 'GET /projects/acme/websites/example/endpoints?limit=3',
  acmeLoginDetails: 'GET /projects/acme/websites/example/endpoints/details?url=https%3A%2F%2Fexample.com%2Flogin',
  acmeAdminDetails: 'GET /projects/acme/websites/example/endpoints/details?url=https%3A%2F%2Fexample.com%2Fadmin',

  targetcorpWebsites: 'GET /projects/targetcorp/websites',
  targetcorpApiEndpoints: 'GET /projects/targetcorp/websites/api/endpoints?limit=3',
  targetcorpUsersDetails: 'GET /projects/targetcorp/websites/api/endpoints/details?url=https%3A%2F%2Fapi.targetcorp.com%2Fv1%2Fusers',

  betaWebsites: 'GET /projects/beta/websites',
  betaEndpoints: 'GET /projects/beta/websites/analytics/endpoints?limit=3',
  betaCollectDetails: 'GET /projects/beta/websites/analytics/endpoints/details?url=https%3A%2F%2Fanalytics.beta.io%2Fcollect'
} as const;
