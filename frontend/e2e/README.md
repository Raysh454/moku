# End-to-end smoke specs

Playwright specs that drive the revamped GUI against a real, seeded backend.
They are intentionally separate from the unit tests: `npm run test` (vitest)
only collects `*.test.*`, while these are `*.spec.ts`.

## Prerequisites

Three processes must be running (the same stack as `DEMO.md`):

```bash
# 1. API server (loopback :8080)
MOKU_ALLOW_PRIVATE_HOSTS=1 go run .

# 2. Demo target site (:9999)
go run ./cmd/demoserver
```

Playwright starts the Vite dev server (:3000) itself.

## Run

```bash
cd frontend
npx playwright install chromium   # first time only
npm run test:e2e                  # or: npm run test:e2e:ui
```

`global.setup.ts` seeds a two-version `e2e-demo` project through the API
(enumerate → fetch → bump-all → fetch) and warms the lazy, Shiki-heavy
workspace chunk (the dev server compiles it on first navigation, which can
take a few minutes the first time). After setup the specs run in seconds.

Screenshots and traces land in `e2e/.artifacts/` (gitignored).

## CI note

In CI, prefer running against a production build (`npm run build` + a static
server) so the workspace chunk is precompiled — the API client targets
`http://localhost:8080` directly in production builds.
