# ADM v1.1 Agent Gateway / MCP 工具参考

本文面向把 AI Agent、IDE Agent 或其他 MCP client 接到 ADM 的开发者。这里描述 `/mcp` Agent surface 的工具语义、推荐调用顺序，以及它和 `/admin/mcp` 的权限差异。

人类 CLI 使用方式见 [USER_GUIDE.md](USER_GUIDE.md) 与 [cli.md](cli.md)。内部状态/权限机制见 [ARCHITECTURE.md](ARCHITECTURE.md)。

## 1. 连接地址

默认 HTTP Gateway：

```text
Base URL:   http://127.0.0.1:43137
Agent MCP:  http://127.0.0.1:43137/mcp
Admin MCP:  http://127.0.0.1:43137/admin/mcp
Health:     http://127.0.0.1:43137/healthz
```

ADM HTTP Gateway 默认监听 loopback。显式使用非 loopback listen 时，服务端必须先配置 Host allowlist、Admin API Key 与 Agent API Key；`/admin/mcp` 和 `/mcp` 使用不同 Key。公网场景仍应通过 HTTPS 反向代理、VPN/防火墙等限制暴露面。

也可以使用：

```text
adm gateway stdio
```

由支持 stdio MCP transport 的客户端作为 child process 自动启动。它不是给人手动打开的普通 terminal mode。

## 2. Agent 的基本规则

### 2.1 永远使用稳定 ID

Agent 不应把 shell cwd 当成授权依据，也不存在“当前 Environment”。

常用 stable IDs：

```text
ws_...       Workspace
env_...      Environment
mcp_...      global MCP
skill_...    Skill
vf_...       Verifier definition
proc_...     Gateway-owned process
run_...      async single-command Run
vfrun_...    async verifier Run
```

每个 project operation 都显式带 `environment_id`。

### 2.2 读与写分开

通常：

- tree/read/search/inspect/list/status 不需要 Writer；
- write/edit/delete/exec/process start-stop/run start-cancel/verifier run 需要 matching Writer。

### 2.3 稳定 Writer owner

一次 Agent 工作会话应使用稳定 owner，例如：

```text
agent-myclient-session-42
```

不要每个 tool call 都生成新 owner，否则无法续租和释放同一个 lease。

### 2.4 不要把 optional capability 当全局 prerequisite

如果 `git_status` 失败，因为目录不是 Git repo，Agent 应继续使用 files/exec 等可用能力。

如果某个 MCP/Skill broken，也不应该停止整个 Environment 的普通开发。

## 3. 推荐 Agent 启动流程

一个通用、低副作用的启动流程：

```text
1. gateway_info
2. workspace_list / environment_list（如果调用方尚不知道 ID）
3. environment_injection_plan(environment_id)
4. environment_context_bundle(environment_id)
5. 需要更细时 environment_inspect / environment_capability_report
6. tree / read / search（隐式 reader；无需 reader/writer lease，也不要为了读代码先 acquire writer）
7. 只有首次 mutation / execution 明确要求 writer_owner 时才 environment_writer_acquire
8. write / edit / exec / verifier / process / run
9. environment_writer_release
```

`environment_injection_plan` 先回答“哪些 MCP/Skill 被选中、来源是什么、当前能否注入、下一步调用什么”；`environment_context_bundle` 再提供 bounded root/tree/capability/MCP/Skill/verifier guidance。两者都不会主动连接 MCP、读取完整 Skill 正文或扩大权限。

## 4. Gateway 基础信息

### `gateway_info`

用途：确认连接的是 ADM、surface 类型、runtime owner 基本信息和核心语义。

输入：空对象。

副作用：无。

## 5. Workspace 工具

Agent surface 只允许发现/查看已登记 Workspace，不允许 Agent 任意向主机添加新目录 registration。

### `workspace_list`

列出所有已登记 Workspace。

输入：空。

Writer：不需要。

### `workspace_inspect`

输入：

```json
{
  "workspace_id": "ws_xxx"
}
```

按 stable ID 读取 Workspace metadata。

### `workspace_discover`

在已登记 Workspace 内进行 bounded metadata-only project discovery。

输入示例：

```json
{
  "workspace_id": "ws_xxx",
  "path": "",
  "query": "api",
  "max_depth": 4,
  "max_entries": 2000,
  "max_candidates": 50,
  "max_digest_entries": 200,
  "max_output_bytes": 131072
}
```

`0`/omitted budget 使用 Core 默认值。

不会：

- 读取文件正文；
- 创建 Environment；
- 要求 Git；
- 要求 Writer；
- 启动 MCP/Skill/verifier。

