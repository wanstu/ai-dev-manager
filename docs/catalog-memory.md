# MCP、Skill 与 Memory（v1.1）

本页集中解释三类“开发上下文能力”的模型。完整操作流程见 [USER_GUIDE.md](USER_GUIDE.md)，CLI flags 见 [cli.md](cli.md)。

## 1. 共同原则：Global definition 与 Environment selection 分离

MCP 和 Skill 都采用：

```text
global definition/catalog
        |
        +-- default_include_in_environment  -> 只影响未来新 Environment
        |
Environment stores enabled IDs only
```

因此：

- catalog 中存在，不代表某 Environment 已启用；
- `set-default` 不会改已有 Environment；
- enable/disable 一个 Environment 不改 global catalog，也不影响其他 Environment；
- global entry 被移除后，Environment 中旧 ID 可能成为 unresolved reference，不会偷偷重建定义。

# MCP

## 2. Global MCP definition

v1.1 支持：

- `streamable-http`
- `stdio`

HTTP：

```powershell
adm mcp add `
  --name filesystem `
  --transport streamable-http `
  --endpoint http://127.0.0.1:9000/mcp
```

HTTP header auth：

```powershell
adm mcp add `
  --name private-api `
  --transport streamable-http `
  --endpoint https://example.invalid/mcp `
  --auth-mode headers `
  --header-refs-json '{"Authorization":"${MCP_AUTH}"}'
```

stdio：

```powershell
adm exec allow --executable mcp-helper

adm mcp add `
  --name local-helper `
  --transport stdio `
  --executable mcp-helper `
  --args-json '["serve","--stdio"]' `
  --env-refs-json '{"API_TOKEN":"${MCP_API_TOKEN}"}'
```

stdio executable 必须通过 ADM exec allowlist。

## 3. Secret/reference

MCP definition 持久化环境引用，不持久化 secret literal：

```text
${MCP_API_TOKEN}
```

实际值只在 activation boundary 解析。正常状态、inventory、错误和 management snapshot 不应返回 secret value。

MCP import 遇到 credential literal 时也会转换为 reference requirement，而不是原样落 state。

## 4. Environment selection

```powershell
adm environment mcp enable --environment-id ENV_ID --mcp-id MCP_ID
adm environment mcp disable --environment-id ENV_ID --mcp-id MCP_ID
```

禁用会撤销该 Environment 的 MCP runtime access。

### Injection plan：selection 与实际可用性分离

Environment / Workspace selection 只表示“希望向 Agent 暴露这项能力”，不代表当前一定可直接使用。统一使用：

```powershell
adm environment injection-plan --environment-id ENV_ID
```

读取当前 selected MCP / Skill 的注入计划。

- `selection_sources=environment`：Environment 显式 selection；
- `selection_sources=workspace`：来自 Workspace inheritance；
- 两者可同时存在；
- `injectable=false` 表示仍然 selected，但当前配置/文件/authority 条件不满足；
- `next_action` 告诉 Agent 应先 inspect，还是可以进入 MCP tool inventory / Skill read。

`default_include_in_environment` 仍然只表示 catalog 对**未来新 Environment**的默认 seed。已有 Environment 中的历史 selection 不会因为当前 default flag 改变而被重新解释。

## 5. Desired config 与 Runtime observation 分离

Persistent definition 不是 health。

Gateway Runtime Owner 在内存中观察：

- live session；
- health state；
- failure stage/count；
- reconnect facts；
- bounded public tool inventory。

这些 runtime observations 不作为 desired state 持久化；Gateway restart 后重新开始观察。

## 6. MCP 四种诊断

### Global probe

```powershell
adm mcp probe --id MCP_ID
```

不需要 Environment；transient connect/protocol/inventory check，结束后关闭。

### Environment status

```powershell
adm mcp status --id MCP_ID --environment-id ENV_ID
```

应用 Environment selection 后做 configured/disabled/healthy/error 探测。

### Passive inspect

```powershell
adm mcp inspect --id MCP_ID --environment-id ENV_ID
```

只读 desired + 已有 owner observation；不 connect/Ping/refresh。

### Explicit refresh

```powershell
adm mcp refresh --id MCP_ID --environment-id ENV_ID
```

显式 reconnect/Ping/list tools，更新 owner observation；不调用 business tool，不修改 desired state。

## 7. Health/reconnect

MCP definition 可配置：

- periodic health check；
- interval；
- bounded probe timeout；
- auto reconnect；
- fixed reconnect interval。

Auto reconnect 默认关闭。即使开启，也不会自动 replay 失败的业务调用。

## 8. Import

```powershell
adm mcp import-preview --file .\mcp.json
adm mcp import-apply --file .\mcp.json
```

也支持 explicit stdin/inline，但三种 source 必须 exactly one。

Import 是 normalization + validation：raw blob 不持久化，apply 只改 global definitions，不改现有 Environment selections。

# Skill

## 9. Skill 是 artifact，不是 metadata-only prompt

ADM Skill 指向真实 `SKILL.md`。核心对象是：

- Skill Source；
- discovered Skill entry；
- artifact root；
- explicit support roots；
- Environment selection；
- availability diagnostics。

ADM 不把 Skill instructions 编译成内部 task workflow；Agent 显式读取并执行 Skill 的建议。

## 10. Source lifecycle

### Add source（不 refresh）

```powershell
adm skill source-add `
  --root C:\skills `
  --support-root C:\shared-a `
  --support-root C:\shared-b `
  --default
