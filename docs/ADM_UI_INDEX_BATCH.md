# ADM Desktop：索引后台任务与批量启停

## 项目索引

在「环境 → 详情 → 基本信息 → 代码索引」：

- 点击「后台生成 / 更新」。Gateway 立即分配任务 ID；分析继续运行，关闭详情窗口不会中断。
- 进度区显示当前阶段、已扫描/预计扫描文件数。扫描过程中为真实文件计数；发布完整快照前进度不会虚报 100%。
- 重新打开详情会查询该 Environment 最近的任务状态；完成后自动刷新索引完整性。
- 状态摘要显示已索引文件数、检查数、变化数。路径和具体变化折叠在详情中，避免 Windows 长路径撑破弹窗。
- 如果索引状态为「部分索引」，会明确提示未覆盖的源码可能导致搜索缺失。
- 「自动监控代码变化」依旧由 ADM Gateway 运行，自动刷新与手动后台任务使用同一套索引锁，不占用 Environment Writer。

```json
{"environment_id":"env_xxx"}
```

对应 MCP 工具为 `project_index_job_start` 与 `project_index_job_status`。两者均与窗口生命周期无关；任务状态保存在当前 Gateway 服务进程内，重启 Gateway 会丢失历史任务状态，但已成功发布的索引仍持久保存在 ADM 数据目录。重复启动运行中的任务会返回同一任务 ID。

## MCP 与 Skill 批量启停

在 MCP 或 Skills 管理页筛选并勾选资源（可跨页选择 MCP 与 Skill），点击「批量设置启停…」：

1. 选中任意多个 **Workspace**，修改 Workspace 的继承资源。
2. 选中任意多个 **Environment**，修改各环境的显式资源。
3. 可同时勾选两个范围；点「在所选目标开启 / 关闭」会明确确认影响范围和资源组合数。
4. 仅对所选资源、所选目标操作；已经是目标状态的组合自动跳过。弹窗中的**真实进度条按资源 × 目标的组合数量逐项增长**，实时显示已完成/总数、正在处理的对象、已修改/跳过/失败数。期间按钮和关闭操作不可用，避免重复执行；完成后恢复操作并逐条展示结果。

**继承语义不变**：关闭 Environment 的显式 MCP/Skill 不会抵消对应 Workspace 的已启用项；若要禁用 Workspace 继承，需同时在 Workspace 范围执行关闭。

主操作栏只保留 **全选当前筛选结果 / 取消选择 / 批量设置启停**；进入对话框后选择 Workspace / Environment 与开启或关闭。**所有批量启停和新环境默认操作均只作用于已勾选项目，筛选不会自动勾选或更改配置。** 新环境默认与维护操作位于列表下方，和主操作流程分开；仅在有勾选时才能更改默认规则。清理和删除保留原有确认流程。

## 验收命令

```powershell
go test ./internal/app ./internal/gateway ./internal/desktop ./internal/projectanalysis -count=1
node --test tests/desktop-ui/environment-bulk.test.cjs
node tests/desktop-ui/browser-smoke.cjs
$env:ADM_UI_SMOKE_FOCUS='environment'; node tests/desktop-ui/browser-smoke.cjs
```

后台索引回归专门覆盖：任务ID、进度状态、已持有源码写入锁的情况、任务重复提交、已完成后可再次生成，以及 Gateway MCP 会话重新查询。浏览器回归覆盖关闭详情后继续查看和 Workspace 单独批量启停。
