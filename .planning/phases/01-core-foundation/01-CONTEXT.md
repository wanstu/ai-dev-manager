# Phase 1 Context — Core Domain & EffectiveConfig

## Phase Goal

建立项目最重要的第一条纵向闭环：用纯 Go 领域模型表示 Global / Profile / Project / Runtime Override，并解析成可追踪来源的 `EffectiveConfig`。

本 Phase 不做真实 MCP 连接、不做 Shell、不做 UI、不做配置文件持久化；只证明核心继承语义正确。

## Requirements

- CONF-01..07
- WORK-01
- RUN-01
- COMPAT-01

## Locked Decisions

1. 新工程独立于 `codexprov4`，不得引用或修改 `codexprov4` 代码。
2. 不 fork CodexPro / DevSpace。
3. 语言优先 Go；Core 不依赖 Wails。
4. 配置优先级固定为：`Runtime Override > Project > Profile > Global`。
5. Runtime 不参与配置继承，只接收 `EffectiveConfig`。
6. `MCP` 和 `Skill` 都必须支持：继承、新增、覆盖、显式禁用。
7. 配置解析必须保留来源 trace；后续 UI/调试必须能回答“为什么这个值是这样”。
8. 本 Phase 只实现内存模型和 resolver；不因 JSON/TOML 选择扩大范围。
9. 不为未来 Docker/Claude/DevSpace 预先建立大量空接口，只定义当前 resolver 测试真正需要的最小类型。

## Minimal Domain Model

建议第一版只需要这些概念：

```go
type Scope string

const (
    ScopeGlobal  Scope = "global"
    ScopeProfile Scope = "profile"
    ScopeProject Scope = "project"
    ScopeRuntime Scope = "runtime"
)

type Workspace struct {
    ID        string
    Path      string
    ProfileID string
    RuntimeID string
}

type MCPDefinition struct {
    ID        string
    Enabled   *bool
    Transport string
    Command   string
    URL       string
    Env       map[string]string
}

type SkillDefinition struct {
    ID      string
    Enabled *bool
    Path    string
}

type Policy struct {
    Mode string
}

type ConfigLayer struct {
    Scope  Scope
    MCPs   map[string]MCPDefinition
    Skills map[string]SkillDefinition
    Policy *Policy
}
```

说明：

- `Enabled *bool` 用三态语义：`nil` = 该层不表态，`true` = 显式启用，`false` = 显式禁用。
- 是否把 transport/env/path 进一步拆类型，只有当测试暴露需求时再做。
- `Workspace` 暂时只放 resolver 需要的稳定引用，不加入 PID、port、process 等 runtime state。

## Merge Semantics

### Layer order

按从低到高应用：

1. Global
2. Profile（可为空）
3. Project（可为空）
4. Runtime Override（可为空）

### Map entities

MCP / Skill 按稳定 `ID` merge：

- 高层不存在该 ID → 保留低层。
- 高层新增 ID → 加入 EffectiveConfig。
- 高层同 ID 提供字段 → 覆盖对应字段。
- 高层 `Enabled=false` → EffectiveConfig 保留定义及来源信息，但标记 disabled，便于解释和 UI 展示。
- 后续更高层可以再次 `Enabled=true` 重新启用。

### Scalar/object values

例如 Policy：最后一个非空定义获胜。

## Source Trace

Phase 1 要证明“来源可解释”。建议：

```go
type SourceTrace struct {
    Scope Scope
    Key   string
}

type ResolvedValue[T any] struct {
    Value T
    Trace SourceTrace
}
```

但不要为了泛型美观过度设计。可先在 `ResolvedMCP` / `ResolvedSkill` 中直接保存：

```go
type ResolvedMCP struct {
    MCPDefinition
    Source Scope
}
```

若一个实体的不同字段来自不同层，需要测试后再决定是否升级为 field-level trace。Phase 1 最低要求：能解释最终实体/策略的主来源，以及 disable/enable 最后由哪层决定。

## Conflict Rules

Phase 1 必须明确拒绝：

- 空 ID。
- 同一 `ConfigLayer` 内无法唯一标识的重复项（如果使用 map 天然规避；若未来从 slice decode，要在 store 层校验）。
- 未知 Scope。

Phase 1 不负责：

- 命令是否存在。
- URL 是否可连接。
- Skill 路径是否存在。
- MCP env 是否包含 secret。
- Workspace 路径是否越界。

这些属于后续 Phase。

## Package Boundary

建议最小结构：

```text
ai-dev-manager/
├── go.mod
├── internal/
│   ├── model/
│   │   └── config.go
│   └── config/
│       ├── resolver.go
│       └── resolver_test.go
└── .planning/
```

Phase 1 不建 `runtime/`、`mcp/`、`git/`、`docker/` 空目录，避免“architecture astronaut”。

## Test Matrix

至少覆盖：

1. 只有 Global → 结果等于 Global。
2. Profile 覆盖 Global policy。
3. Project 新增私有 MCP，不修改输入 Global。
4. Project 禁用 Global MCP。
5. Runtime Override 重新启用 Project 禁用的 MCP。
6. Project Skill 覆盖 Global 同 ID Skill path。
7. Runtime Skill 禁用 Project Skill。
8. Profile 缺失时仍能解析。
9. Project / Runtime 层为空时仍能解析。
10. resolver 不 mutate 任一输入 layer。
11. source trace 与最终高优先级来源一致。
12. 非法 Scope / 空 ID 返回结构化错误。

## Exit Criteria

- 所有上述测试通过。
- `go test ./...` 通过。
- 没有 Wails / UI / MCP SDK / Docker 等非必要依赖。
- README / PROJECT / STATE 与实际实现没有明显漂移。
- 如果实现过程中发现 field-level trace 必不可少，先更新本 Context/Plan 再扩展，不静默改范围。
