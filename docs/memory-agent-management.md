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

## 结构化记忆和版本

- Entry 新增 revision、metadata、updated_at。历史 KV 在首次修改前 revision=0，更新时间未知，不伪造历史。现有写入接口保留；新记录从 revision=1 开始，相同内容和元数据的重复写入不新增版本。
- organize 的 change 可指定 metadata（category、tags、importance）及 expected_revision。更新元数据必须提供 value 和 expected_revision；类别为 convention、decision、progress、issue、environment、preference、other 或空，重要度 0（未设）～5，标签去重规范化。搜索匹配类别及标签。
- 新增 global_history/environment_history：key 必填，revision 可选；默认只列版本摘要，指定 revision 才读取完整历史正文。
- 新增 global_restore/environment_restore：key、revision、expected_revision 必填。恢复历史正文与元数据，内容有变化则创建新版本；拒绝过时 revision。每个 key 保留当前版本和最近 20 个历史版本。
- 删除当前条目后，旧版本仍可通过 key 查阅和恢复；环境移除时同步移除对应历史。当前不是永久清除接口，管理员永久清除及按已删除条目浏览仍需实现。请勿存储秘密。
- 新增 global_duplicates/environment_duplicates：报告作用域内正文完全相同的非空条目，最多 100 组、64 KiB 输出；不自动合并。近似重复、语义召回仍待实现。
- 全局恢复仅管理员，环境恢复由 Agent 显式指定环境执行。所有操作不借用 Writer。
- 管理页面当前显示分类、标签、重要度、版本及更新时间，搜索包含分类编码与标签；历史恢复和元数据编辑 UI 尚待接入，Agent MCP 已支持。

## 管理端及回收站

- Desktop/Web 已接入分类筛选、标签和重要度编辑、历史版本查看及正文并排对比、恢复、回收站、完全重复正文查找和保留指定条目的合并。合并前执行原子预演，并在用户确认后提交；未保留的条目进入回收站，仍可查看其元数据及历史。
- UI 新增、编辑和删除使用 organize 的原值及版本校验，避免直接覆盖 Agent 并发更新；编辑和确认时验证连接与环境作用域。
- 新增 global_trash/environment_trash，列出已删除条目及可恢复版本；global_purge/environment_purge 仅管理员可用，要求 expected_revision 且仅允许已删除记录。永久删除移除该 Key 在当前状态文件中的历史，不承诺擦除备份。
- 已有状态存储跨 Gateway 的互斥和并发安全仍需单独整改。关键词搜索与完全重复识别不等同于语义检索。

## 项目记忆继承与相关性召回

- environment_project_scope 根据 ADM 记录的 ManagedWorktree.SourceEnvironmentID 逐级解析持久项目环境，返回显式 ID 及来源链；不凭仓库名称、remote 或路径猜测关系。拒绝缺失来源、循环和无持久源的临时环境。
- Agent 在获授权的项目任务中显式读取当前环境和已验证的持久源环境。Worktree 私有记忆仍独立，不自动复制、注入或合并。
- environment_handoff 参数：environment_id、target_environment_id、items（key、expected_revision、expected_value，可选 target_key）、dry_run。目标必须是已验证的持久源且与当前环境不同。逐项核实原值与版本，拒绝已存在或有删除历史的目标键；整个批次原子提交，不删除源。先 dry_run=true，确认后 false。已有目标记忆须显式读取再 organize。此机制依赖 Agent 在清理前调用，未交接的临时记忆仍随环境清理消失。
- global_recall/environment_recall：与 search 使用相同 query/limit/max_output_bytes 参数，支持部分关键词覆盖，英文单词、中文字符和二元组，键名/标签/类别加权，重要度小幅加权。仅返回匹配项并限制序列化输出。不调用模型或向量服务，不宣称理解同义词或语义。
- global_similar/environment_similar：threshold 默认 0.75，范围 0.5～1；limit 默认 30，最多 100。按词项集合 Jaccard 相似度识别非完全相同正文的候选；每次扫描按键排序的前 200 条，每条最多 8192 字符，返回扫描规模及截断状态。仅供人工或 Agent 复核，不自动合并。
- 本阶段仅接通 Agent MCP，近似查重和项目交接的管理端专用交互、记忆有效期及语义向量检索待实现。跨 Gateway 状态事务仍依赖既有 Store 边界。
