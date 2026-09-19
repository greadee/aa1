# Phase 9 — Job Learning: Plan

> Authored at the **start** of the phase. Branch and folder: `ph9-joblearn`.

## Objective

Deliver `aa`'s job-learning loop: attribute completed work to roles, trades, workers, and work packages; score outcomes deterministically from evidence; generate learning candidates (patterns, pitfalls, strategies, routing hints) with provenance; enable each learned capability only behind an explicit evidence gate that requires a deterministic backtest improvement over a baseline; expose learned routing as a non-authoritative hint that always carries a deterministic fallback; and persist candidates as `CANDIDATE` memory records through a memory seam. The entire loop is testable offline and makes no model calls.

## Starting State

- Starting ref: `main @ f9063fd` (after phase 8 merge and record).
- Available:
  - `kernel/telemetry`: bounded `Record`s, deterministic `Derive` candidates, and `Summarize` aggregation.
  - `kernel/orchestrator` and `kernel/gate`: the supervised cycle, gate outcomes, and a `Sink` seam pattern.
  - `kernel/registry`: roles, trades, and workers with deterministic selection.
  - `memory/repo`: the `StrategyRepository` and the shared lifecycle state machine (`CanTransition`/`Transition`).
  - `memory/query`: the event log, `WorkHistory`, `ProjectHistory`, and `JobHistory` derived histories.
  - `contracts` v1: `MemoryRecord`, `Strategy`, `Issue`, `Telemetry`, `RouteRequest`/`RouteResponse`, `Reference`, and `Provenance`.
  - `sifter`: a deterministic, rule-based `recommend_profile` (the routing baseline) and a `MemorySink` candidate shape (`memory.py`).
  - The kernel boundary harness (`tools/archtest`): `kernel -> contracts, obsv, memory, toolbox`.
  - The `kernel/aa-joblearn.md` directive and the phase documentation conventions.
- Missing:
  - Any `joblearn` implementation; the only learning today is `kernel/telemetry.Derive`.
  - Outcome attribution and versioned scoring.
  - Deterministic similarity and conflict detection.
  - The evidence-gate registry and capability states.
  - A backtest harness and outcome estimator/recommender evaluation.
  - Learned routing hints with a guaranteed fallback.
  - A typed memory-record repository and a kernel promotion sink.
  - Deterministic learning reports.
- Known constraints:
  - `sifter` is reached over RPC, never imported; production RPC wiring is deferred. A `Baseline` seam and a deterministic fake stand in.
  - `obsv` remains scaffolded; observation evidence arrives as telemetry and history, not a live stream.
  - No model calls, no training/fine-tuning, and no embeddings in v1 (`docs/architecture/README.md` §2.2).
  - Learned signals are never authoritative; a deterministic fallback must always remain.
  - Evidence gates default to disabled; enablement requires data and a backtest win.
  - Lifecycle transitions are owned by `memory`; the kernel emits candidates only.
  - The default test suite makes no network calls; ordering derives from `Sequence`, never the wall clock.
  - The phase branch name equals the phase folder name (`ph9-joblearn`) and the phase PR exists as a Draft from phase start.

## Scope

### In Scope
- Domain types for attributed outcomes, scores, evidence, candidates, capability states, gate policies, backtest results, and routing hints.
- Deterministic outcome attribution: link attempts, results, gates, and telemetry to work packages, roles, trades, and workers.
- Versioned scoring: success, cost, duration, retries, and quality signals normalized into stable scores.
- Candidate generation with provenance and scope: patterns, pitfalls, strategies, and routing hints.
- Deterministic, feature-based similarity and conflict detection (no embeddings).
- An evidence-gate registry: per-capability policy, sample thresholds, and enable/disable decisions.
- A backtest harness comparing a candidate policy against a deterministic baseline, with a bounded dataset budget.
- Learned routing hints that return a fallback when the capability is disabled and never bypass a gate or contract.
- Persistence of candidates as `CANDIDATE` memory records through a `Sink` seam and a typed memory-record repository.
- Deterministic learning reports (data, not UI).

### Out of Scope
- Enabling learned routing in production, and the kernel↔sifter RPC wiring; a `Baseline` seam and a deterministic fake stand in.
- Model training, fine-tuning, embeddings, vector stores, and any learned similarity that requires a model.
- Automatic promotion: promotion stays explicit and evidence-gated; candidates are never auto-promoted.
- The console dashboards and any UI; reports are data structures behind a small report seam.
- Cross-project or global (workforce-level) learning beyond the project/session scope.
- Persisting raw prompts, source, or file contents; evidence stays metadata-only.