## 6. Environment 查看与上下文

### `environment_list`

返回轻量 Environment summaries，包括 identity/root/writer/selection IDs/private memory count 等安全事实。

不返回 private Memory values。

### `environment_inspect`

输入：

```json
{
  "environment_id": "env_xxx"
}
```

返回更完整 management/development view：

- Workspace metadata；
- capability facts；
- resolved MCP/Skill；
- unresolved selected IDs；
- private Memory count。

### `environment_tree_digest`

输入：

```json
{
  "environment_id": "env_xxx",
  "path": "src",
  "max_depth": 3,
  "max_entries": 1000,
  "max_candidates": 50,
  "max_digest_entries": 100,
  "max_output_bytes": 65536
}
```

只扫 Environment root authority 下的 metadata，不会扩到 Workspace sibling。

### `environment_capability_report`

输入：

```json
{
  "environment_id": "env_xxx"
}
```

返回 authoritative `CapabilityReport`：available/unavailable + reason/evidence。

有 Runtime Owner 时可加入**已有** MCP/process/run observation，但不会为了 report 主动：

- reconnect；
- probe；
- call MCP business tool；
- run verifier；
- acquire Writer。

### `environment_injection_plan`

输入：

```json
{
  "environment_id": "env_xxx"
}
```

返回当前 Environment 对 Agent 的 canonical passive MCP / Skill 注入计划。只包含最终 selected 的 MCP / Skill，并明确返回：

- `selected`：当前确实被选择；
- `selection_sources`：`environment` / `workspace`；
- `default_include_in_environment`：当前 catalog default flag，仅作事实展示；
- `injectable`：当前 passive 检查下是否允许进入下一步；
- `state / reason_code / message`：不可注入时的原因；
- `next_action`：下一步应调用 `environment_mcp_tools`、`environment_skill_read`，还是先 inspect。

生成 plan 不会主动连接 MCP、读取 Skill 正文、获取 Writer，也不会修改 selection。Agent 不应把 `selected=true` 直接解释为“当前可用”，应以 `injectable` 为准。

### `environment_context_bundle`

输入示例：

```json
{
  "environment_id": "env_xxx",
  "path": "",
  "max_depth": 3,
  "max_entries": 1000,
  "max_digest_entries": 80,
  "max_output_bytes": 65536
}
```

返回 bounded context：

- identity；
- root/tree digest；
- capability summary/issues；
- MCP safe summary；
- Skill safe summary；
- verifier summary；
- operation guidance；
- omission/truncation evidence。

不会自动读取：

- Global Memory values；
- Environment-private Memory values；
- 完整 Skill instructions。

也不会主动执行/连接任何能力。

## 7. Writer 工具

### `environment_writer_acquire`

```json
{
  "environment_id": "env_xxx",
  "owner": "agent-session-42"
}
```

获取或同 owner 续租 Writer。

Writer 是 physical root 级单写者，所以另一个 Environment 如果指向同一个 canonical root，也会冲突。

### `environment_writer_heartbeat`

同样输入，只续租，不做 mutation。

### `environment_writer_release`

正常：

```json
{
  "environment_id": "env_xxx",
  "owner": "agent-session-42"
}
```

工具 schema 也支持 `force`，但 force 是 recovery fallback。普通 Agent 工作流不应该用 force 抢占别人的 Writer。

## 8. 文件工具

### `tree`

```json
{
  "environment_id": "env_xxx",
  "path": "src",
  "max_depth": 3,
  "max_entries": 500
}
```

只读，不需要 Writer。

### `read`

完整读取：

```json
{
  "environment_id": "env_xxx",
  "path": "src/main.go",
  "max_bytes": 65536
}
```

按行读取（1-based，首尾都包含）：

```json
{
  "environment_id": "env_xxx",
  "path": "src/main.go",
  "start_line": 120,
  "end_line": 180,
  "max_bytes": 65536
}
```

`start_line` / `end_line` 都是可选的；只给 `start_line` 时读取到 EOF，只给 `end_line` 时从第 1 行开始。范围读取只用 `max_bytes` 限制选中片段，因此可以从大文件中读取一个小片段，而不要求整个文件小于 `max_bytes`。完整读取保持原有行为。

### `compare_files`

对比两个文本文件片段，并返回 bounded unified line diff：

```json
{
  "environment_id": "env_xxx",
  "left": {
    "path": "old/service.go",
    "start_line": 120,
    "end_line": 180
  },
  "right": {
    "path": "new/service.go",
    "start_line": 135,
    "end_line": 205
  },
  "max_bytes": 262144,
  "max_output_bytes": 262144
}
```

