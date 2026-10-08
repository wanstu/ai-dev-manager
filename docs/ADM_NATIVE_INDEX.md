# ADM 原生代码索引：外置存储、自动增量与锁隔离

代码索引是 AI 的只读源码缓存，**不属于业务项目文件**，不应申请 Environment Writer。

## 数据位置

索引目录：ADM 状态文件所在目录下的 `indexes/<Environment ID>/`。结构：

```text
indexes/
  env_xxxx/
    current.json
    generations/
      gen-xxxx/
        .adm/project-overview.md
        .adm/index/{manifest.json,files.jsonl,symbols.jsonl,calls.jsonl}
```

每个 Environment 独立。`current.json` 通过原子替换发布一份完整生成代；上一个完整代暂时保留，避免并发查询访问到未写完的文件。普通 Environment 删除和 managed-worktree 删除自动清理该环境的缓存及自动刷新偏好，**不会删除项目源码**。

升级前业务目录中已有的 `.adm` 不会自动清除（可能混有用户文件）。新版首次分析重新创建 ADM 管理的外置索引，后续分析按源码 SHA-256 增量复用未变化记录。

## 如何启用

- 首次使用 `project_analyze`，只需要 `environment_id`，不用填 `writer_owner`；
- Desktop：环境详情 → 基本信息 → 代码索引 → **自动监控代码变化，增量更新索引**；
- Admin MCP：`project_index_auto_set`，`enabled=true`；状态可通过 `project_index_auto_status` 查询。关闭或重启 Gateway 后会记住配置。

自动监听 Go/PHP/JS/TS 文件的写入、新增、删除、重命名。连续事件去抖约 1.6 秒后触发增量分析；忽略 `.git`、`.adm`、`vendor`、`node_modules` 等目录。监控目录超过 3000 个会提示缩小环境根目录。

**代码修改与索引刷新不会争用 Environment 写入锁。** 缓存内部有按 Environment ID 隔离的读写锁；源码只读，缓存写入不可变 generation，校验哈希并原子发布新版本。查询仍要区分旧索引和现有源码：状态不一致时使用 `project_index_status` 检查并刷新。

**并发边界**：独立索引锁目前是单 Gateway 进程内保证。同一个 ADM 状态目录不应由多个 Gateway 进程同时写入，独立 Gateway 应使用独立状态目录。

## 验证

```powershell
go test ./internal/projectanalysis ./internal/app ./internal/gateway ./internal/desktop -count=1
```

- `TestExternalProjectIndexNeverRequiresEnvironmentWriter`：AI 有写入锁时仍可索引，项目根目录未写入 .adm，环境删除清理缓存；
- `TestProjectIndexConcurrentReadersObserveCompleteGenerations`：多次刷新时索引查询总能得到完整生成代；
- `TestAutomaticIndexUpdatesWithoutStealingSourceWriter`：真实 fsnotify 监控保存后自动刷新、不中断已有的 AI Writer；
- `tests/desktop-ui/browser-smoke.cjs`：不同尺寸窗口的自动监控开关交互。
