# ADM 常驻进程资源监控与生命周期

- Process 属于 Gateway 运行时，不属于 Writer 租约。Agent 的启动/停止授权及管理员无需 Writer 的管理入口保持不变。资源采样、日志读取、进程退出不续期 Writer。
- Windows 使用独立 Job Object（KILL_ON_JOB_CLOSE）。启动时先挂起主线程，将进程加入 Job 后才恢复执行；无法建立 containment 则启动失败，不静默降级。取消通过 Job 终止。主进程结束后最多等待 2 秒输出管道（Go WaitDelay），关闭 Job 回收仍在其中的子进程。Gateway 异常退出时系统关闭 Job handle，并回收 Job 内进程。不能承诺控制通过外部服务、提权或其他特殊机制脱离 Job 的进程。
- Linux 保留现有进程组取消机制，统计同组活进程，核对主进程启动标识；暂未实现父进程正常退出或 Gateway 崩溃时的完整组回收。macOS/其他平台仍使用原取消逻辑，资源数据明确不可用。
- 资源字段：采样时间、CPU 累计毫秒数、CPU 利用率（100% 为一个逻辑核心，可超过 100%）、内存工作集/RSS 之和、活进程数、统计范围、是否完整。共享页可能重复计数，不是主机物理内存独占量。Linux /proc 采用目标 Linux amd64/arm64 的 USER_HZ=100，CPU 总量仅包括活组成员；成员变化时丢弃跨样本 CPU 差值。Windows CPU 总量来自 Job accounting；内存读取前检查 Job 归属。Windows Job PID 列表有 1024 项观测上限，超出或查询失败显示不可用。初次采样不伪造 CPU 数值。
- Process 资源采样按请求执行，单进程缓存 1 秒，不启动额外监控服务。已有端口观测仍是主 PID 的监听 TCP 端口，不代表全树端口。运行时间、实际工作目录、退出原因及资源统计通过现有 API 和 Desktop/Web 管理端展示。统计不可用时不显示假数据。退出后保留本次 Gateway 内的日志与退出状态，不持久化进程历史，不在新 Gateway 中自动重启或认领旧 PID。
- EADM Windows：真实 Process、资源采样、无 Writer 管理、Writer 交接、主进程退出后子进程端口回收、隔离测试 owner 强制退出后 Job 端口回收通过；浏览器模拟适配器三种尺寸各 307 项通过。DADM Linux：在独立临时测试 Worktree 中验证新 runtime 的真实进程组内存/成员统计及取消，已恢复测试树干净并释放锁。未进行完整 Linux Gateway 集成测试。
- 后续：Linux 异常退出与重启回收设计、Gateway 运行历史与归属恢复、完整跨平台集成及安装验收。