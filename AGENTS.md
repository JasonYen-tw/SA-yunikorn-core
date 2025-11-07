# Repository Guidelines

## Project Structure & Module Organization
Core scheduler logic lives in `pkg/`, grouped by domain (`pkg/scheduler`, `pkg/webservice`, `pkg/log`, etc.). CLI prototypes and helper binaries reside in `cmd/`. Cluster and queue templates sit under `config/`, while automation, graphs, and lint helpers live in `scripts/`. Build artifacts and coverage reports belong in `build/` (never commit). Tests are co-located with the code they exercise using the `_test.go` suffix; follow that layout when adding new packages.

## Build, Test, and Development Commands
- `make build`: compiles `simplescheduler`, `schedulerclient`, and `queueconfigchecker` into `build/`.
- `make test`: runs `go test ./...` with the race detector, deadlock tags, and writes `build/coverage.txt`.
- `make lint`: installs and runs `golangci-lint` using `.golangci.yml`; formats files via `goimports`.
- `make check_scripts`: passes every shell helper through `shellcheck`.
- `go test ./pkg/scheduler/... -run TestName`: narrow a failing suite; add `-count=1` when debugging caches.
Use `make bench` for performance work, and `make clean` before publishing artifacts to ensure a fresh `build/`.

## Coding Style & Naming Conventions
Target Go 1.21 as locked in `go.mod`. Keep tabs for indentation, run `go fmt`/`goimports` (or `make lint`) before committing, and keep package names lowercase. Exported identifiers use PascalCase, locals are lowerCamelCase, and test helpers start with `test`. Log exclusively through `pkg/log`. Assertions should rely on `gotest.tools/v3/assert` because `depguard` forbids `testify` and `logrus`.

## Testing Guidelines
Name tests `Test<Component><Scenario>` and mirror the package hierarchy for readability. Every feature PR must extend or add `_test.go` coverage, including failure paths. Run `make test` prior to pushing so the race detector stays green and `build/coverage.txt` is updated. Prefer table-driven tests; add focused benchmarks when you touch placement or scheduling hot paths.

## Commit & Pull Request Guidelines
Follow the bracketed subject convention from history (`[Feature]`, `[Bug]`, `[Refactor]`, etc.) and keep the summary <72 characters. Reference the relevant JIRA (`YUNIKORN-####`) or GitHub issue inside the body along with a short impact statement and the commands you ran. Pull requests should include: (1) change overview, (2) testing matrix, (3) screenshots or logs if behavior changes, and (4) confirmation that license headers remain intact.

## Security & Configuration Tips
Queue and placement definitions live in `config/`. After editing them, rebuild `queueconfigchecker` (`make build`) and validate with `./build/queueconfigchecker -conf config/queues.yaml`. Never commit environment-specific secrets; document overrides in deployment notes instead. Limit visibility of experimental tools by keeping prototypes inside `cmd/` until hardened.
