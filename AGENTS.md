# Repository Guidelines

## Project Structure & Module Organization
The scheduler core lives under `pkg/`, grouped by domain (for example `pkg/scheduler` for placement logic, `pkg/webservice` for REST handlers, and `pkg/log` for logging utilities). CLI prototypes and tooling binaries live in `cmd/`. Cluster configuration templates and sample queue files sit under `config/`. Automation, lint helpers, and graph generators are in `scripts/`. Build artifacts and coverage reports are written to `build/`; keep it out of version control. Tests live next to source files with the `_test.go` suffix, so prefer co-locating new cases beside the code they exercise.

## Build, Test, and Development Commands
- `make build` compiles the sample binaries (`simplescheduler`, `schedulerclient`, `queueconfigchecker`) into `build/`.
- `make test` runs `go test ./...` with race detection, deadlock tags, and writes `build/coverage.txt`.
- `make lint` downloads and runs `golangci-lint` using `.golangci.yml`.
- `make check_scripts` validates shell utilities via `shellcheck`.
- `make clean` removes Go caches and the `build/` directory; run before packaging artifacts.
Use `go test ./pkg/scheduler/... -run TestSomething` for focused investigation and `make bench` to execute benchmarks.

## Coding Style & Naming Conventions
Target Go 1.21 as defined in `go.mod`. Format all files with `go fmt`/`goimports` (run automatically by `make lint`); tabs are the default indentation. Keep package names short and lowercase, exported identifiers in PascalCase, and locals in lowerCamelCase. Log through `pkg/log` rather than third-party loggers, and stick with `gotest.tools/v3/assert` for test assertions—`depguard` blocks `testify` and `logrus`.

## Testing Guidelines
Name tests `Test<Component><Scenario>` and mirror the package structure. Ensure new logic is covered by unit tests and, when relevant, benchmarks. Always run `make test` before pushing; CI expects the race detector clean and coverage written. Add focused assertions instead of blanket `t.Fatal` to keep failure output actionable.

## Commit & Pull Request Guidelines
Recent history shows bracketed subjects such as `[Bug] json null`; follow that style for clarity (`[Feature]`, `[Refactor]`, etc.) and keep the summary under ~72 characters. Reference the relevant Apache JIRA ticket (`YUNIKORN-####`) or GitHub issue in the body, and describe observable impact and testing evidence. Pull requests should include a concise change overview, testing matrix (commands run), and links or screenshots when behavior changes. Double-check lint/test status and license headers before requesting review.

## Configuration Tips
Queue and placement definitions live in `config/`. After updating them, rebuild `queueconfigchecker` (`make build`) and run `./build/queueconfigchecker -conf config/queues.yaml` to validate. Never commit environment-specific secrets; instead, document overrides in deployment guides.