### Required End State
- [ ] Attribution links outcomes to roles, trades, workers, and work packages deterministically.
- [ ] Scoring is deterministic and versioned; the same evidence yields the same scores.
- [ ] Candidates are generated with provenance, level, and applicability.
- [ ] Similarity and conflict detection are deterministic and require no model or embedding.
- [ ] Evidence gates default to disabled and enable only on a sample threshold plus a backtest improvement.
- [ ] Backtests compare a candidate policy against a deterministic baseline.
- [ ] Learned routing returns a deterministic fallback and never bypasses a human gate or execution contract.
- [ ] Candidates persist as `CANDIDATE` through a memory seam; lifecycle transitions stay in `memory`.
- [ ] Learning reports are deterministic.
- [ ] `go build`, `go vet`, `go test`, `gofmt`, and boundary checks pass.

## Architecture Decisions

| # | Decision | Reasoning |
|---|---|---|
| ADR-P9-001 | Learning produces candidates only; it never promotes or rewrites history | `memory` owns canonical state and its lifecycle; the kernel proposes |
| ADR-P9-002 | Attribution and scoring are deterministic functions of evidence | Reproducible evaluation and testable gates; no model in the loop |
| ADR-P9-003 | Similarity is feature-based and deterministic | v1 forbids embeddings/training; determinism keeps backtests honest |
| ADR-P9-004 | Every capability sits behind an explicit evidence gate and is disabled by default | Learned behavior must never enable silently |
| ADR-P9-005 | Enablement requires enough attributed outcomes and a backtest win over a baseline | Evidence before capability |
| ADR-P9-006 | Learned routing is a hint, never authoritative; a fallback is always returned | A deterministic path must remain when the signal is absent or disabled |
| ADR-P9-007 | Learned routing never bypasses a human gate or an execution contract | Safety-critical decisions stay mechanical |
| ADR-P9-008 | Candidates carry provenance and scope (`level`, `applicability`) | Promotion and audit need to know where a claim came from and where it applies |
| ADR-P9-009 | The engine is pure; persistence and promotion go through seams | Offline determinism, and a clean `memory` boundary |
| ADR-P9-010 | Observability is deterministic data reports | Keeps the console/UI out of the tested core |

Diagrams: [plan.uml](./plan.uml).

## Slices

Each slice is independently validatable and maps to exactly one commit.

### Slice 1 — Phase plan and UML
**Goal** — Plan the phase and its intended infrastructure. **Inputs** — architecture, contracts, kernel, memory, sifter, the joblearn directive. **Expected Output** — `docs/phases/ph9-joblearn/plan.md` + `plan.uml`. **Model Class** — Strong. **Commit Message** — `add ph9 joblearn phase plan`. **Validation** — docs link check. **Dependencies** — none.

### Slice 2 — Core types and errors
**Goal** — Attribution, outcome, score, evidence, candidate, capability, and gate-policy types plus sentinel errors and a deterministic clock. **Inputs** — the joblearn directive, `contracts` v1. **Expected Output** — `kernel/joblearn/types.go`, `kernel/joblearn/errors.go`, `kernel/joblearn/clock.go` + tests. **Model Class** — General. **Commit Message** — `add joblearn core types and errors`. **Validation** — `go test ./...`. **Dependencies** — slice 1. **Documentation** — `kernel/README.md`.

### Slice 3 — Attribution and scoring
**Goal** — Attribute attempts and gate outcomes to work packages, roles, trades, and workers, and normalize evidence into versioned scores. **Inputs** — slice 2, `kernel/telemetry`, `kernel/registry`. **Expected Output** — `kernel/joblearn/attribution/*.go` + tests. **Model Class** — Strong. **Commit Message** — `add joblearn attribution and scoring`. **Validation** — `go test ./...`; determinism and scoring tests. **Dependencies** — slice 2. **Documentation** — `kernel/README.md`.

### Slice 4 — Candidate generation
**Goal** — Generate patterns, pitfalls, strategies, and routing hints from attributed outcomes, each with provenance and scope. **Inputs** — slice 3, `contracts` `MemoryRecord`/`Strategy`. **Expected Output** — `kernel/joblearn/candidates/*.go` + tests. **Model Class** — Strong. **Commit Message** — `add joblearn candidate generation`. **Validation** — `go test ./...`; provenance and scope tests. **Dependencies** — slice 3. **Documentation** — `kernel/README.md`.

