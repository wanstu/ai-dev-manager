# Phase 6 UAT — Dev Process / Logs / Ports

**Result:** passed by deterministic automated + independent external dogfood evidence on 2026-09-07.

No human-only acceptance item remains for the Phase 6 slice.

## Covered behavior

1. **Start and detach from launching client** — current-source detached HTTP Gateway started a real loopback dev server and returned a stable `proc_` identity. The launching MCP probe exited while the server remained reachable.
2. **Later-client status/logs/port** — a separate MCP probe observed the same process, bounded stdout/stderr tails and listening port; direct HTTP traffic succeeded on the reported port.
3. **Explicit stop** — matching writer stopped the process and the serving port was released; wrong writer was rejected.
4. **Owner cleanup** — a second server was deliberately left running; graceful `gateway stop` closed the owned process and port.
5. **Restart negative acceptance** — restarted Gateway had a new owner and an empty process list; prior `proc_` identities did not survive.
6. **Persistence boundary** — private `state.json` contained no `proc_` identities, `owned_dev_processes`, or `listening_ports` observation.
7. **Authority boundary** — forbidden executable and escaped cwd are rejected through the same Runtime policy used by short `exec`; no arbitrary PID control tool exists.
8. **Regression** — full Go tests, vet, diff-check, and repeated cross-process acceptance passed.

## Platform note

Listening-port facts are verified on the active Windows dogfood target. Non-Windows currently returns no port facts instead of exposing a generic OS scanner.

## Transition

This UAT artifact records local evidence only. Integration review passed on 2026-09-08 after the observation fix in `75e919d`; current regression and red/green evidence are in `06-INTEGRATION-REVIEW.md` and `evidence/regression.json`. Phase 6 was integrated to local master on 2026-09-08 after explicit authorization; post-integration validation passed and is recorded in `evidence/integration.json`. No automated Phase 6 completion/advance command was run, and Phase 7 remains not started.
