# ADM Code Intelligence Provider Contract v1

本协议定义外部 Code Intelligence Provider 如何通过 MCP 接入 ADM。首个目标实现是 PhpStorm / JetBrains 插件，但协议本身不绑定 IDE。

## 1. 传输与版本

Provider 使用 ADM 已有的 MCP catalog / Environment authorization / Gateway runtime owner，不新增第二套连接或授权模型。

固定契约：

- `contract`: `adm.code_intelligence`
- `protocol_version`: `1`

ADM 只会在以下条件同时满足时尝试外部 Provider：

1. MCP 已被当前 Environment 启用；
2. Gateway 已观察 MCP 为 healthy；
3. tool inventory 明确包含本次调用需要的只读 tool；
4. `code_intelligence_info` negotiation 成功；
5. Provider 返回的 contract / protocol_version 与 ADM 当前版本一致。

协议不匹配时，ADM 不会继续调用业务 tool。

## 2. 必需与可选 MCP tools

Provider 应暴露：

- `code_intelligence_info`：必需，用于 negotiation。
- `code_intelligence_query`：symbol definition 查询。
- `code_intelligence_status`：Provider/索引新鲜度或可用状态。
- `code_intelligence_references`：引用查询。
- `code_intelligence_hierarchy`：父子层级查询。

Provider 可以只实现部分业务能力，但 `code_intelligence_info` 必须准确声明 capabilities。ADM 不会因为 tool 名存在就忽略 capability 声明。

未知 tool、重构、写文件、rename 等 mutating capability 不属于本协议，ADM 不会将其识别为 Code Intelligence read capability。

## 3. Negotiation

ADM 在真正调用业务 tool 前调用：

```json
{
  "name": "code_intelligence_info",
  "arguments": {
    "environment_id": "env_xxx",
    "project_root": "D:\\projects\\example",
    "contract": "adm.code_intelligence",
    "protocol_version": 1
  }
}
```

Provider 必须通过 MCP `structuredContent` 返回：

```json
{
  "contract": "adm.code_intelligence",
  "protocol_version": 1,
  "provider_id": "phpstorm",
  "name": "PhpStorm Code Intelligence",
  "provider_version": "0.1.0",
  "source": "phpstorm.psi",
  "requires_generated_index": false,
  "capabilities": {
    "definitions": true,
    "references": true,
    "hierarchy": true
  }
}
```

约束：

- `provider_id` 不能为空。
- `contract` 必须精确匹配。
- `protocol_version` 必须等于 ADM 当前支持版本。
- `capabilities` 必须反映 Provider 实际能力。
- Provider 不应在 negotiation 时执行项目代码或修改项目。

## 4. 通用请求字段

业务 tool 请求均包含：

- `environment_id`：ADM Environment stable ID。
- `project_root`：该 Environment 在 ADM 连接主机上的物理根目录。
- `contract`: `adm.code_intelligence`。
- `protocol_version`: `1`。

Provider 必须把 `project_root` 当作本次查询的项目边界，不应越过该根目录返回其他项目的结果。

## 5. Definition query

输入：

```json
{
  "environment_id": "env_xxx",
  "project_root": "D:\\projects\\example",
  "contract": "adm.code_intelligence",
  "protocol_version": 1,
  "query": "Service.Run",
  "path": "internal/service",
  "kind": "method",
  "language": "Go",
  "exact": false,
  "max_results": 50
}
```

`max_results` 默认 50，上限 200。

structured result 与 ADM symbol query result 对齐：

```json
{
  "index_path": "provider://phpstorm",
  "matches": [
    {
      "path": "src/Foo.php",
      "language": "PHP",
      "namespace": "App",
      "kind": "class",
      "name": "Foo",
      "qualified_name": "App\\Foo",
      "line": 17,
      "match": "provider"
    }
  ],
  "returned": 1,
  "truncated": false
}
```

## 6. Status

输入：

```json
{
  "environment_id": "env_xxx",
  "project_root": "D:\\projects\\example",
  "contract": "adm.code_intelligence",
  "protocol_version": 1,
  "max_changes": 50
}
```

