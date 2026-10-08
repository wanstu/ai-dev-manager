# ADM Code Intelligence：面向 AI 开发的验收记录

## 产品目标

索引的价值不是生成 `.adm` 文件，而是让 AI 在调查、修复业务问题时更快定位源码、查清调用影响，并清楚区分事实、静态分析线索和未知内容。ADM 原生实现是默认路径，不依赖 PhpStorm、JetBrains MCP 或其他 IDE。

## 本轮发现：默认索引上限导致漏检

2026-10-08，在 Windows 开发机上对 `wm_main/base/application/controllers` 进行**只读内存分析**，不写入原业务仓库：

| 情况 | PHP 文件 | 索引符号 | 完整 | 能否索引 `User_goods::getDealBaseInfo` | 耗时 |
| --- | ---: | ---: | --- | --- | ---: |
| 旧默认 4,000 files / 1,200 symbols | 610 | 1,200 | 否 | 否 | 8.59s |
| 手动增大上限 | 610 | 7,656 | 是 | 是 | 9.37s |
| 调整后的新默认 25,000 files / 100,000 symbols | 610 | 7,656 | 是 | 是 | 9.26s |

注：耗时仅是同一机器的参考观测，不能视为正式基准。单次默认上限仍不能保证任意超大型仓库一定完整；`project_index_query.index_complete` 和 `coverage_warning` 明确告知 AI 结果是否可能漏掉定义。不要把部分索引的零匹配解释成“源码没有这个函数”。

## 一次调用取得调查所需的最小证据

`project_investigate_php` 在单次 MCP 调用中组合：

- 目标定义位置；
- 调用图及每条调用边的 `kind`、`reason`、可选 `type_hint`；
- 少量代表性的源码片段及具体行号（单段至多 2,048 字节）。

对 `wm_main` 的 `User_goods::getDealBaseInfo` 做只读源码复制的端到端烟测，得到了 1 条调用边、2 段源码片段（合计 433 字节源码正文），并定位测试类中的继承调用。原仓库未生成索引、未做修改。此结果证明**工具可一次返回必要线索**，并不等价于 AI 已经自动完成业务根因调查。

建议 AI 工作流：

1. 对明确的 PHP 函数直接使用 `project_investigate_php`；若有重名报错，用 `project_index_query` 核对准确类名和文件路径。
2. 检查 `index_complete`、`index_warning`、`truncated`，索引缺失/陈旧则使用 `project_index_status` 并重新 `project_analyze`。
3. 根据返回行号精读关键业务代码，交叉验证真实请求参数、配置和数据流。
4. 只有运行时证据或足够的代码分析支持时，才确定原因与改动方案；静态 `candidate_call` / `inherited_candidate` 不能称为已证实执行。

## 回归测试（可在纯 ADM 仓库执行）

```powershell
go test ./internal/projectanalysis ./internal/app ./internal/gateway ./internal/codeintel -count=1
```

关键断言：

- `TestDefaultIndexLimitsCoverMoreThanLegacySymbolBudget`：超过旧 1,200 符号上限仍完整。
- `TestPartialSymbolIndexExposesCoverageWarningToAgent`：部分索引零匹配时返回覆盖告警。
- `TestProjectInvestigatePHPReturnsDefinitionAndCallEvidenceInOneTool`：单次 MCP 工具调用返回带证据的片段，避免读取完整长文件，且拒绝陈旧索引。
- 原有调用链、继承与类型证据测试，避免将不确定的 PHP 调用报告为确证关系。

## 2026-10-08 补充：真实控制器调用链与候选优先级

再次从原 `wm_main/base/application/controllers` **只读复制** PHP 文件至临时目录，完整运行 ADM v5 索引与调用图查询：

| 观测项 | 结果 |
| --- | ---: |
| 复制 PHP 文件 | 605 |
| PHP 源码总字节 | 13,309,022 |
| 静态符号 | 7,614 |
| 词法调用记录 | 100,059 |
| 分析及查询耗时（含临时索引生成） | 约 11.05 秒 |
| 对 `User_goods::getDealBaseInfo` 的调用边 | 1 条 `inherited_candidate` |
| 是否包含 `User_goods_test.php:116` | 是 |
| 调用图是否标记截断 | 否 |

两次代码快照的文件数量及符号数量可能有差异（上一次为 610/7,656）；各次独立测试只说明当次的真实状态。**本轮没有直接写入原业务仓库，也没有执行业务代码**。

受控回归构造了另一个可能导致 AI 误判的场景：15 条低可信度的动态调用出现在确定调用之前，且工具最多返回 4 条。旧实现返回的 4 条全是 `candidate_call`，未返回真正明确的 `Service::run()` 调用；修复后先取 `resolved_call`，再取 `inherited_candidate`，再取带类型线索的候选，最后才取纯动态候选。对下游调用同样测试了仅返回 2 条时的优先级。

该排序是**证据展示顺序**，并不改变 PHP 的静态类型推断能力。`truncated=true` 仍提醒 AI 可能存在尚未返回的调用；没有返回某条调用不代表不存在。

持续回归测试：

- `TestPHPCallGraphPrefersResolvedEvidenceUnderSmallLimit`
- `TestPHPCallGraphPrefersResolvedCalleeUnderSmallLimit`

## 下一轮需要量化的指标

固定 3～5 个可复现的业务缺陷问题（真实重现记录或受控回放），分别使用普通 `search/read` 和索引辅助工作流，记录：调用次数、读取源码字节数、是否找到真正定义、候选误报、最终原因是否正确。**尚未完成 AI 模型端到端的对照评测**，不能因为工具测试通过就声称 AI 已提升某个百分比。
