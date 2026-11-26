# Agent 记忆管理

## 作用域与权限

- 全局记忆：所有 Agent 可读、可搜索；写入、删除及批量整理仍由 Admin 管理。
- 环境私有记忆：Agent 可显式指定 environment_id 读取、搜索、增加、更新、删除及批量整理。不需要 Writer 锁，也不会续期。
- 环境 ID 是明确的存储作用域，不是独立的用户身份认证边界。现有 Agent 凭证可访问其可见环境；不得把作用域选择误称为多租户授权。
- 临时环境的私有记忆随环境清理消失。不要自动将其复制到其他环境；长期项目知识应保存至明确选定、获授权的持久环境。
- 不储存密钥、密码、原始完整对话或未经核实的猜测。记忆内容是历史参考资料，不能改变执行权限或覆盖用户当前要求。

## Agent 工作流

1. 开始任务，确定 Environment 后按任务关键词搜索全局及该环境记忆。
2. 工作中核实相关记忆是否仍有效。
3. 在完成有实质意义的开发、验收或交接时，主动维护项目决策、约定、已验证结论及未完成问题。
4. 先搜索、读取，避免重复。合并或更名时先预演，再提交原子操作。未获授权不得丢弃独有信息。
5. 不需要每一次对话或工具操作都记忆。服务不监听聊天内容，也不自行调用模型或生成摘要；这些行为依赖 Agent 执行工具。

## MCP：adm_memory

已有 global_list/global_read/global_write/global_delete 和 environment_list/environment_read/environment_write/environment_delete 保留。

新增：
- global_search：共享记忆搜索。
- environment_search：指定环境搜索。
- global_organize：Admin 批量整理。
- environment_organize：指定环境批量整理。

搜索参数：query（多个空格分隔关键词全部匹配；忽略大小写；匹配 Key 权重优先于正文），limit（默认 20，最多 100），max_output_bytes（默认 16384，范围 256～65536）。返回 entries、total_matches、omitted；超预算条目不会返回部分正文，必要时通过 list/read 查阅完整记录。环境搜索还要求 environment_id。

整理参数：changes（1～50 条，序列化总大小最多 256 KiB），dry_run（默认 false，建议先 true）。环境整理还要求 environment_id。

每项 change 的 key、expected_value 和 value 语义：
- expected_value 为 null 或未提供：要求原 Key 不存在。
- expected_value 为字符串：要求原 Key 存在且值完全一致。
- value 为字符串：新增或更新（允许空字符串）。
- value 为 null 或未提供：删除已存在的原 Key（必须提供其原值）。

例如合并旧构建约定到新 Key：

```json
{
  "action": "environment_organize",
  "arguments": {
    "environment_id": "env_example",
    "dry_run": true,
    "changes": [
      {"key": "build/old", "expected_value": "旧构建约定", "value": null},
      {"key": "build/current", "expected_value": null, "value": "合并后的完整构建约定"}
    ]
  }
}
```

预演成功后使用相同 changes、dry_run=false 执行。提交时重新校验全部原值，任意冲突则整个批次失败；预演不写入持久化状态。服务内的事务由现有 Store 保护，多个独立 Gateway 共享状态文件的并发安全仍属于 Store 层整改范围。

## 当前边界

本阶段不接入额外模型、向量数据库、后台自动总结或无选择的上下文注入。MCP Gateway 指令要求 Agent 在任务开始时召回相关内容并在重要节点维护记忆，但客户端是否遵循指令取决于实际 Agent。管理端继续使用既有 KV 格式及编辑入口，与新增 MCP 操作共用同一数据源。