Provider 至少应返回非空 `state`。推荐状态使用 `fresh` / `stale` / `partial` / `invalid` / `unavailable` 这类稳定语义。

## 7. References

symbol locator：

```json
{
  "path": "src/Foo.php",
  "line": 17,
  "name": "Foo",
  "qualified_name": "App\\Foo",
  "kind": "class",
  "language": "PHP"
}
```

至少需要 `path` / `name` / `qualified_name` 之一。

请求：

```json
{
  "environment_id": "env_xxx",
  "project_root": "D:\\projects\\example",
  "contract": "adm.code_intelligence",
  "protocol_version": 1,
  "symbol": {
    "path": "src/Foo.php",
    "line": 17,
    "qualified_name": "App\\Foo"
  },
  "max_results": 100
}
```

`max_results` 默认 100，上限 500。

structured result：

```json
{
  "symbol": {
    "path": "src/Foo.php",
    "line": 17,
    "qualified_name": "App\\Foo"
  },
  "references": [
    {
      "path": "src/Bar.php",
      "line": 31,
      "column": 9,
      "language": "PHP",
      "kind": "call",
      "name": "Foo",
      "qualified_name": "App\\Foo",
      "context": "$foo = new Foo();"
    }
  ],
  "returned": 1,
  "truncated": false
}
```

`context` 必须 bounded，不应返回大段文件内容。

## 8. Hierarchy

请求：

```json
{
  "environment_id": "env_xxx",
  "project_root": "D:\\projects\\example",
  "contract": "adm.code_intelligence",
  "protocol_version": 1,
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

约束：

- `direction`: `parents` / `children` / `both`。
- `max_depth` 默认 2，上限 8。
- `max_results` 默认 100，上限 500。

structured result 使用 graph：

```json
{
  "symbol": {
    "path": "src/Foo.php",
    "line": 17,
    "qualified_name": "App\\Foo"
  },
  "direction": "both",
  "nodes": [
    {
      "id": "base",
      "path": "src/Base.php",
      "line": 8,
      "kind": "class",
      "name": "Base",
      "qualified_name": "App\\Base"
    },
    {
      "id": "foo",
      "path": "src/Foo.php",
      "line": 17,
      "kind": "class",
      "name": "Foo",
      "qualified_name": "App\\Foo"
    }
  ],
  "edges": [
    {
      "from": "base",
      "to": "foo",
      "kind": "extends"
    }
  ],
  "returned": 2,
  "truncated": false
}
```

edge kind 由 Provider 给出，例如 `extends` / `implements` / `overrides`。

## 9. 错误与降级

ADM 不解析自由文本 JSON。业务结果必须位于 MCP `structuredContent`。

Provider 出现以下情况时：

- MCP call 失败；
- tool 返回 `IsError`；
- structured content 缺失；
- structured content 无法解码；
- contract / protocol_version 不兼容；
- info capability 与实际调用不匹配；

ADM 都不会把该结果当成成功的 Code Intelligence。

definitions/status 可回退到 `adm_static_index`。返回中：

- `provider`：实际使用的 Provider；
- `attempted_provider`：曾尝试但失败的外部 Provider；
- `fallback_reason`：降级原因。

references/hierarchy 当前没有 built-in static 实现，因此外部 Provider 不可用时返回：

```json
{
  "provider": {"provider_id": "adm_static_index"},
  "available": false,
  "reason": "provider_capability_unavailable"
}
```

如果已尝试外部 Provider，则还会包含 `attempted_provider`，reason 会是 negotiation/call/result 相关原因。

## 10. 安全边界

Provider v1 是只读协议：

- 不写文件；
- 不运行项目代码；
- 不执行 shell；
- 不做 rename/refactor；
- 不自动修改 IDE/project settings；
- 只返回当前 `project_root` 内的代码智能结果。

ADM 的 Environment authorization 与 MCP health/session 管理由 Gateway 继续负责，Provider 不应自行建立绕过 ADM 的项目访问授权模型。
