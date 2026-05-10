import { Project, ProjectStatus, Severity, Snapshot } from '../types/project';

const generateMockHtml = (title: string, content: string) => `
<!DOCTYPE html>
<html>
<head><title>${title}</title><style>body{font-family:sans-serif;background:#1a1a2e;color:#fff;padding:2rem;line-height:1.5;} .card{background:#252545;padding:1rem;border-radius:8px;margin-top:1rem; border:1px solid #333;}</style></head>
<body>
  <h1>${title}</h1>
  <p>${content}</p>
  <div class="card">
    <h3>System Status: Operational</h3>
    <form><input type="text" placeholder="Search resources..." style="padding:0.5rem;border-radius:4px;border:none;margin-right:0.5rem;"/><button style="padding:0.5rem 1rem;background:#5170ff;color:white;border:none;border-radius:4px;cursor:pointer;">Action</button></form>
  </div>
</body>
</html>
`;

const createBasicSnapshot = (id: string, version: number, url: string, title: string): Snapshot => ({
  id: `${id}-v${version}`,
  version_id: `v${version}`,
  version: version,
  created_at: new Date(Date.now() - ( (7 - version) * 3600000 * 4)).toISOString(),
  statusCode: 200,
  url,
  body: generateMockHtml(title, `Snapshot content for version ${version}`),
  headers: { 'Content-Type': ['text/html'], 'Server': ['Cloudflare'], 'X-Frame-Options': ['SAMEORIGIN'] },
  metadata: { contentLength: 1400 + (version * 20), loadTime: 90 - version },
  scoreResult: {
    score: 0.9 + (version * 0.01),
    normalized: Math.min(90 + version, 100),
    confidence: 1.0,
    snapshot_id: `${id}-v${version}`,
    version_id: `v${version}`,
    timestamp: new Date().toISOString(),
    riskTrend: 'stable',
    evidence: [
      { id: `ev-${id}-${version}`, title: `Scan Version ${version}`, severity: 'info', description: 'Periodic baseline verification completed.', confidence: 1.0 }
    ]
  }
});

