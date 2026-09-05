# External Agent Dogfood — `@adm`

v0.6 is the first release that is implementation-ready for a real external Agent connection through one stable ADM MCP Gateway.

## 1. Start one stable Gateway

Build or use the release binary, then run:

```powershell
.\ai-dev-manager.exe gateway up
.\ai-dev-manager.exe gateway status
```

The status output contains a complete Streamable HTTP MCP endpoint:

```text
http://127.0.0.1:<persisted-port>/mcp
```

The concrete port selected on the first `:0` start is persisted. Daemon restart reuses the same endpoint. ADM does not allocate one MCP endpoint per Environment.

## 2. Register one connector

Use the same external-Agent connector/MCP registration mechanism that is already used for another local MCP such as CodexPro.

Register:

```text
name: adm
transport: Streamable HTTP
url: <the endpoint returned by `gateway status`>
```

The exact UI/config syntax belongs to the external Agent product. ADM only requires that the connector runtime can reach the Gateway URL.

If the connector runtime can reach the development machine's loopback interface, use the normal loopback Gateway directly.

If the connector runs in a local Docker container, switch the same persisted Gateway port into explicit Docker dogfood mode:

```powershell
.\ai-dev-manager.exe gateway up --docker
.\ai-dev-manager.exe gateway status
```

Use the reported Docker endpoint, for example:

```text
http://host.docker.internal:41137/mcp
```

The mode survives daemon restart and keeps the same port. It uses a Host/Origin allowlist for `host.docker.internal`, but it is not an authenticated remote exposure mode; use it only for trusted local Docker dogfood.

If the connector truly executes remotely and cannot reach the development machine directly, treat that as a Gateway exposure/auth problem. Do not introduce an SSH Remote Runtime merely to solve Agent -> ADM connectivity, and do not expose the unauthenticated Docker mode as a remote/LAN service.

## 3. First connection smoke test

A newly registered `adm` connector should expose one Gateway tool surface. Start with:

```text
gateway_info
workspace_list
environment_list
```

A newly registered Workspace remains read-only by default. Before an Agent can create managed Environments, a human explicitly authorizes the minimum development policy once from the CLI:

```powershell
.\ai-dev-manager.exe workspace prepare <ws_...>
```

This keeps registration safe-by-default while enabling a machine-local minimum `standard + git` policy for managed worktrees. It does not grant arbitrary executables and the authorization is stored in the user-level Workspace Registry rather than the repository's Project config.

For a Go project, a human can additionally authorize one explicit local Go executable and add the conventional project verifier:

```powershell
.\ai-dev-manager.exe workspace prepare <ws_...> --go D:\tools\go\go1.26.5\bin\go.exe
```

The absolute Go ToolPath and `go` execution permission remain machine-local. The shareable Project config only carries `go-test = go test ./...`; an existing Project policy or existing `go-test` definition is not silently overwritten. The local `standard` authorization is a minimum, so an existing Project `full` policy is not downgraded.

Then select the prepared Workspace and create one task Environment:

```text
environment_create
  workspace_id: <ws_...>
  name: <task-name>
```

Acquire the single writer lease for that Agent/session:

```text
environment_writer_acquire
  environment_id: <env_...>
  owner: <stable-agent-or-session-owner>
```

The Agent can then use the same `environment_id` for:

```text
tree
read
search
git_status
git_diff
git_branch
write
edit
exec
run_verifier
run_verifiers
```

Read-only tools do not require the writer. Mutation/active tools require the matching `writer_owner`.

## 4. Dogfood rule

For the first real task, do not add missing capabilities pre-emptively. Record friction as evidence:

- Gateway/Agent API ergonomics
- connector reachability / exposure / auth
- Environment lifecycle
- process / dev server
- logs
- ports
- HTTP verification
- another concrete blocker

The next v0.7 phase is chosen from the first real blocker.

## 5. Review-first completion

ADM does not automatically merge, rebase, commit, squash, or cherry-pick the Environment branch. Leave the Environment and branch available for human review when the task is complete.