```

`--support-root` 可重复。

### Update source（不 refresh）

```powershell
adm skill source-update `
  --id SKILL_SOURCE_ID `
  --root C:\skills-new `
  --support-root C:\shared
```

省略 `--default` 保留当前 default setting。

### Explicit refresh

```powershell
adm skill source-refresh --id SKILL_SOURCE_ID
```

对一个 source 原子刷新 discovery snapshot。update/refresh 分离，避免编辑配置产生隐藏 filesystem scan。

### Remove source

```powershell
adm skill source-remove --id SKILL_SOURCE_ID
```

移除 source 及其 source-owned Skills；Environment selections 不静默重写。

### Compatibility convenience

```powershell
adm skill add --root C:\skills --support-root C:\shared --default
```

等价于更“立即”的 register + discovery convenience。要控制 refresh 时机时优先 source commands。

## 11. Availability

Global structural：

```powershell
adm skill availability
```

检查 source/artifact/support facts，与 Environment selection 无关。

Environment-specific：

```powershell
adm environment skill list --environment-id ENV_ID
adm environment skill inspect --environment-id ENV_ID --skill-id SKILL_ID
```

叠加 enabled/disabled/unresolved/artifact_missing 等状态。

Availability inspection 不读取 Skill 正文，也不执行 Skill reasoning policy。

## 12. Agent content authority

Agent 通过：

- `environment_skill_files`
- `environment_skill_read`

读取 enabled Skill。

读取范围严格限制在 Skill artifact root 或 explicit support roots；Skill 自己提到主机其他路径不会自动扩大权限。

# Memory

## 13. Global Memory

跨 Environment 共享持久 context：

```powershell
adm memory global list
adm memory global read --key machine
adm memory global write --key machine --value windows
adm memory global delete --key machine
```

Global Memory 不随 Environment 删除。

## 14. Environment-private Memory

```powershell
adm memory environment list --environment-id ENV_ID
adm memory environment read --environment-id ENV_ID --key task
adm memory environment write --environment-id ENV_ID --key task --value 'private context'
adm memory environment delete --environment-id ENV_ID --key task
```

只属于一个显式 Environment，默认不泄漏给其他 Environment。

## 15. Memory privacy

以下视图默认不展开 private values：

- management snapshot；
- Environment list；
- Environment inspect；
- Capability Report；
- Environment Context Bundle。

Context Bundle 也不会自动注入 Global Memory values。

需要 Memory value 时必须显式 `read`。写入时必须显式选择 global 或 environment scope；ADM 不自动 copy/promote/merge/sync 两个 scope。

## 16. 三者如何进入 Agent 工作

推荐思路：

```text
environment_context_bundle
  -> 看 MCP/Skill 安全摘要与 availability/capability guidance

需要 MCP business capability
  -> environment_mcp_inspect / refresh / tools / call

需要 Skill instructions
  -> environment_skill_inspect / read

需要 durable context value
  -> memory_global_read / memory_environment_read
```

ADM 刻意不把所有 MCP/Skill/Memory 内容自动塞进每次 Agent prompt。这样上下文更 bounded，也保持 privacy/authority 显式。