`left` / `right` 可以是不同文件，也可以是同一个文件的不同区间。行号是 1-based 且首尾包含；省略 `end_line` 表示读取到 EOF。工具完全在 ADM 内部做文本行对比，不依赖系统额外安装 `diff`，也不需要 Writer。

### `project_analyze`

对当前 Environment 根目录做 bounded 静态项目分析，并刷新项目总览和机器索引：

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "max_files": 4000,
  "max_symbols": 1200
}
```

当前 MVP 重点支持 Go / PHP：识别 `go.mod` / `composer.json`、主要源码目录，以及 Go type/func/method 和 PHP class/interface/trait/enum/function 大纲。分析不会执行项目代码，并跳过 `.git`、`.adm`、`vendor`、`node_modules`、`dist`、`build` 等常见依赖或生成目录。

生成：

- `.adm/project-overview.md`：给人和 Agent 快速建立项目心智模型的 bounded 摘要。
- `.adm/index/manifest.json`：索引 schema、统计、bounds、生成时间和 artifact SHA-256。
- `.adm/index/files.jsonl`：项目文件清单；Go/PHP 源文件附 language / package-or-namespace / SHA-256。
- `.adm/index/symbols.jsonl`：type/class/func/method 等定义到文件和行号的机器索引。

`project-overview.md` 不再承担完整索引职责；精确查找应优先使用 `.adm/index`。manifest 最后写入，且记录 artifact hashes，消费者可以检测中途失败产生的 partial/stale index。

因为会写入 `.adm`，需要 matching Writer。返回结果只包含概要统计和 artifact 路径，不把 Markdown / JSONL 正文塞进 MCP 响应；Agent 后续可用 `read` / `search` 对索引做 bounded 查询。


### `code_intelligence_info`

查看当前 Environment 使用的 Code Intelligence Provider 及能力，不需要 Writer：

```json
{
  "environment_id": "env_xxx"
}
```

当前默认 Provider 是 `adm_static_index`，能力为 definitions=true、references=false、hierarchy=false，数据来源 `.adm/index`。上层工具通过 Provider 接口调用；未来接入 PhpStorm/JetBrains Provider 时，可以在不改 Agent 查询流程的前提下增加 references / hierarchy 等能力。

ADM 将两类 JetBrains 能力分开识别：`PhpStorm Code Intelligence` / `JetBrains Code Intelligence` 表示实现 ADM Contract v1 的外部 Provider；官方 JetBrains IDE 自带 MCP（例如 `PhpStorm` / `JetBrains` / `IDEA` / `WebStorm` / `GoLand` / `Rider`）单独标记为 `jetbrains_native`。Contract-v1 Provider 识别 `code_intelligence_info/query/status/references/hierarchy`；JetBrains native 当前只识别官方只读语义工具 `search_symbol`、`get_symbol_info`、`analyze_calls`，不会把 rename/build/terminal 等工具算作 Code Intelligence 能力。当前自动 `code_intelligence_*` 路由仍只消费 Contract v1；`jetbrains_native` 先作为可观测候选 Provider 展示，待真实 IDE 返回结构适配完成后再接入 generic query。

JetBrains native 适配还会在显式 Provider 检查时通过现有 `tools/list` 按需读取完整 schema，不把 schema 持久化进 Gateway observation。当前 `search_symbol` gate 要求 input schema 至少包含 `q`、`projectPath`、`limit`，且 `q` 为 required；同时单独记录是否存在 MCP `OutputSchema`。即使 input/output schema 都满足检查，当前 `auto_route_enabled` 仍为 false，因此 schema 检查不会调用 `search_symbol`，也不会改变实际 Provider。

Provider 选择保持保守：只有 Environment 已授权、Gateway 已观察为 healthy、tool inventory 明确包含本次所需只读 tool，并且 `code_intelligence_info` 的 `adm.code_intelligence` Contract v1 negotiation 成功时，generic `code_intelligence_*` 才会调用 PhpStorm MCP。Provider 调用失败、返回 tool error、缺失 structured content、结构不兼容或协议版本不匹配时，ADM 会回退 `adm_static_index`；返回中的 `provider` 表示实际使用者，`attempted_provider` / `fallback_reason` 说明发生过的回退。单纯查看 capability report 不会执行项目代码。外部 Provider 的完整实现契约见 `docs/CODE_INTELLIGENCE_PROVIDER.md`。

### `code_intelligence_query`

Provider-neutral 的 symbol definition 查询。参数与 `project_index_query` 相同，但返回中额外包含当前 `provider`：

```json
{
  "environment_id": "env_xxx",
  "query": "Service.Run",
  "path": "internal/service",
  "kind": "method",
  "language": "Go",
  "exact": false,
  "max_results": 50
}
```

Agent 新代码优先调用这一工具。默认使用 `adm_static_index`；若 healthy 的 Environment-authorized PhpStorm MCP 提供 `code_intelligence_query`，则自动路由到 PhpStorm。传给外部 Provider 的参数除查询条件外还包含 `environment_id` 和 `project_root`。外部 Provider 必须通过 MCP `structuredContent` 返回与 ADM symbol query result 兼容的结构；ADM 不依赖自由文本 JSON 猜测结果。

### `code_intelligence_status`

Provider-neutral 的 freshness / health 查询：

```json
{
  "environment_id": "env_xxx",
  "max_changes": 50
}
```

返回当前实际使用的 `provider` 以及 provider-specific `result`。对 `adm_static_index`，状态语义与 `project_index_status` 相同；healthy PhpStorm MCP 若提供 `code_intelligence_status`，则会优先调用它。外部 Provider 同样必须返回 structured result。

### `code_intelligence_references`

查询某个符号的引用位置。输入使用稳定的 symbol locator，而不是只传一个自由文本名称：

```json
{
  "environment_id": "env_xxx",
  "symbol": {
    "path": "src/Foo.php",
    "line": 17,
    "name": "Foo",
    "qualified_name": "App\\Foo",
    "kind": "class",
    "language": "PHP"
  },
  "max_results": 100
}
```

`symbol` 至少需要 `path` / `name` / `qualified_name` 之一；`max_results` 默认 100、上限 500。外部 Provider 会额外收到 `project_root`，结果必须通过 MCP `structuredContent` 返回 references 数组，每项可包含 path、line、column、kind、qualified_name 和 bounded context。

当前 `adm_static_index` 明确标记 `references=false`，因此没有可用外部 Provider 时不会伪造静态引用结果，而是返回：

```json
{
  "provider": {"provider_id": "adm_static_index"},
  "available": false,
  "reason": "provider_capability_unavailable"
}
```

### `code_intelligence_hierarchy`

查询类型/符号的父子层级：

```json
{
  "environment_id": "env_xxx",
  "symbol": {
    "path": "src/Foo.php",
    "line": 17,
    "qualified_name": "App\\Foo"
  },
  "direction": "both",
  "max_depth": 2,
  "max_results": 100
}
```

`direction` 仅允许 `parents` / `children` / `both`，默认 `both`；`max_depth` 默认 2、上限 8；`max_results` 默认 100、上限 500。structured result 使用 nodes + edges 表达层级，edge kind 由 Provider 返回，例如 `extends` / `implements`。

当前 `adm_static_index` 明确标记 `hierarchy=false`。没有 capable Provider 时同样返回 `available=false`；如果已尝试 PhpStorm Provider 但调用失败或返回结构不兼容，会额外返回 `attempted_provider` 和对应 `reason`。

### `project_index_query`（兼容别名）

只读查询由 `project_analyze` 生成的 `.adm/index/symbols.jsonl`。不需要 Writer；查询前会用 `manifest.json` 中的 SHA-256 校验 symbol index，避免读取半生成或被修改的索引。

```json
{
  "environment_id": "env_xxx",
  "query": "Service.Run",
  "path": "internal/service",
  "kind": "method",
  "language": "Go",
  "exact": false,
  "max_results": 50
}
```

`query` 会按 qualified-name exact、name exact、qualified suffix、prefix、contains 的顺序排序；`path` 是不区分大小写的子串过滤，`kind` / `language` 是精确过滤。`query` 可以省略，但至少要提供 `path` / `kind` / `language` 中一个。

`project_index_query` 仍只覆盖 built-in static symbol definitions；references / hierarchy 统一走上面的 provider-neutral `code_intelligence_*` 工具。

### `project_index_status`（兼容别名）

检查 `.adm/index` 是否仍然能代表当前源码，不需要 Writer：

```json
{
  "environment_id": "env_xxx",
  "max_changes": 50
}
```

状态：

- `fresh`：artifact hash 正确，且当前项目文件与索引一致。
- `stale`：检测到新增、删除或修改的项目文件；返回 bounded changed paths。
- `partial`：索引生成或 freshness scan 触及 `max_files` 等 bounds，未发现变化但不能声称完整新鲜。
- `missing`：尚未生成索引。
- `invalid`：manifest/schema/artifact hash 等索引完整性检查失败。

`project_index_query` 只做快速 symbol 查询和 artifact 完整性校验，不会每次重新扫描整个项目；需要判断源码是否变化时显式调用 `project_index_status`，`stale/partial/invalid` 时再运行 `project_analyze` 刷新索引。

### `search`

```json
{
  "environment_id": "env_xxx",
  "path": "src",
  "query": "TODO",
  "max_files": 500,
  "max_matches": 100,
  "max_bytes_per_file": 1048576
}
```

是 literal search，不是 regex/semantic search。

### `write`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "path": "src/new.go",
  "content": "package main\n",
  "create_parents": true
}
```

