# Linux 常驻 Process 的异常回收

每个 Linux ADM Process 由同一 ADM 可执行文件的内部 supervisor 模式管理。它不是额外常驻全局服务，不需要 root、systemd 或额外安装，也不使用 Writer 锁。Gateway 持有控制管道写端，supervisor 监控读端 EOF。Gateway 正常取消或被 SIGKILL 时，控制端关闭，supervisor 终止其业务进程组。业务主进程正常退出也执行组清理，保留主进程退出码（信号退出映射为 128+signal）。

业务进程组与 supervisor 的组分离。业务主进程通过 Waitid WNOWAIT 保持身份，组信号发送完成后才 Wait，避免退出后向复用的组 ID 发信号。supervisor 设为 child subreaper，并在退出前有限时间回收孤儿后代。业务主进程 Pdeathsig 防护针对 supervisor 自身消失；它不等同于 cgroup 全量回收。控制管道和启动握手管道不继承给业务程序。仅在启动握手确认业务 PID 与启动标识后向 Gateway 返回成功。实际工作目录、预授权路径、参数、环境、日志路由保留。API 记录和端口监控使用业务 PID 而非 supervisor PID。

范围：仅长驻 Process，有限 Exec/Run/Verifier 保持既有实现。普通进程组成员可回收；主动 setsid/setpgid 逃逸、外部服务、不可中断内核等待或 supervisor 本身异常死亡的全部后代不能保证清理。未实现 cgroup containment，不根据历史 PID 强行认领或杀进程。重启后的未确认记录仍保留未确认状态。每个常驻服务会新增一个轻量 supervisor 进程。没有改变 Writer 授权或续期。

验收：DADM 独立 Worktree 运行 runtime 与 process history Store 专项三轮通过。覆盖业务主进程正常退出后子进程端口释放、隔离 owner 被强制杀死后子进程端口释放、资源采样、退出码 23、缺失 executable 启动失败及历史记录持有者状态。EADM Windows Process/WriterHandoff/TailLog 相关回归通过。DADM 测试树恢复干净并释放 Writer。未运行 Linux 完整 Gateway/桌面安装集成，也未发布。