export const MOCK_PROJECTS: Project[] = [
  {
    id: 'p1',
    name: 'TargetCorp.com',
    description: 'Multi-domain monitoring for Target Corp infrastructure.',
    createdAt: new Date(Date.now() - 86400000 * 7).toISOString(),
    status: 'active',
    domains: [
      {
        id: 'd1',
        hostname: 'targetcorp.com',
        endpoints: [
          {
            id: 'e1',
            path: '/login',
            snapshots: [
              {
                id: 's1-v1',
                version_id: 'v1',
                version: 1,
                created_at: new Date(Date.now() - 86400000).toISOString(),
                statusCode: 200,
                url: 'https://targetcorp.com/login',
                body: generateMockHtml('Login v1', 'Old login page content.'),
                headers: { 'Content-Type': ['text/html'], 'Server': ['nginx'] },
                metadata: { contentLength: 1024, loadTime: 120 },
                scoreResult: {
                   score: 0.82,
                   normalized: 82,
                   confidence: 0.9,
                   snapshot_id: 's1-v1',
                   version_id: 'v1',
                   timestamp: new Date().toISOString(),
                   riskTrend: 'stable',
                   evidence: [
                     { id: 'f1', title: 'Missing CSP', severity: 'medium', description: 'Content Security Policy is not defined.', confidence: 0.9 }
                   ]
                }
              },
              {
                id: 's1-v2',
                version_id: 'v2',
                version: 2,
                created_at: new Date(Date.now() - 3600000 * 20).toISOString(),
                statusCode: 200,
                url: 'https://targetcorp.com/login',
                body: generateMockHtml('Login v2', 'Login page with tracking.'),
                headers: { 'Content-Type': ['text/html'], 'Server': ['nginx'], 'Set-Cookie': ['session=xyz; HttpOnly'] },
                metadata: { contentLength: 1150, loadTime: 145 },
                diff: {
                  filePath: '/login',
                  bodyDiff: {
                    lines: [
                      { type: 'unchanged', content: '<!DOCTYPE html>' },
                      { type: 'removed', content: '  <h1>Login v1</h1>' },
                      { type: 'added', content: '  <h1>Login v2</h1>' },
                      { type: 'modified', content: '  <p>Old login page content. (Now Updated)</p>' },
                    ]
                  },
                  headersDiff: {}
                },
                scoreResult: {
                  score: 0.78,
                  normalized: 78,
                  confidence: 0.85,
                  snapshot_id: 's1-v2',
                  version_id: 'v2',
                  timestamp: new Date().toISOString(),
                  riskTrend: 'worsened',
                  evidence: [
                    { id: 'f1', title: 'Missing CSP', severity: 'medium', description: 'Policy still missing.', confidence: 0.9 },
                    { id: 'f2', title: 'New Tracking Script', severity: 'info', description: 'Detected analytics.js addition.', confidence: 1.0 }
                  ]
                },
                securityDiff: {
                  url: '/login',
                  base_version_id: 'v1',
                  head_version_id: 'v2',
                  base_snapshot_id: 's1-v1',
                  head_snapshot_id: 's1-v2',
                  score_base: 0.82,
                  score_head: 0.78,
                  score_delta: -0.04,
                  attack_surface_changed: true,
                  attackSurfaceChanges: [
                    { type: 'added', category: 'scripts', after: '/js/analytics.js', significance: 'low' },
                    { type: 'added', category: 'cookies', after: 'session=xyz', significance: 'medium' }
                  ]
                }
              },
              {
                ...createBasicSnapshot('s1', 3, 'https://targetcorp.com/login', 'Login v3'),
                diff: {
                  filePath: '/login',
                  bodyDiff: {
                    lines: [
                      { type: 'unchanged', content: '<!DOCTYPE html>' },
                      { type: 'modified', content: '  <p>Updated UI Layout in v3</p>' },
                    ]
                  },
                  headersDiff: {}
                }
              },
              {
                ...createBasicSnapshot('s1', 4, 'https://targetcorp.com/login', 'Login v4'),
                diff: {
                  filePath: '/login',
                  bodyDiff: {
                    lines: [
                      { type: 'added', content: '  <meta name="viewport" content="width=device-width">' },
                      { type: 'unchanged', content: '  <h1>Login v4</h1>' },
                    ]
                  },
                  headersDiff: {}
                }
              },
              {
                ...createBasicSnapshot('s1', 5, 'https://targetcorp.com/login', 'Login v5'),
                diff: {
                  filePath: '/login',
                  bodyDiff: {
                    lines: [
                      { type: 'unchanged', content: 'Optimization scan v5' },
                    ]
                  },
                  headersDiff: { 'X-Content-Type-Options': 'nosniff' }
                }
              },
              {
                id: 's1-v6',
                version_id: 'v6',
                version: 6,
                created_at: new Date().toISOString(),
                statusCode: 200,
                url: 'https://targetcorp.com/login',
                body: generateMockHtml('Login v6', 'Production Ready.'),
                headers: { 
                  'Content-Type': ['text/html'], 
                  'Strict-Transport-Security': ['max-age=31536000; includeSubDomains'],
                  'X-Content-Type-Options': ['nosniff']
                },
                metadata: { contentLength: 1280, loadTime: 82 },
                diff: {
                  filePath: '/login',
                  bodyDiff: { lines: [{ type: 'unchanged', content: 'v6 Finalized' }] },
                  headersDiff: { 'HSTS': 'Added' }
                },
                scoreResult: {
                  score: 0.99,
                  normalized: 99,
                  confidence: 1.0,
                  snapshot_id: 's1-v6',
                  version_id: 'v6',
                  timestamp: new Date().toISOString(),
                  riskTrend: 'improved',
                  evidence: [{ id: 'f6', title: 'HSTS Implementation', severity: 'info', description: 'HSTS header enforced for 1 year.', confidence: 1.0 }]
                },
                securityDiff: {
                  url: '/login',
                  base_version_id: 'v5',
                  head_version_id: 'v6',
                  base_snapshot_id: 's1-v5',
                  head_snapshot_id: 's1-v6',
                  score_base: 0.97,
                  score_head: 0.99,
                  score_delta: 0.02,
                  attack_surface_changed: true,
                  attackSurfaceChanges: [{ type: 'added', category: 'headers', after: 'Strict-Transport-Security', significance: 'medium' }]
                }
              }
            ]
          },
          {
            id: 'e2',
            path: '/admin',
            snapshots: [
              createBasicSnapshot('s-admin', 1, 'https://targetcorp.com/admin', 'Admin Dashboard'),
              {
                ...createBasicSnapshot('s-admin', 2, 'https://targetcorp.com/admin', 'Admin Dashboard v2'),
                securityDiff: {
                  url: '/admin',
                  base_version_id: 'v1',
                  head_version_id: 'v2',
                  base_snapshot_id: 's-admin-v1',
                  head_snapshot_id: 's-admin-v2',
                  score_base: 0.9,
                  score_head: 0.85,
                  score_delta: -0.05,
                  attack_surface_changed: true,
                  attackSurfaceChanges: [
                    { type: 'modified', category: 'forms', before: 'login-form', after: 'admin-debug-form', significance: 'high' }
                  ]
                }
              }
            ]
          }
        ]
      },
      {
        id: 'd2',
        hostname: 'api.targetcorp.com',
        endpoints: [
          {
            id: 'e3',
            path: '/v1/users',
            snapshots: [
              createBasicSnapshot('s-api-users', 1, 'https://api.targetcorp.com/v1/users', 'API - User Directory'),
              {
                ...createBasicSnapshot('s-api-users', 2, 'https://api.targetcorp.com/v1/users', 'API - User Directory (Modified)'),
                scoreResult: {
                  score: 0.45,
                  normalized: 45,
                  confidence: 0.95,
                  snapshot_id: 's-api-users-v2',
                  version_id: 'v2',
                  timestamp: new Date().toISOString(),
                  riskTrend: 'worsened',
                  evidence: [
                    { id: 'f-idor', title: 'Potential IDOR', severity: 'critical', description: 'Endpoint returns sensitive data for unauthenticated requests.', confidence: 0.85 }
                  ]
                }
              }
            ]
          },
          {
            id: 'e4',
            path: '/health',
            snapshots: [createBasicSnapshot('s-api-health', 1, 'https://api.targetcorp.com/health', 'API Health Status')]
          }
        ]
      }
    ]
  },
  {
    id: 'p2',
    name: 'Alpha Project Internal',
    description: 'Internal infrastructure auditing.',
    createdAt: new Date(Date.now() - 86400000 * 3).toISOString(),
    status: 'idle',
    domains: [
      {
        id: 'd-p2',
        hostname: 'internal.alpha.net',
        endpoints: [
          {
            id: 'e-p2-1',
            path: '/config',
            snapshots: [createBasicSnapshot('s-alpha-config', 1, 'https://internal.alpha.net/config', 'Configuration')]
          }
        ]
      }
    ]
  },
  {
    id: 'p3',
    name: 'Beta Analytics Suite',
    description: 'Public facing analytics endpoints tracking.',
    createdAt: new Date(Date.now() - 86400000 * 12).toISOString(),
    status: 'monitoring',
    domains: [
      {
        id: 'd-p3',
        hostname: 'analytics.beta.io',
        endpoints: [
          {
            id: 'e-p3-1',
            path: '/collect',
            snapshots: [createBasicSnapshot('s-beta-collect', 1, 'https://analytics.beta.io/collect', 'Data Collection')]
          }
        ]
      }
    ]
  },
  {
    id: 'p4',
    name: 'Gamma E-commerce Platform',
    description: 'Online store frontend and checkout monitoring.',
    createdAt: new Date(Date.now() - 86400000 * 15).toISOString(),
    status: 'active',
    domains: [
      {
        id: 'd-p4',
        hostname: 'shop.gamma.com',
        endpoints: [
          {
            id: 'e-p4-1',
            path: '/checkout',
            snapshots: [
              createBasicSnapshot('s-gamma-checkout', 1, 'https://shop.gamma.com/checkout', 'Checkout Page'),
              createBasicSnapshot('s-gamma-checkout', 2, 'https://shop.gamma.com/checkout', 'Checkout Page v2')
            ]
          },
          {
            id: 'e-p4-2',
            path: '/cart',
            snapshots: [createBasicSnapshot('s-gamma-cart', 1, 'https://shop.gamma.com/cart', 'Shopping Cart')]
          }
        ]
      }
    ]
  },
  {
    id: 'p5',
    name: 'Delta Cloud Services',
    description: 'Cloud infrastructure API monitoring.',
    createdAt: new Date(Date.now() - 86400000 * 20).toISOString(),
    status: 'monitoring',
    domains: [
      {
        id: 'd-p5',
        hostname: 'api.delta-cloud.com',
        endpoints: [
          {
            id: 'e-p5-1',
            path: '/v2/instances',
            snapshots: [
              createBasicSnapshot('s-delta-instances', 1, 'https://api.delta-cloud.com/v2/instances', 'Instance Management'),
              createBasicSnapshot('s-delta-instances', 2, 'https://api.delta-cloud.com/v2/instances', 'Instance Management v2'),
              createBasicSnapshot('s-delta-instances', 3, 'https://api.delta-cloud.com/v2/instances', 'Instance Management v3')
            ]
          }
        ]
      }
    ]
  },
  {
    id: 'p6',
    name: 'Epsilon Payment Gateway',
    description: 'Payment processing endpoints security monitoring.',
    createdAt: new Date(Date.now() - 86400000 * 5).toISOString(),
    status: 'active',
    domains: [
      {
        id: 'd-p6',
        hostname: 'pay.epsilon.io',
        endpoints: [
          {
            id: 'e-p6-1',
            path: '/api/payment',
            snapshots: [
              createBasicSnapshot('s-epsilon-payment', 1, 'https://pay.epsilon.io/api/payment', 'Payment API'),
              {
                ...createBasicSnapshot('s-epsilon-payment', 2, 'https://pay.epsilon.io/api/payment', 'Payment API v2'),
                scoreResult: {
                  score: 0.95,
                  normalized: 95,
                  confidence: 0.98,
                  snapshot_id: 's-epsilon-payment-v2',
                  version_id: 'v2',
                  timestamp: new Date().toISOString(),
                  riskTrend: 'improved',
                  evidence: [
                    { id: 'f-pci', title: 'PCI Compliance', severity: 'info', description: 'Payment endpoint meets PCI DSS standards.', confidence: 0.98 }
                  ]
                }
              }
            ]
          }
        ]
      }
    ]
  },
  {
    id: 'p7',
    name: 'Zeta Social Network',
    description: 'Social media platform monitoring.',
    createdAt: new Date(Date.now() - 86400000 * 30).toISOString(),
    status: 'idle',
    domains: [
      {
        id: 'd-p7',
        hostname: 'www.zeta-social.com',
        endpoints: [
          {
            id: 'e-p7-1',
            path: '/feed',
            snapshots: [createBasicSnapshot('s-zeta-feed', 1, 'https://www.zeta-social.com/feed', 'News Feed')]
          },
          {
            id: 'e-p7-2',
            path: '/profile',
            snapshots: [createBasicSnapshot('s-zeta-profile', 1, 'https://www.zeta-social.com/profile', 'User Profile')]
          }
        ]
      }
    ]
  },
  {
    id: 'p8',
    name: 'Theta Healthcare Portal',
    description: 'HIPAA compliant patient portal monitoring.',
    createdAt: new Date(Date.now() - 86400000 * 8).toISOString(),
    status: 'active',
    domains: [
      {
        id: 'd-p8',
        hostname: 'portal.theta-health.org',
        endpoints: [
          {
            id: 'e-p8-1',
            path: '/patient/records',
            snapshots: [
              createBasicSnapshot('s-theta-records', 1, 'https://portal.theta-health.org/patient/records', 'Medical Records'),
              createBasicSnapshot('s-theta-records', 2, 'https://portal.theta-health.org/patient/records', 'Medical Records v2'),
              createBasicSnapshot('s-theta-records', 3, 'https://portal.theta-health.org/patient/records', 'Medical Records v3'),
              createBasicSnapshot('s-theta-records', 4, 'https://portal.theta-health.org/patient/records', 'Medical Records v4')
            ]
          }
        ]
      }
    ]
  },
  {
    id: 'p9',
    name: 'Iota Logistics Tracker',
    description: 'Real-time shipment tracking system.',
    createdAt: new Date(Date.now() - 86400000 * 18).toISOString(),
    status: 'monitoring',
    domains: [
      {
        id: 'd-p9',
        hostname: 'track.iota-logistics.com',
        endpoints: [
          {
            id: 'e-p9-1',
            path: '/api/shipments',
            snapshots: [
              createBasicSnapshot('s-iota-shipments', 1, 'https://track.iota-logistics.com/api/shipments', 'Shipment Tracking')
            ]
          }
        ]
      }
    ]
  },
  {
    id: 'p10',
    name: 'Kappa Financial Services',
    description: 'Banking API security monitoring.',
    createdAt: new Date(Date.now() - 86400000 * 25).toISOString(),
    status: 'active',
    domains: [
      {
        id: 'd-p10-1',
        hostname: 'api.kappa-bank.com',
        endpoints: [
          {
            id: 'e-p10-1',
            path: '/accounts',
            snapshots: [
              createBasicSnapshot('s-kappa-accounts', 1, 'https://api.kappa-bank.com/accounts', 'Account Management'),
              createBasicSnapshot('s-kappa-accounts', 2, 'https://api.kappa-bank.com/accounts', 'Account Management v2')
            ]
          }
        ]
      },
      {
        id: 'd-p10-2',
        hostname: 'secure.kappa-bank.com',
        endpoints: [
          {
            id: 'e-p10-2',
            path: '/transfer',
            snapshots: [
              createBasicSnapshot('s-kappa-transfer', 1, 'https://secure.kappa-bank.com/transfer', 'Wire Transfer'),
              {
                ...createBasicSnapshot('s-kappa-transfer', 2, 'https://secure.kappa-bank.com/transfer', 'Wire Transfer v2'),
                scoreResult: {
                  score: 0.92,
                  normalized: 92,
                  confidence: 0.96,
                  snapshot_id: 's-kappa-transfer-v2',
                  version_id: 'v2',
                  timestamp: new Date().toISOString(),
                  riskTrend: 'stable',
                  evidence: [
                    { id: 'f-2fa', title: '2FA Enforced', severity: 'info', description: 'Two-factor authentication required for transfers.', confidence: 0.96 }
                  ]
                }
              }
            ]
          }
        ]
      }
    ]
  },
  {
    id: 'p11',
    name: 'Lambda Education Platform',
    description: 'Online learning management system.',
    createdAt: new Date(Date.now() - 86400000 * 10).toISOString(),
    status: 'idle',
    domains: [
      {
        id: 'd-p11',
        hostname: 'learn.lambda-edu.com',
        endpoints: [
          {
            id: 'e-p11-1',
            path: '/courses',
            snapshots: [createBasicSnapshot('s-lambda-courses', 1, 'https://learn.lambda-edu.com/courses', 'Course Catalog')]
          },
          {
            id: 'e-p11-2',
            path: '/dashboard',
            snapshots: [createBasicSnapshot('s-lambda-dashboard', 1, 'https://learn.lambda-edu.com/dashboard', 'Student Dashboard')]
          }
        ]
      }
    ]
  },
  {
    id: 'p12',
    name: 'Mu IoT Device Manager',
    description: 'IoT device fleet monitoring and management.',
    createdAt: new Date(Date.now() - 86400000 * 22).toISOString(),
    status: 'monitoring',
    domains: [
      {
        id: 'd-p12',
        hostname: 'devices.mu-iot.net',
        endpoints: [
          {
            id: 'e-p12-1',
            path: '/api/devices/status',
            snapshots: [
              createBasicSnapshot('s-mu-status', 1, 'https://devices.mu-iot.net/api/devices/status', 'Device Status'),
              createBasicSnapshot('s-mu-status', 2, 'https://devices.mu-iot.net/api/devices/status', 'Device Status v2'),
              createBasicSnapshot('s-mu-status', 3, 'https://devices.mu-iot.net/api/devices/status', 'Device Status v3')
            ]
          }
        ]
      }
    ]
  },
  {
    id: 'p13',
    name: 'Nu Streaming Service',
    description: 'Video streaming platform CDN monitoring.',
    createdAt: new Date(Date.now() - 86400000 * 45).toISOString(),
    status: 'active',
    domains: [
      {
        id: 'd-p13-1',
        hostname: 'stream.nu-media.tv',
        endpoints: [
          {
            id: 'e-p13-1',
            path: '/player',
            snapshots: [
              createBasicSnapshot('s-nu-player', 1, 'https://stream.nu-media.tv/player', 'Video Player'),
              createBasicSnapshot('s-nu-player', 2, 'https://stream.nu-media.tv/player', 'Video Player v2')
            ]
          }
        ]
      },
      {
        id: 'd-p13-2',
        hostname: 'cdn.nu-media.tv',
        endpoints: [
          {
            id: 'e-p13-2',
            path: '/manifest.m3u8',
            snapshots: [
              createBasicSnapshot('s-nu-manifest', 1, 'https://cdn.nu-media.tv/manifest.m3u8', 'HLS Manifest'),
              createBasicSnapshot('s-nu-manifest', 2, 'https://cdn.nu-media.tv/manifest.m3u8', 'HLS Manifest v2'),
              createBasicSnapshot('s-nu-manifest', 3, 'https://cdn.nu-media.tv/manifest.m3u8', 'HLS Manifest v3')
            ]
          }
        ]
      }
    ]
  }
];