需要 matching Writer。

### `edit`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "path": "src/main.go",
  "old_text": "old",
  "new_text": "new",
  "expected_replacements": 1
}
```

精确文本替换。`expected_replacements` 可用于避免“本来想改 1 处却改了 N 处”。

### `delete`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "path": "src/obsolete.go"
}
```

只删除一个文件；Agent surface 不暴露递归目录删除。

## 9. `exec`：同步本机命令

输入：

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "executable": "go",
  "args": ["test", "./..."],
  "cwd": "",
  "timeout_ms": 120000,
  "max_output_bytes": 262144
}
```

必须同时满足：

- Writer matching；
- executable 满足当前 execution policy：不在 command blacklist，且在 Strict 模式下位于 global allowlist（Full Authorization 可放行未列入 allowlist 的 executable）；
- cwd 在 Environment 内；
- managed worktree（若有）identity 仍有效。

`cwd` 是 Environment-relative。不要传任意主机 cwd。

适合短/中等时长命令。长命令可以考虑 `run_start`，结构化 test/build 可以考虑 async verifier。

## 10. Verifier 工具

Agent 可以运行已有 Verifier definition，但不能通过 Agent surface 添加/删除 definition。

### `environment_verifier_list`

```json
{
  "environment_id": "env_xxx"
}
```

只读。

### `environment_verifier_run`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "verifier_id": "vf_xxx",
  "max_output_bytes": 262144
}
```

