// This directory is a JavaScript/TypeScript app, not part of the Go module.
// Declaring a nested module here keeps `go list ./...` (and therefore build,
// vet, test, lint, and coverage) from descending into node_modules, which can
// otherwise vendor stray .go files (e.g. flatted/golang) into the build graph.
module github.com/raysh454/moku/frontend

go 1.26
