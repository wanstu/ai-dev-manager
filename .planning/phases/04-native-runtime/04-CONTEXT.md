# Phase 4 Context — Native Runtime & Security Policy

## Phase Goal

建立第一个真正能对本地 Workspace 做受控操作的 Native Runtime：查询 capability、读取/搜索/编辑项目文件，并执行经过策略校验的本地命令；两个 Workspace Runtime 必须物理隔离，路径越界和 symlink escape 必须在执行层被拒绝。

本 Phase 不启动 MCP Host、不实现 Git worktree、不做 Docker 结构化 API、不做 Agent、不做 UI。

## Requirements

- WORK-02
- RUN-02
- RUN-03（Native runtime context 部分；MCP Server instance 部分仍留 Phase 6）
- EXEC-01
- EXEC-02（Shell 部分；Git/Verification 继续 Phase 5）
- EXEC-04
- SEC-01

## Locked Decisions

1. Runtime 只接收 `Workspace + EffectiveConfig`，不读取 Global/Profile/Project 文件，也不参与配置继承。
2. Native Runtime 使用纯 Go 核心，不依赖 Wails/MCP SDK。
3. Phase 4 的命令执行 API 接收 `Executable + Args`，不接收任意 shell command string；Standard 模式不通过 shell interpreter 执行，避免 shell metacharacter 注入。
4. `Full` 表示 executable 不受 allowlist 限制，但依然使用结构化 argv、workspace cwd containment、timeout 和 output limit；真正的 raw shell interpreter 如以后需要，必须作为显式 capability 单独设计。
5. Policy mode 语义固定：
   - `read-only`: 允许文件读取/目录/搜索；拒绝文件写入和命令执行。
   - `workspace-write`: 允许 Workspace 内文件读写；拒绝命令执行。
   - `standard`: workspace-write + 仅允许显式 allowlist executable 的命令执行。
   - `full`: workspace-write + 任意 executable（仍受 cwd/timeout/output/blocked-path 边界）。
6. `Policy` 在 Phase 4 增加 `AllowedExecutables []string` 与 `ToolPaths map[string]string`；Policy 仍作为一个整体由现有 resolver 的高层非空 Policy 覆盖。
7. EXEC-04：工具解析优先使用 `ToolPaths[name]` 显式路径；否则使用 `exec.LookPath(name)`。错误必须区分 tool-not-found 与 policy-denied。
8. 所有路径授权由 Runtime 内部统一的 path guard 强制执行，不能只靠调用者传“正确路径”。
9. 目录 containment 不使用字符串前缀；使用 canonical absolute root + `filepath.Rel`，并防止 symlink 指向 Workspace 外。
10. `.git` 目录与 `.ai-dev-manager/runtime` 默认禁止直接写入；Git 操作后续通过结构化 Git capability 处理。普通项目配置 `.ai-dev-manager/config.json` 不默认阻止读取。
11. Search Phase 4 使用 Go 标准库递归文本搜索，不依赖 `rg`；后续可增加 faster adapter，不改变 Runtime contract。
12. `apply-patch` 从 Phase 4 的第一条 vertical slice 延后到 Phase 5：先用 exact edit/write 验证安全边界，Phase 5 在 Git/diff 语义存在后再决定 patch 实现，避免现在引入不必要的 diff parser。

## Model / Policy Extension

Phase 4 扩展：

```go
type Policy struct {
    Mode               string
    AllowedExecutables []string
    ToolPaths          map[string]string
}
```

Store v1 JSON 继续兼容 optional fields：

```json
{
  "policy": {
    "mode": "standard",
    "allowed_executables": ["go", "php", "npm"],
    "tool_paths": {
      "go": "D:\\tools\\go\\go1.26.5\\bin\\go.exe"
    }
  }
}
```

不保存进程 PID、当前命令、runtime logs 到配置层。

## Runtime Capability Model

建议新增：

```text
internal/runtime/
├── runtime.go
├── pathguard.go
├── files.go
├── search.go
├── exec.go
└── *_test.go
```

最小 capability：

```text
files.tree
files.read
files.write
files.edit
search.text
shell.exec
```

`Capabilities()` 返回稳定排序的结构化 capability，而不是让 UI/Agent 猜支持什么。

## Native Runtime Shape

建议：

```go
type Native struct {
    workspace model.Workspace
    config    model.EffectiveConfig
    root      string
    guard     *PathGuard
}

func NewNative(workspace model.Workspace, cfg model.EffectiveConfig) (*Native, error)
func (r *Native) Capabilities() []Capability
```

创建时：

1. Workspace path 必须存在、为目录。
2. canonicalize root。
3. validate policy mode；空 Policy/空 mode 不静默提升权限，默认 `read-only`。
4. Runtime 保存自己的 immutable config snapshot，避免外部 mutate 后改变运行权限。

## Path Guard

### Existing path read