同步阻塞到 verifier 完成。

### `environment_verifier_run_start`

同样主要参数，立即返回 owner-local verifier run identity。

适合长 test/build。

### `environment_verifier_run_list`

按 Environment 列 `vfrun_`。

Writer：不需要。

### `environment_verifier_run_status`

```json
{
  "environment_id": "env_xxx",
  "verifier_run_id": "vfrun_xxx"
}
```

读取 bounded live output / terminal structured verifier result。

### `environment_verifier_run_cancel`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "verifier_run_id": "vfrun_xxx"
}
```

必须是当前 matching Writer，而且是启动该 verifier run 的 writer owner。

`vfrun_` 不跨 Gateway restart 持久化。

## 11. Long-running Process

### `process_start`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "executable": "node",
  "args": ["server.js"],
  "cwd": "app",
  "max_log_bytes": 131072
}
```

返回 `proc_...`。

要求 Writer + execution policy；command blacklist 在 Strict / Full Authorization 下都优先拒绝。

### `process_list`

列一个 Environment 下当前 Runtime Owner 持有的 processes。

### `process_status`

```json
{
  "environment_id": "env_xxx",
  "process_id": "proc_xxx"
}
```

可包含 owned process 状态/listening port facts。

### `process_logs`

同上输入，读取 bounded stdout/stderr tails。

### `process_stop`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "process_id": "proc_xxx"
}
```

只允许停止 Gateway-owned stable `proc_`，不能传任意 OS PID。

## 12. Async Agent Run

Run 是一个异步单命令资源，不是任务编排。

### `run_start`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "executable": "go",
  "args": ["test", "./..."],
  "cwd": "",
  "timeout_ms": 600000,
  "max_output_bytes": 262144
}
```

返回 `run_...`。

### `run_list`

列 Environment 的 owner-local Runs。

### `run_status`

```json
{
  "environment_id": "env_xxx",
  "run_id": "run_xxx"
}
```

状态：`running` / `succeeded` / `failed` / `canceled`。

