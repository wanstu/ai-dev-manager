# Phase 2 Context — Config Store & Workspace Registry

## Phase Goal

把 Phase 1 已验证的内存配置模型落到真实本地配置，并建立多个 Workspace 的持久化注册表。完成后系统应能从磁盘恢复 Workspace，读取 Global / Profile / Project 三层配置，再交给 Phase 1 resolver 生成 `EffectiveConfig`。

本 Phase 不启动 MCP Server、不执行 Shell、不扫描 Skill 内容、不做 Runtime 进程生命周期。

## Requirements

- WORK-01
- CONF-01..04 persistence path/schema
- SKILL-01
- SKILL-02

## Locked Decisions

1. 继续使用纯 Go Core，不引入 UI 框架。
2. Phase 1 resolver 语义不在本 Phase 重写；Store 负责读取，Resolver 负责继承。
3. Runtime Override 仍为内存态，不持久化到 Project/Global 配置。
4. v0.1 持久化格式使用 JSON，优先标准库，暂不引入 TOML/YAML dependency。
5. 用户级配置根目录使用 `os.UserConfigDir()` + `ai-dev-manager`；测试必须允许注入临时 root，不依赖真实用户目录。
6. Project 配置放在 `<workspace>/.ai-dev-manager/config.json`，它表达项目差异而不是 runtime state。
7. Project 配置默认应允许被版本控制；`.gitignore` 只忽略 `.ai-dev-manager/runtime/` 等运行时状态，不应粗暴忽略整个 `.ai-dev-manager/`。
8. Secret 不直接作为 Phase 2 主题；配置可保存 env/reference 字段，但后续 Phase 3 必须建立 secret-safe 规则。
9. Workspace path 在 Windows 下比较时至少使用 `filepath.Clean/Abs` + case-insensitive equality，禁止同一路径重复注册。
10. Workspace ID 使用标准库生成的稳定随机 ID，不引入 UUID dependency；建议 `ws_` + 16 bytes crypto/rand hex。

## On-disk Layout

建议 v1 schema：

```text
<UserConfigDir>/ai-dev-manager/
├── config.json
└── profiles/
    ├── work.json
    └── personal.json

<workspace>/
└── .ai-dev-manager/
    └── config.json
```

### User config

`config.json` 至少包含：

```json
{
  "version": 1,
  "global": {
    "mcps": {},
    "skills": {},
    "policy": null
  },
  "workspaces": []
}
```

`global.scope` 不需要持久化；读取时由 Store 构造成 `ScopeGlobal`，减少用户写错 scope 的机会。

### Profile config

`profiles/<id>.json`：

```json
{
  "version": 1,
  "mcps": {},
  "skills": {},
  "policy": null
}
```

文件名来自经过校验的 profile ID，不接受路径分隔符或 `..`。

### Project config

`<workspace>/.ai-dev-manager/config.json`：

```json
{
  "version": 1,
  "mcps": {},
  "skills": {},
  "policy": null
}
```

读取时构造成 `ScopeProject`。

## Workspace Persistence Model

Phase 1 的：

```go
type Workspace struct {
    ID        string
    Path      string
    ProfileID string
    RuntimeID string
}
```

继续作为领域对象。

Phase 2 需要注册表行为：

- Add workspace
- Update workspace
- Remove workspace
- List workspaces
- Get workspace by stable ID
- Reject duplicate normalized path
- Preserve stable ID across edits/restarts

暂不加入 PID、port、process state。

## Store Semantics

### Load

- 用户级 `config.json` 不存在 → 返回 version=1 的空默认配置；不因为只读操作自动写盘。
- profile 文件不存在 → 若 workspace 引用了该 profile，返回明确 not-found error；不静默当空 profile。
- project config 不存在 → 视为空 Project layer，这是常见情况。
- JSON 损坏 → 返回结构化 decode error，不覆盖原文件。
- schema version > 当前支持版本 → 返回 unsupported-version error。
- schema version < 当前版本 → Phase 2 只为 migration 接口留最小入口；若只有 v1，不做虚构 migration。

### Save

- 父目录不存在时创建。
- 同目录 temp file + flush/close + rename，避免半截 JSON。
- JSON UTF-8，格式化缩进便于人工查看和 Git diff。
- 保存前校验 workspace / profile ID / config entity ID。
- 不在日志中打印完整 env map。

## Project Config / Git Boundary

Phase 2 需要修正当前 `.gitignore`：

```text
.ai-dev-manager/runtime/
```

可以忽略 runtime state，但：

```text
.ai-dev-manager/config.json
```

不应默认被忽略，因为它未来就是 Project 私有 MCP/Skill 差异的可共享配置入口。

## Resolution Flow

Phase 2 最小闭环：

```text
Workspace ID
   ↓
Workspace Registry
   ↓
load Global
load optional Profile
load optional Project
   ↓
config.Resolve(...)
   ↓
EffectiveConfig
```

建议提供一个薄 service，而不是让调用方自己拼路径和 layer：

```go
type WorkspaceConfigService struct { ... }

func (s *WorkspaceConfigService) ResolveWorkspace(id string, runtime *model.ConfigLayer) (model.EffectiveConfig, error)
```

如果实现中发现命名不合适可以调整，但职责必须保持：Store 负责 IO，Resolver 负责 merge，Service 负责 orchestration。

## Package Direction

合理的最小扩展：

```text
internal/
├── model/
│   └── config.go
├── config/
│   ├── resolver.go
│   ├── resolver_test.go
│   ├── store.go
│   └── store_test.go
└── workspace/
    ├── registry.go
    ├── service.go
    └── *_test.go
```

这些 package 现在有真实 vertical-slice 用途，因此不属于 speculative scaffolding。

## Test Matrix

### User config store

1. missing config -> default v1, no write side effect.
2. save + reload round-trip.
3. UTF-8 / Windows path round-trip.
4. malformed JSON -> structured error, original file untouched.
5. unsupported newer version -> structured error.
6. atomic save leaves valid previous file if write/rename path fails where testable.

### Workspace registry

1. add workspace generates stable non-empty ID.
2. reload preserves ID/path/profile/runtime.
3. duplicate normalized Windows path rejected.
4. update keeps ID stable.
5. remove only target workspace.
6. unknown ID returns structured not-found error.

### Profile / Project layers

1. profile loads as `ScopeProfile`.
2. missing referenced profile errors explicitly.
3. missing project config becomes empty `ScopeProject`.
4. project config loads MCP/Skill differences without mutating Global.
5. profile ID path traversal rejected.

### End-to-end resolution

1. Global + Profile + Project resolves using Phase 1 precedence.
2. Project disables Global MCP after disk round-trip.
3. Project Skill overrides Global path after disk round-trip.
4. runtime override passed in memory still wins and is never persisted.

## Exit Criteria

- All test matrix items that are practical for v1 are covered and passing.
- `go test ./...` passes with the actual local Go executable.
- `go vet ./...` passes.
- Two workspaces can be persisted and reloaded with stable IDs.
- Project A config cannot change Project B effective config.
- Global config is shared without copying into each project.
- No MCP process, Shell execution, Docker, Wails, or Skill file scanning is introduced.
- `.planning/STATE.md` and requirement status are updated before transition.