- relative path -> root join。
- absolute path也允许传入，但必须仍位于 root 内。
- `EvalSymlinks` 后再次 containment check。
- Workspace 外 -> `path_outside_workspace`。

### Write/new path

目标可能不存在：

1. clean/abs target。
2. lexical `filepath.Rel` containment check。
3. 找到最近存在 parent。
4. `EvalSymlinks(parent)`。
5. resolved parent 仍必须位于 root。
6. blocked write path 检查。

这样 `workspace/link -> D:\outside` 后写 `link/file` 会被拒绝。

## File APIs

建议最小结构化结果：

```go
type FileInfo struct {
    Path  string
    IsDir bool
    Size  int64
}

type EditResult struct {
    Path         string
    Replacements int
    BytesBefore  int
    BytesAfter   int
}
```

### Tree
- bounded max depth / max entries。
- stable sort。
- 不跟随 directory symlink。

### Read
- max bytes，超限返回 structured limit error。
- 不提供任意二进制 decode；Phase 4 返回 bytes/text，由 adapter 决定展示。

### Write
- `workspace-write/standard/full` 才允许。
- parent mkdir 可作为显式 option，默认 false，避免意外创建深层目录。
- atomic replace 优先。

### Edit
- exact old/new replacement。
- 默认要求 old text 唯一匹配；支持显式 expected replacement count。
- 不实现 regex replacement。

### Search
- configured root 下递归 regular files。
- bounded max files/max matches/max bytes per file。
- 遇到明显 binary（NUL byte）跳过。
- stable results。

## Command Execution

API 方向：

```go
type Command struct {
    Executable string
    Args       []string
    Cwd        string
    Timeout    time.Duration
}

type CommandResult struct {
    Executable string
    Args       []string
    Cwd        string
    ExitCode   int
    Stdout     string
    Stderr     string
    Duration   time.Duration
    TimedOut   bool
}
```

规则：

- ReadOnly / WorkspaceWrite -> deny。
- Standard -> executable 名必须在 `AllowedExecutables`（Windows case-insensitive）。
- Full -> executable allowlist 不检查。
- Cwd 默认 Workspace root；自定义 cwd 必须通过 PathGuard。
- ToolPaths 有对应项 -> 验证该 executable 文件存在并使用它。
- 否则 `exec.LookPath`。
- timeout 必须有上限；Phase 4 默认 30s、最大 10m。
- stdout/stderr 各自设置合理 byte cap，避免进程打爆内存。
- command env Phase 4 默认继承当前进程；不解析 MCP EnvRefs，也不把 MCP 环境注入 shell。
- 不自动调用 `cmd /c`、PowerShell 或 Bash。

## Multi-Workspace Isolation

测试至少创建：

```text
Workspace A/
  a.txt
Workspace B/
  b.txt
```

Native A：

- read A/a.txt -> allowed
- write A/new.txt -> according to policy
- read/write B/b.txt by absolute path -> denied
- `../B/b.txt` -> denied
- symlink escape -> denied when platform allows symlink test

Native B 与 A 拥有独立 root/config snapshot。

## Structured Errors

至少区分：

```text
invalid_policy
path_outside_workspace
path_blocked
read_only
execution_denied
tool_not_allowed
tool_not_found
timeout
output_limit
not_found
invalid_edit
limit_exceeded
```

错误文本不得带文件内容、环境变量值或完整敏感配置。

## Test Matrix

### Runtime / capability

1. default no-policy -> read-only。
2. capability list deterministic。
3. Runtime snapshots EffectiveConfig; caller mutate config afterwards不改变 policy。

### Containment

1. relative in-root path allowed。
2. absolute in-root path allowed。
3. `..` escape denied。
4. absolute other-workspace denied。
5. symlink escape denied where supported。
6. blocked `.git` write denied。

### Files

1. ReadOnly can tree/read/search but cannot write/edit。
2. WorkspaceWrite can write/edit within root。
3. exact edit wrong/ambiguous match -> structured error。
4. write atomic round-trip。
5. search bounds and binary skip。

### Execution

1. ReadOnly/WorkspaceWrite shell denied。
2. Standard allowlisted executable works。
3. Standard non-allowlisted executable denied before process start。
4. explicit ToolPaths executable works even when ordinary PATH differs。
5. missing tool -> structured error。
6. cwd outside workspace -> denied。
7. timeout produces TimedOut result/error contract without orphan process where feasible。
8. output cap prevents unbounded capture。

## Exit Criteria

- Runtime A 不能读写 Workspace B，包含 `..` 和 symlink escape。
- ReadOnly / WorkspaceWrite / Standard / Full 权限在执行层真实生效。
- Files/Search/Edit 和 structured exec 都有真实测试。
- `ToolPaths` 证明可解决当前已观察到的 Windows PATH 差异。
- 两个 Native Runtime 可同时存在且配置/root 不污染。
- `go test ./...`、`go vet ./...` 通过。
- 无 Git worktree、Docker、MCP Host、Agent、UI 跨 Phase 实现。