### Slice 5 — Similarity and conflict
**Goal** — Cluster attributed outcomes deterministically by features and detect conflicting candidates for the same scope. **Inputs** — slices 3–4. **Expected Output** — `kernel/joblearn/similarity/*.go` + tests. **Model Class** — Strong. **Commit Message** — `add joblearn similarity and conflict`. **Validation** — `go test ./...`; order-independence and conflict tests. **Dependencies** — slice 4. **Documentation** — `kernel/README.md`.

### Slice 6 — Evidence gates
**Goal** — A capability registry with per-capability policy, sample thresholds, baseline/backtest requirements, and a default-disabled state. **Inputs** — slices 3–5. **Expected Output** — `kernel/joblearn/gate/*.go` + tests. **Model Class** — Strong. **Commit Message** — `add joblearn evidence gates`. **Validation** — `go test ./...`; disabled-by-default and enablement tests. **Dependencies** — slice 5. **Documentation** — `kernel/README.md`.

### Slice 7 — Backtest evaluation
**Goal** — Compare a candidate policy against a deterministic baseline over a bounded dataset, and produce an evaluation report the gate consumes. **Inputs** — slices 3–6, the sifter recommender baseline. **Expected Output** — `kernel/joblearn/backtest/*.go` + tests and a benchmark. **Model Class** — Strong. **Commit Message** — `add joblearn backtest evaluation`. **Validation** — `go test ./...`; baseline, budget, and determinism tests. **Dependencies** — slice 6. **Documentation** — `kernel/README.md`.

### Slice 8 — Routing hints
**Goal** — Expose learned routing as a non-authoritative hint that always returns a deterministic fallback and never bypasses a gate or contract. **Inputs** — slices 3, 6, 7. **Expected Output** — `kernel/joblearn/route/*.go` + tests. **Model Class** — General. **Commit Message** — `add joblearn routing hints`. **Validation** — `go test ./...`; fallback and no-bypass tests. **Dependencies** — slice 7. **Documentation** — `kernel/README.md`.

### Slice 9 — Memory promotion wiring
**Goal** — Persist candidates as `CANDIDATE` records through a `Sink` seam and a typed memory-record repository, with explicit lifecycle promotion. **Inputs** — slices 4, 6. **Expected Output** — `memory/repo/memory.go` (typed `MemoryRecord` repository) + `kernel/joblearn/promote/*.go` + tests. **Model Class** — General. **Commit Message** — `add joblearn memory promotion wiring`. **Validation** — `go test ./...`; candidate-only, idempotency, and transition tests. **Dependencies** — slice 8. **Documentation** — `kernel/README.md`, `memory/README.md`.

### Slice 10 — Phase summary and UML
**Goal** — Record the as-built phase. **Inputs** — all prior slices. **Expected Output** — `summary.md` + `summary.uml` + updated `kernel/README.md`. **Model Class** — Strong. **Commit Message** — `add ph9 joblearn phase summary`. **Validation** — link check. **Dependencies** — slice 9.

## Exit Criteria

- [ ] Attribution and scoring are deterministic and versioned.
- [ ] Candidates carry provenance and scope.
- [ ] Similarity and conflict detection are deterministic and model-free.
- [ ] Capabilities are disabled by default and enable only on evidence.
- [ ] Backtests compare against a deterministic baseline within budget.
- [ ] Learned routing always returns a fallback and never bypasses a gate or contract.
- [ ] Candidates persist as `CANDIDATE`; promotion stays explicit and lifecycle-checked.
- [ ] Reports are deterministic.
- [ ] `go build`, `go vet`, `go test`, `gofmt`, and boundary checks pass.
- [ ] Phase PR merged to `main`; branch retained.

## Test Plan

| Layer | What is tested |
|---|---|
| unit | attribution, scoring, candidate generation, similarity, conflict, gates, backtest, routing fallback |
| contract | `MemoryRecord`/`Strategy` candidate conformance; `Telemetry` conformance; result-envelope conformance |
| integration | telemetry -> attribution -> candidates -> gate -> promotion (fake sink); backtest -> enable -> routing |
| security | no model calls; metadata-only evidence; gates and contracts never bypassed; kernel never imports sifter/sync/forge |
| determinism | stable attribution, scores, candidates, gate decisions, and reports, independent of input order |
| performance | backtest over a large synthetic dataset within a budget (benchmark) |
| boundary | `kernel` imports only contracts, obsv, memory, toolbox (archtest) |
