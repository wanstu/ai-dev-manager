# Phase 24 Closeout — Desktop/CLI Surface Boundary Split

Date: 2026-09-15
Opening baseline: `2b14a57`
24-01 implementation: `a96558c` (`refactor(surfaces): unify ADM connection target boundaries`)
24-02 implementation: `810d6bc` (`refactor(cli): extract command surface package`)
24-03 implementation: `9a4fea0` (`refactor(desktop): separate management surface boundaries`)
Status: COMPLETE

## Outcome

Phase 24 completes the logical separation of the CLI and Desktop presentation surfaces without splitting ADM's product model, state authority or normal management plane.

Both surfaces remain projections of the same Core/application behavior and the same Admin MCP contract. The phase changes source ownership and dependency direction only: it does not create a second state file, second management protocol, Desktop-only business authority, CLI-only product model, hidden current Environment, or writable local fallback.

The stable release names remain `adm` and `adm-desktop`.

## Delivered structure

### 24-01 — Shared ADM target semantics

`a96558c` centralized HTTP Base URL normalization, health/Agent/Admin MCP endpoint derivation and local loopback lifecycle eligibility in shared Gateway helpers. CLI and Desktop consume those helpers instead of maintaining separate target/lifecycle interpretations.

Production management paths continue to use Admin MCP, and boundary tests assert that disconnected management does not silently mutate persisted local state.

### 24-02 — CLI surface extraction

`810d6bc` moved CLI command routing, parsing/help, JSON behavior, MCP import ergonomics, Admin MCP construction and explicit local recovery commands into `internal/cli`.

`cmd/ai-dev-manager` is now process bootstrap only: executable-adjacent dotenv loading, `os.Args[1:]` delegation to `cli.Run`, final error printing and process exit status. Existing CLI acceptance tests moved with the owning package, while an executable-level smoke test proves thin-main wiring.

### 24-03 — Desktop adapter separation

`9a4fea0` split Desktop DTO, ADM connection/bootstrap, management facade and runtime facade responsibilities into focused files while retaining the existing Wails-exported methods and JSON contracts. Existing profile persistence remains separately owned in `profiles.go`.

`NewClientAdapter()` remains production-only Admin-MCP wiring. `NewAdapter(*management.Service)` remains an explicit test/offline/recovery constructor and is not an automatic fallback.

Source-level boundary tests now cover CLI/Desktop independence, Core-to-surface dependency direction, shared target semantics and release-name stability.

## Phase acceptance P24-01–P24-10

### P24-01 — One Core/state authority

PASS. No second desired-state store/cache was added. CLI and Desktop continue to operate over the same canonical Core/application state and Admin MCP management plane.

### P24-02 — Shared target semantics

PASS at `a96558c`. HTTP target normalization, health/Agent/Admin endpoint derivation and local lifecycle eligibility come from shared Gateway helpers rather than surface-local copies.

### P24-03 — Normal management remains Admin-MCP-only

PASS. Normal Workspace/Environment/exec/MCP/Skill/Memory management in both surfaces still requires a connected Admin MCP backend. Disconnect/incompatible/failure paths do not fall back to writable local state.

### P24-04 — Thin CLI executable

PASS at `810d6bc`. `cmd/ai-dev-manager` is bootstrap-only and command implementation resides in `internal/cli`, with existing commands/flags/help/JSON/error behavior preserved by the moved test suite.

### P24-05 — Desktop ownership split

PASS at `9a4fea0`. Wails-facing DTOs, connection/bootstrap, management and runtime responsibilities are separately owned while frontend bindings remain stable.

### P24-06 — Dependency direction

PASS. CLI and Desktop do not import one another. Core/application/Gateway/Admin-MCP do not import presentation packages. No repository/module split or duplicated protocol package was introduced.

### P24-07 — Existing product semantics preserved

PASS. MCP probe/status/inspect/refresh distinctions, Skill source update/refresh semantics, Environment context/temporary lifecycle safety, Memory scoping, Runtime/writer authority, optional Git behavior and explicit stable IDs remain unchanged.

### P24-08 — Desktop/browser compatibility

PASS for automated evidence. Desktop helper regression is 20/20 green and production browser smoke is 198 checks across each existing viewport/scaling configuration. The Wails production build succeeds with the standard artifact name.

Native GUI-control click-through was not newly available/performed, so existing Phase-19/22 native evidence notes remain pending rather than being converted to PASS by browser/build evidence.

### P24-09 — Fixed-head repository regression

PASS on the implementation tree committed as `9a4fea0`:

- focused surfaces/Gateway `run_5ce0d3ef6e16b33a` — PASS; Gateway 199.827s;
- Phase-23 high-risk CLI repeat x3 — PASS;
- browser smoke `run_b41fb7165ab5bf09` — PASS, 198 checks x 3 configurations;
- isolated real HTTP verifier acceptance `run_efe702b18143f636` — PASS, 148.313s;
- final full repository `run_4d6ad47c00baa399` — PASS, Gateway 219.887s;
- `go vet ./...` — PASS;
- `git diff --check` / staged diff check — PASS;
- CLI and production Wails builds — PASS.

During acceptance, one earlier full-suite run hit two timing/resource issues: the real HTTP verifier fixture's inner `go test ./...` exceeded its hard-coded 180-second test budget by about one second, and one Windows cancellation timing test failed under the same contended run. The verifier fixture budget was increased to 300 seconds without changing production timeout semantics; isolated acceptance passed. The Windows timing test passed three isolated repeats. The final non-contended full repository run passed all packages.

### P24-10 — Release identity and non-goals

PASS. Release/build names remain `adm` and `adm-desktop`. No installer/updater/signing work, remote auth/TLS, second API, second state model, task records, Planner/Executor/Reviewer policy, GSD state automation, automatic merge/push or investigation-expansion feature was added.

## Final artifacts

Built from clean implementation commit `9a4fea0`:

- `dist/adm-windows-amd64.exe`
  - size: 17,154,048 bytes
  - SHA-256: `2BA627A588A41C337AA63A0B3E0182290F55EE609941E8D056D7DA7810401D40`
- `dist/adm-desktop-windows-amd64.exe`
  - size: 17,916,416 bytes
  - SHA-256: `32653763CFC156B80AEE30A477635E86389505BFECC218C89A25D8A54AFCD42B`

These are validation artifacts only. No push, tag or release was performed.

## Preserved architecture contract

After Phase 24:

- one canonical Core/application model owns product semantics;
- one persisted desired state remains authoritative;
- normal CLI and Desktop management use Admin MCP;
- CLI owns CLI presentation behavior in `internal/cli`;
- Desktop owns Desktop presentation behavior in `internal/desktop`;
- shared Gateway target/lifecycle semantics prevent target interpretation drift;
- presentation packages do not become dependencies of Core/Gateway/Admin-MCP;
- explicit bootstrap/offline/recovery exceptions remain explicit rather than silent fallback;
- ADM remains development infrastructure, not a task/GSD orchestration engine.

## Next phase

Phase 25 — Distribution Polish If Needed — remains **Standby / conditional**. Phase 24 closeout does not open it automatically.

A future phase should be opened only from concrete dogfood evidence and an explicit planning decision.

No push, tag, release, or subagents were used during Phase 24 closeout.
