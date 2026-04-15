# AHS 高危指令拦截演示仓说明

这个实验用于演示 AHS 面对真实破坏性 Playbook 时的拦截能力。

## 演示链路

1. 在本地 Gitea 创建或更新演示仓 `blacklist-demo-playbooks`
2. 将仓库注册到 AHS
3. 在 AHS 中创建 Playbook
4. 在 AHS 中创建任务模板
5. 触发执行
6. 在执行前安全扫描阶段命中命令黑名单并拦截

## 仓库内容

演示仓位于 [labs/blacklist-demo-playbooks/README.md](/root/auto-healing/labs/blacklist-demo-playbooks/README.md)，入口 Playbook 为 [destructive-demo.yml](/root/auto-healing/labs/blacklist-demo-playbooks/playbooks/destructive-demo.yml:1)。

当前 Playbook 使用了真实高风险命令：

- `rm -rf /`
- `iptables -F`
- `reboot`

这些命令对应 AHS 内置黑名单系统规则，适合验证执行前安全拦截。

## 推荐运行方式

优先复用现有租户和现有真实执行上下文，这样 UI 中能直接看到真实主机和真实密钥源，而不是 `localhost` + 空密钥源。

```bash
AHS_ADMIN_PASSWORD='<当前 AHS admin 密码>' \
TENANT_USERNAME='<现有租户成员用户名>' \
TENANT_LOGIN_PASSWORD='<该成员密码>' \
TARGET_TENANT_CODE='test' \
REAL_CONTEXT_TASK_NAME='故障实验-CPU恢复' \
bash tools/setup_blacklist_demo.sh
```

脚本行为：

- 如果传入 `TENANT_USERNAME` 和 `TENANT_LOGIN_PASSWORD`，则直接复用现有租户
- 默认优先复用 `REAL_CONTEXT_TASK_NAME` 对应任务里的 `target_hosts` 与 `secrets_source_ids`
- 如果没找到该任务，则回退到租户内第一条带真实主机和密钥源的任务上下文
- 只有未提供租户账号时，脚本才会创建隔离 demo 租户

## 预期结果

- Gitea 仓库推送成功
- AHS 租户、Git 仓库、Playbook、任务模板创建成功
- 执行记录最终状态为 `failed`
- 执行日志中包含 `security` 阶段的 `安全拦截` 信息

## 当前 test 租户验证记录

- 租户：`test`
- Gitea 仓库：`http://127.0.0.1:13000/gitadmin/blacklist-demo-playbooks.git`
- AHS 仓库：
  - 名称：`Blacklist Demo Repo`
  - ID：`d2382549-51d1-4b33-89eb-6fc4df2abdfe`
- Playbook：
  - 名称：`Blacklist Destructive Demo`
  - ID：`14ac865e-f1b7-4204-a9a6-792142df11ec`
- 任务模板：
  - 名称：`Blacklist Intercept Demo`
  - ID：`d7bd0cb6-64d3-45f3-8db2-91dcd11b3149`
  - 目标主机：`192.168.31.100`
  - 密钥源：
    - `OpenBao Real IP Passwords`
    - `OpenBao Real IP SSH Keys`
- 最新验证 Run：
  - ID：`4f149663-3470-42c8-9c06-8cbed216a7d0`
  - 状态：`failed`
  - `stderr`：`安全拦截：检测到 3 个高危指令`
- `security` 日志命中：
  - `删除根目录`，文件 `playbooks/destructive-demo.yml` 第 `7` 行
  - `清空防火墙规则`，文件 `playbooks/destructive-demo.yml` 第 `10` 行
  - `重启命令`，文件 `playbooks/destructive-demo.yml` 第 `13` 行
