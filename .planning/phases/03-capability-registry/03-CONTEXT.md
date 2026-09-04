# Phase 3 Context — MCP Catalog & Skill Discovery

## Phase Goal

在 Phase 1/2 已经成立的配置继承和磁盘持久化之上，建立真正可消费的共享能力层：MCP 以稳定 Catalog 暴露给后续 Runtime；Skill 可以从配置的全局/Profile/Project roots 自动发现，不需要逐项目复制 GSD 等 Skill。

本 Phase 不连接 MCP 进程、不执行 Skill、不启动 Agent、不运行 Shell。

## Requirements

- MCP-01..05
- SKILL-01..04

`AGENT-01` 已按 GSD scope review 延后到 v0.3 Agents / GSD Runtime，避免在没有真实 orchestration 用例时创建空抽象。

## Locked Decisions

1. 复用 Phase 1 `config.Resolve`；能力层不重写四层 precedence。
2. 复用 Phase 2 `Store / Registry / ConfigService` 边界；磁盘 IO 仍只属于 Store。
3. Global/Profile/Project 的显式 `Skills` 继续有效，并且显式 Skill 定义优先于同层 discovery 结果。
4. 新增 `SkillRoots` 作为配置层字段，用于自动发现；它不是 Runtime state。
5. Skill discovery 只识别实际存在的 `SKILL.md`，Skill ID 第一版采用其父目录名，不在 Phase 3 引入 YAML parser。
6. Discovery roots 按声明顺序处理；同一层多个 root 发现相同 ID 时，后声明 root 覆盖前声明 root；同层显式 `Skills[id]` 最终再覆盖 discovery。
7. `~` 路径展开到当前用户 Home；绝对路径直接使用；相对 Project root 相对 workspace 解析；Global/Profile 相对路径相对 user config root 解析。
8. MCP `Env` 表示可直接持久化的普通静态值；敏感值使用新增 `EnvRefs`（只保存引用名），普通 error/log 不展开实际环境值。
9. MCP health 是运行态，不能写进 config JSON；Phase 3 Catalog 只提供 `unprobed` 状态和未来 capability 扩展位。
10. 不自动读取 Codex/Claude/DevSpace 的私有配置格式；这些属于后续 compatibility adapters。Phase 3 先把自己的 Registry 语义做稳定。

## Model Extensions

### ConfigLayer

Phase 3 增加：

```go
type ConfigLayer struct {
    Scope      Scope
    MCPs       map[string]MCPDefinition
    Skills     map[string]SkillDefinition
    SkillRoots []string
    Policy     *Policy
}
```

`SkillRoots` 不进入 `EffectiveConfig`；它在 resolver 前被 discovery 展开成该层的 `Skills`。

### MCPDefinition

增加敏感环境引用：

```go
type MCPDefinition struct {
    // existing fields...
    Env     map[string]string
    EnvRefs map[string]string
}
```

例如：

```text
Env:
  LOG_LEVEL = debug

EnvRefs:
  SERVICE_CREDENTIAL = [SECRET_ENV_NAME]
```

这里只持久化引用名，不读取/复制宿主环境中的敏感值。

## Skill Discovery

建议 package：

```text
internal/skill/
├── discovery.go
└── discovery_test.go
```

API 可类似：

```go
func ExpandLayer(layer model.ConfigLayer, baseDir string) (model.ConfigLayer, error)
```

行为：

1. clone 输入 layer，不 mutate Store/Resolver 输入。
2. 依次解析 `SkillRoots`。
3. 对每个 root 递归查找 `SKILL.md`；不跟随目录 symlink。
4. 发现 `<dir>/SKILL.md` -> `SkillDefinition{ID: filepath.Base(dir), Path: dir}`。
5. 后 root 同 ID 覆盖前 root。
6. 最后应用 layer 原本显式 `Skills`，显式配置最高。
7. 返回相同 Scope 的 expanded layer。

结构化错误至少区分：invalid root / root missing / walk failure。

## Workspace Resolution Integration

Phase 2：

```text
load layers -> Resolve
```

Phase 3：

```text
load Global/Profile/Project
        ↓
expand SkillRoots per layer
        ↓
expand optional Runtime layer
        ↓
config.Resolve
        ↓
EffectiveConfig
```

baseDir：

- Global: `Store.Root()`
- Profile: `Store.Root()`
- Project: `Workspace.Path`
- Runtime Override: `Workspace.Path`

这样一个 Global root 例如 `D:\tools\skills` 只配置一次，所有 Workspace 都能得到其中的 `gsd`。

## MCP Catalog

建议 package：

```text
internal/mcp/
├── registry.go
└── registry_test.go
```

Catalog 不启动 MCP；它把 `EffectiveConfig.MCPs` 转成稳定消费接口：

```go
type Health string
const HealthUnprobed Health = "unprobed"

type Entry struct {
    Definition     model.MCPDefinition
    Source         model.Scope
    EnabledSource  model.Scope
    Health         Health
    Capabilities   []string
}
```

最低行为：

- Build from EffectiveConfig
- Get by stable ID
- deterministic List
- Enabled list
- disabled entry 仍保留，以便 UI/诊断解释来源
- Health 默认 unprobed；Phase 6/Runtime 后再实际 probe

## Persistence Compatibility

Phase 2 JSON schema version 仍保持 v1：新增字段是向后兼容 optional field。

新增：

```json
{
  "skill_roots": ["D:\\tools\\skills"]
}
```

MCP 增加：

```json
{
  "env_refs": {
    "SERVICE_CREDENTIAL": "[SECRET_ENV_NAME]"
  }
}
```

旧 v1 配置缺字段时按空值处理，不需要虚构 migration。

## Test Matrix

### Skill discovery

1. Global root 中 `gsd/SKILL.md` 被发现。
2. 一个 Global root 同时供两个 Workspace 使用，无复制。
3. Project root 只影响 Project A，不影响 B。
4. Project 同 ID Skill 覆盖 Global discovered Skill。
5. Project 显式 `Skills[id]` 覆盖 Project root discovered Skill。
6. 后声明 Global root 同 ID 覆盖前 root。
7. `~` root 正确展开（可通过 injectable home helper 或小函数单测）。
8. missing configured root 返回结构化错误。
9. discovery 不 mutate 输入 layer。

### MCP Catalog

1. Global MCP 在 A/B Catalog 中都存在。
2. Project A disable 后 Catalog 保留 entry 但 Enabled=false；B 仍 true。
3. Project private MCP 只出现在 A。
4. List 顺序稳定。
5. Health 默认 unprobed。
6. EnvRefs 经 Store round-trip + resolver merge 保持引用，不读取实际环境敏感值。

### Integration

1. Global `gsd` root + Project A private root -> A/B EffectiveSkills 不同且来源正确。
2. Runtime explicit Skill 可覆盖 project discovered Skill。
3. `.planning/` 不参与 Skill root discovery，除非用户显式把它配成 root（正常情况下不配）。

## Exit Criteria

- Global GSD Skill 通过一个 root 配置即可出现在两个 Workspace 的 EffectiveConfig。
- Project A 私有/覆盖 Skill 不泄漏到 B。
- Global/Profile/Project MCP 行为通过 Catalog API 验证，disabled entries 可解释。
- EnvRefs 只持久化引用，不读取实际敏感值。
- `go test ./...` 与 `go vet ./...` 真实通过。
- 不启动任何 MCP 进程，不执行 Shell，不创建 Agent 空框架。
- Phase 完成前更新 PROJECT/ROADMAP/STATE。