### `run_cancel`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "run_id": "run_xxx"
}
```

Run observation 不持久化、restart 不 resume。

## 13. Git tools

### `git_status`
### `git_diff`
### `git_branch`

都使用：

```json
{
  "environment_id": "env_xxx"
}
```

Git 是 optional capability。非 Git root 返回本地能力错误是正常情况，Agent 不应该因此停止普通开发。

## 14. Managed worktree tools

Agent 的普通任务隔离应使用 `environment_temporary_create(mode=managed_worktree)`，让新 Environment / worktree 带 owner + TTL，并在任务结束后 cleanup；需要长期保留时再 promote。

### `environment_worktree_create`（Admin-only）

这是显式创建 **durable** managed worktree 的管理工具，不再暴露给普通 Agent，避免每个开发任务都永久堆积 Environment / worktree。


```json
{
  "source_environment_id": "env_source",
  "name": "isolated-work",
  "branch_name": "fix/check-ref",
  "base_ref": "origin/dev",
  "migrate_uncommitted_changes": true
}
```

也可以用 Git top-level `workspace_id` 代替 `source_environment_id`，两者只能提供一个。

Agent 决定 `branch_name` 的业务语义和 `base_ref`；ADM 决定物理 destination，并把当前配置的 `branch_prefix` 加到 branch fragment 前。`branch_prefix` 会规范化为以 `/` 结尾，因此配置 `agent` 或 `agent/`、Agent 传 `fix/check-ref`，最终 branch 都是 `agent/fix/check-ref`。

`branch_name` 只允许 ASCII 字母、数字及 `/ - _ + .`，还会经过 Git ref 校验。空 segment、`..`、`.lock`、空格、中文和 Git 特殊字符都会被拒绝。

`migrate_uncommitted_changes=true` 时，ADM 会把 source checkout 的 staged、unstaged、delete、rename 以及 untracked 非 ignored 文件迁移到新 worktree；ignored 文件不迁移。若这些变动不能安全应用到新的 `base_ref`，创建整体失败并回滚。

Managed worktree 是 create-once 资源：不要 reset/sync/reuse 旧 worktree 来承载新任务。新任务从新的 `base_ref` 再创建一个 managed worktree。Agent 也不应通过 `exec` 调用 `git worktree`；ADM 会拒绝并要求使用 managed-worktree tools。
### `environment_worktree_list`

列 persisted ADM-managed worktrees。

### `environment_worktree_destroy`

```json
{
  "environment_id": "env_xxx",
  "writer_owner": "agent-session-42",
  "force": false
}
```

默认拒绝 dirty/unpublished work。generic managed destroy 有 explicit `force` recovery 选项，但即使 force，generated branch 仍保留。

不要把这个 force 与 temporary cleanup 混淆；temporary lifecycle **没有 force**。

## 15. Temporary Environment tools

Temporary Environment 是 lifecycle infrastructure，不是 task object。

### `environment_temporary_create`

existing root：

```json
{
  "workspace_id": "ws_xxx",
  "name": "temp-task",
  "owner_id": "task-owner-42",
  "ttl_seconds": 3600,
  "session_id": "session-42",
  "run_id": "external-run-provenance",
  "mode": "existing_root",
  "root": "D:\\projects\\app"
}
```

managed worktree：

```json
{
  "workspace_id": "ws_xxx",
  "name": "temp-isolated",
  "owner_id": "task-owner-43",
  "ttl_seconds": 7200,
  "mode": "managed_worktree",
  "branch_name": "task/fix-example",
  "base_ref": "HEAD"
}
```

`session_id` / `run_id` 只做 provenance，不会自动启动 Run，也不会建立 task parent-child semantics。

### `environment_temporary_status`

```json
{
  "environment_id": "env_xxx"
}
```

只读，无 owner 要求。显示 expiry/retention/blockers。

### `environment_temporary_promote`

```json
{
  "environment_id": "env_xxx",
  "owner_id": "task-owner-42"
}
```

要求 lifecycle owner 匹配。只把 retention 改 durable。

### `environment_temporary_cleanup`

Preview：

```json
{
  "environment_id": "env_xxx",
  "owner_id": "task-owner-42"
}
```

Execute：

```json
{
  "environment_id": "env_xxx",
  "owner_id": "task-owner-42",
  "execute": true
}
```

执行前 fresh recheck。没有 `force` 参数。

active writer/MCP/process/run/vfrun、owner mismatch、not expired、dirty/unpublished/tampered managed worktree 等都会成为 blocker。

## 16. Environment-authorized MCP

### `environment_mcp_status`

```json
{
  "environment_id": "env_xxx",
  "mcp_id": "mcp_xxx"
}
```

一次性 Environment-aware configured/disabled/healthy/error probe。

### `environment_mcp_inspect`

相同 IDs。

读取 sanitized desired config + **已有** owner observation/tool inventory。

不会 connect/Ping/refresh。

### `environment_mcp_refresh`

相同 IDs。

显式 discard old session/observation，然后 reconnect/Ping/list tools。

不会调用 business tool，不修改 definition/selection。

### `environment_mcp_tools`

相同 IDs。列该 Environment 已启用、可用 MCP 的 tool definitions。

### `environment_mcp_call`

```json
{
  "environment_id": "env_xxx",
  "mcp_id": "mcp_xxx",
  "tool": "some_tool",
  "arguments": {
    "key": "value"
  }
}
```

只有 Environment selection + runtime activation 满足时允许。

ADM 不自动 replay 失败的 business call。

## 17. Environment-authorized Skill

### `environment_skill_list`

```json
{
  "environment_id": "env_xxx"
}
```

返回 Environment-specific availability；不会读取 Skill 正文。

### `environment_skill_inspect`

```json
{
  "environment_id": "env_xxx",
  "skill_id": "skill_xxx"
}
```

返回 source/artifact/support/availability facts。

### `environment_skill_files`

```json
{
  "environment_id": "env_xxx",
  "skill_id": "skill_xxx",
  "root_kind": "artifact",
  "max_entries": 200
}
```

`root_kind`：

- `artifact`
- `support`
- `support:<index>`

列 bounded file inventory。

### `environment_skill_read`

读 SKILL.md：

```json
{
  "environment_id": "env_xxx",
  "skill_id": "skill_xxx"
}
```

读明确授权的 supporting file：

```json
{
  "environment_id": "env_xxx",
  "skill_id": "skill_xxx",
  "path": "relative/path.txt",
  "max_bytes": 65536
}
```

必须是 enabled Skill，并且 path 在 artifact/support authority 内。

ADM 不解释 Skill task policy；Agent 自己读并遵循 Skill。

## 18. Memory tools

### Global Memory

Agent surface 可：

- `memory_global_list`
- `memory_global_read`

写/删 Global Memory 是 Admin-only。

read：

```json
{
  "key": "machine"
}
```

### Environment-private Memory

Agent surface 可：

- `memory_environment_list`
- `memory_environment_read`
- `memory_environment_write`
- `memory_environment_delete`

示例：

```json
{
  "environment_id": "env_xxx",
  "key": "task",
  "value": "private context"
}
```

Scope 必须显式。ADM 不会自动把 private Memory promotion 到 Global。

注意：`environment_context_bundle` 不自动读取这些 values。

## 19. Evidence-first investigation

### `investigate_endpoint`

```json
{
  "environment_id": "env_xxx",
  "target": "/api/users/123",
  "method": "GET",
  "path": "",
  "max_files": 500,
  "max_matches": 100,
  "max_bytes_per_file": 1048576
}
```

做 bounded static route evidence resolution，返回 evidence/confidence/uncertainties。

不会：

- 运行项目代码；
- 真正请求 endpoint；
- 运行 verifier；
- mutation。

### `investigation_provider_inspect`

查看可选 code intelligence provider（例如通过 MCP 提供的 GitNexus）是否有可用 desired/observed evidence。

它是 passive inspect，不会 connect/refresh/index/call provider tool。

## 20. Agent surface 不提供的主要管理能力

以下属于 `/admin/mcp`，普通 Agent `/mcp` 不应拿它们做机器管理：

### Workspace / Environment management

- `workspace_add`
- `workspace_rename`
- `workspace_remove`
- `environment_create`（durable generic create）
- `environment_worktree_create`（durable managed-worktree create；Agent 应改用 temporary managed-worktree）
- `environment_rename`
- `environment_remove`
- `environment_verifier_add`
- `environment_verifier_remove`

Agent 如需短生命周期 context，可使用专门的 temporary Environment lifecycle，而不是获得任意 durable management 权限。

### Exec management

- `exec_allow`
- `exec_allow_remove`
- `exec_block`
- `exec_block_remove`
- `exec_block_list`
- `exec_deny_list`
- `exec_deny_clear`
- `exec_deny_clear_all`

默认 Strict 模式下 Agent 只能在管理员已批准的 allowlist 下执行；Full Authorization 可放行未列入 allowlist 的 executable，但 command blacklist 始终拥有最高拒绝优先级。

### Global MCP management

- `mcp_list`
- `mcp_add`
- `mcp_update`
- `mcp_remove`
- `mcp_set_default`
- `mcp_probe`
- `mcp_import_preview`
- `mcp_import_apply`
- `environment_mcp_set`

Agent 的 Environment MCP runtime tools 只消费管理员已配置/选择的 capability。

### Global Skill management

- `skill_list`
- `skill_add`
- `skill_remove`
- `skill_set_default`
- `skill_availability_list`
- `skill_source_list`
- `skill_source_add`
- `skill_source_update`
- `skill_source_refresh`
- `skill_source_remove`
- `environment_skill_set`

### Retention admin recovery

- `resource_retention_inspect`
- `resource_retention_cleanup`
- `resource_retention_mark_temporary`
- `resource_retention_promote`
- `environment_worktree_cleanup_stale`

`environment_worktree_cleanup_stale` 专门处理历史 durable managed worktree 堆积：默认只 preview；按 Environment `last_activity_at` 和 `inactive_seconds` 判断 stale，同时检查 active Writer / MCP / process / run / verifier、Git dirty、unpublished commit。`execute=true` 时必须显式传 `environment_ids`，并在真正删除前再次检查；没有 force 路径，删除 worktree 后 generated branch 仍保留。temporary managed worktree 不会被这条工具接管，继续走 owner-scoped temporary cleanup。

Agent temporary Environment lifecycle 是更窄、owner-scoped、no-force 的 workflow。

### Global Memory mutation

- `memory_global_write`
- `memory_global_delete`

## 21. 管理面还有哪些通用工具

`/admin/mcp` 还提供 `management_snapshot`，用于 Desktop/CLI 获取 sanitized overview：Workspace、Environment summaries、exec allowlist / command blacklist、MCP/Skill catalog、Global Memory count 等。

snapshot 不返回 Global/Private Memory values。

## 22. Tool 选择指南

### “我只想知道当前项目长什么样”

优先：

```text
environment_context_bundle
```

需要更多文件：`tree/read/search`。

### “这个能力为什么不能用”

优先：

```text
environment_capability_report
```

MCP 细查：`environment_mcp_inspect`。

Skill 细查：`environment_skill_inspect`。

### “我要改代码”

```text
environment_writer_acquire
-> read/search
-> edit/write/delete
-> exec/verifier
-> environment_writer_release
```

### “我要跑短命令”

`exec`。

### “我要跑一个很久但最终结束的普通命令”

`run_start/status/cancel`。

### “我要启动 dev server”

`process_start/status/logs/stop`。

### “我要跑结构化 test/build verifier”

短：`environment_verifier_run`。

长：`environment_verifier_run_start/status/cancel`。

### “我要确认 MCP 配置/健康”

Agent 已在 Environment 下：

- passive evidence：`environment_mcp_inspect`
- 需要实际刷新：`environment_mcp_refresh`

Global `mcp_probe` 是 Admin 管理行为。

### “我要临时隔离做任务”

`environment_temporary_create`，必要时 `mode=managed_worktree`。

结束时先 `environment_temporary_cleanup` preview，再 `execute=true`；要保留则 `promote`。

## 23. 不应由 Agent 假设的事情

Agent 不应该假设：

- 当前 cwd 就是 Environment；
- Workspace 一定是 Git repo；
- `go`/`node`/`git` 一定在 allowlist；
- MCP catalog entry 一定在当前 Environment enabled；
- configured MCP 一定 healthy；
- Skill catalog entry 一定 artifact 存在；
- Memory 自动注入 context；
- expired temporary Environment 可以直接删除；
- Gateway restart 后旧 process/run/vfrun 还存在；
- Admin MCP 只要有 API Key 就可以不经 TLS/网络边界直接暴露公网；
- ADM 会替 Agent 做 task planning/GSD/Git integration decisions。

## 24. 一个完整 Agent 开发示例

假设调用方已经提供 `ENV_ID=env_xxx`，Agent session owner 为 `agent-42`：

```text
1. environment_context_bundle(env_xxx)
   -> 发现 root、项目摘要、可用 go、Git 是否可用、verifier/MCP/Skill facts

2. search(env_xxx, query="HandleLogin")
3. read(env_xxx, path="internal/http/login.go")

4. environment_writer_acquire(env_xxx, "agent-42")

5. edit(...)

6. environment_verifier_list(env_xxx)
   -> 如果有适合的 test verifier：
      environment_verifier_run_start(...)
      environment_verifier_run_status(...)
   -> 否则如果 go 已 allow：
      run_start(executable="go", args=["test","./..."])
      run_status(...)

7. 根据结果继续 edit

8. git_diff(env_xxx)   # 如果 Git capability 可用

9. environment_writer_release(env_xxx, "agent-42")
```

整个流程中 ADM 负责：

- root containment；
- Writer authority；
- executable allowlist；
- bounded output；
- Runtime lifecycle；
- MCP/Skill Environment authorization；
- capability diagnostics。

Agent 负责：

- 为什么改；
- 改什么；
- 如何解释 test 结果；
- 是否完成任务；
- 是否 commit/merge/push（需要时通过已有能力显式执行）。

这就是 ADM v1.1 的 Agent/Gateway 边界。
