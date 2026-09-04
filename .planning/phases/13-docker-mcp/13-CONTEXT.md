# Phase 13 Context — Docker-reachable Workspace MCP

## Goal

修复真实用户验收问题：Workspace runtime 当前固定监听 `127.0.0.1`，因此 Docker 中通过 `host.docker.internal:<port>` 无法连接。默认安全行为必须保持不变，但用户显式请求时应允许 Docker-reachable MCP bind。

## Locked Decisions

1. daemon Control API 永远保持 loopback-only。
2. Workspace MCP 默认仍为 `127.0.0.1:0`。
3. `--docker` 是显式 exposure，映射到 `0.0.0.0:0`；`--listen` 也是显式高级入口。
4. 非 loopback exposure 不能通过原有安全默认路径意外发生；Host Manager 保留 safe StartHTTP，并增加显式 exposed start path。
5. runtime status 应给出可直接复制的 client endpoint；Docker 模式返回 `http://host.docker.internal:<port>/mcp`。
6. desired state 需要保存 Workspace + listen intent；旧 v1 `workspace_ids` 必须可读并按 loopback 迁移。
7. 不增加 token/auth；本阶段只解决本机 Docker 网络可达性。公网/远程访问仍 out of scope。

## Acceptance

- loopback default regression test。
- explicit exposed host test。
- runtime HTTP `--docker` equivalent path test。
- desired v1 compatibility + v2 restart reconciliation test。
- daemon control non-loopback rejection remains green。
