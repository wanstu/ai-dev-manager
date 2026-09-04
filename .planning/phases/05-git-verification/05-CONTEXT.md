# Phase 5 Context — Git & Verification

## Phase Goal

让 Native Runtime 从“能安全改文件/跑命令”升级成“能理解代码改动并自行验证”：提供结构化 Git status/diff/branch/worktree 能力，Project 可以声明 test/lint/build/custom verifier，Runtime 能执行并返回结构化结果，最终验证一次“修改 → diff → test/build”。

本 Phase 不启动 MCP Host、不做 Docker、不做 Agent/UI。

## Requirements

- WORK-03
- EXEC-02（Git + Verification remaining portions）
- VERIFY-01..03
- RUN-02（Git/Test capability portion）

## Locked Decisions

1. Git structured API 必须复用 `Native.Exec`，不能绕开 Phase 4 policy/tool resolution。
2. Standard 模式只有 allowlist 包含 `git` 时暴露 Git capability；Full 模式可以暴露。
3. status/diff/branch 只对 Runtime 当前 Workspace root 操作，不接受任意 repo path。
4. Worktree target 不接受任意用户路径。v0.1 使用 managed root：`<workspace-parent>/.ai-dev-manager-worktrees/<workspace-id>/`。
5. Worktree name/branch 只接受安全标识，不允许路径分隔符、`.`、`..` 或绝对路径；Create 默认用 `git worktree add -b <branch> <managed-target> HEAD` 创建新 branch。
6. Worktree Remove 只允许删除 managed root 下由该 Runtime 管理的名字；默认不 `--force`。
7. 新 worktree 创建后属于新的 checkout，未来可由 Control Plane 注册成独立 Workspace；主 Runtime 不自动把自己的 filesystem root 切过去。
8. Verifier 是配置能力，加入四层 ConfigLayer/EffectiveConfig 与 v1 JSON optional fields；不单独发明第二套配置系统。
9. Verifier Kind v0.1：`test | lint | build | custom`；定义包含 executable/args/cwd/timeout_seconds/enabled。
10. Verifier merge 使用稳定 ID + Phase 1 同样的层级 precedence；Enabled 为 tri-state。运行时 `Enabled=false` 跳过，nil/true 表示可运行（定义存在即默认启用）。
11. Verifier Runner 复用 `Native.Exec`，因此 cwd containment、ToolPaths、allowlist、timeout/output cap 自动继承 Phase 4 安全规则。
12. Verifier result 结构化记录 id/kind/status/exit code/duration/summary/stdout/stderr/timedOut；失败不只返回一段 shell 文本。
13. 本 Phase 不实现 arbitrary unified patch parser。现有 exact Edit + Git diff 已足够形成首个修改/审查/验证闭环；apply-patch 后续有真实需求再加。

## Git API

建议 `internal/runtime/git.go`：

```go
type GitStatusEntry struct {
    Path string
    X    string
    Y    string
}

type GitDiff struct {
    Files []string
    Patch string
}

type Worktree struct {
    Path   string
    Head   string
    Branch string
    Bare   bool
}
```

最低方法：

- `GitStatus()`
- `GitDiff()`
- `GitBranch()`
- `GitWorktrees()`
- `GitWorktreeCreate(name, branch)`
- `GitWorktreeRemove(name)`

所有命令固定 cwd = workspace root。

## Managed Worktree Safety

`managedRoot`：

```text
<parent-of-workspace>/.ai-dev-manager-worktrees/<workspace-id>/
```

Runtime 只可通过 worktree API 创建/删除其下一级安全 name。

- name: `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`
- branch 同样拒绝空、路径分隔符和 `..`；允许常见 `feature-x` 风格。
- Create 前 target 必须不存在。
- Remove 前 target 必须在 `git worktree list --porcelain` 中且位于 managed root。

## Verifier Model

```go
type VerifierDefinition struct {
    ID             string
    Kind           string
    Enabled        *bool
    Executable     string
    Args           []string
    Cwd            string
    TimeoutSeconds int
}
```

`ConfigLayer.Verifiers map[string]VerifierDefinition`
`EffectiveConfig.Verifiers map[string]ResolvedVerifier`

Resolver：

- executable/kind/cwd/args/timeout 非空/非零字段由高层覆盖。
- Enabled 单独追踪 EnabledSource。
- slices 深拷贝，禁止 alias。

## Verifier Runner

建议 `internal/runtime/verifier.go`：

```go
type VerifierStatus string
const (
    VerifierPassed  = "passed"
    VerifierFailed  = "failed"
    VerifierSkipped = "skipped"
)
```

- `RunVerifier(id)`
- `RunVerifiers(ids...)`
- 没有给 IDs 时可运行所有可用 verifier，按 ID 稳定排序。
- failure = process exit code != 0 或 timeout。
- policy/tool lookup failure 作为 structured error，不伪装成测试失败。
- Summary 从 status/id/kind/exit code 生成，不解析工具特定输出。

## Test Matrix

### Git

1. temp repo commit 后修改文件 -> status 返回结构化 modified entry。
2. diff 返回 changed files + patch。
3. branch 返回当前 branch。
4. Standard 未 allowlist git -> Git API policy denied。
5. ToolPaths git absolute path 可用。
6. worktree create 只在 managed root 创建，主 checkout HEAD/working tree 不被切换。
7. worktree list 可看到新 worktree。
8. worktree remove 只删除 managed target；非法 name/path 被拒绝。

### Verifier config/resolver

1. Project 保存 test/build verifier round-trip。
2. higher layer override command/args/timeout。
3. project disable verifier。
4. resolver output 不 alias args/enabled。
5. old v1 JSON 无 verifiers 仍加载。

### Runner

1. test verifier pass -> structured passed result。
2. build verifier pass。
3. failing verifier -> failed + exit code。
4. timeout -> failed/timedOut。
5. disabled verifier -> skipped without process start。
6. cwd outside workspace -> path error。

### End-to-end

真实 temp git repo：

```text
commit baseline
  ↓
Native.Edit
  ↓
GitStatus/GitDiff
  ↓
Run test verifier
  ↓
Run build verifier
```

必须证明 diff 指向修改文件，两个 verifier 均 passed。

## Exit Criteria

- 结构化 status/diff/branch 真实工作。
- managed worktree create/list/remove 真实工作且不切换主 checkout。
- Project 至少 test + build verifier 可声明/持久化/解析/执行。
- verifier failure 明确 id/kind/exit code/timedOut。
- 一个真实 temp repo 完成“修改 → diff → test/build”。
- `go test ./...`、`go vet ./...` 全绿。
- 无 MCP Host/Docker/Agent/UI scope leak。
