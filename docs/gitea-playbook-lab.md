# 本地 Git Playbook 联调说明

这套环境用于把故障自愈 Ansible Playbook 放到本地 Git 服务，再由 AHS 注册和扫描。

## 服务信息

- Gitea URL: `http://127.0.0.1:13000`
- 管理员: `gitadmin`
- 密码: `GitAdmin123!`
- 套件仓库: `http://127.0.0.1:13000/gitadmin/fault-playbooks.git`
- legacy 仓库: `http://127.0.0.1:13000/gitadmin/fault-playbooks-legacy.git`

## 文件

- Compose: [deployments/docker/docker-compose.gitea.yml](/root/auto-healing/deployments/docker/docker-compose.gitea.yml)
- 环境变量示例: [deployments/docker/gitea.env.example](/root/auto-healing/deployments/docker/gitea.env.example)
- Playbook 套件仓库目录: [labs/fault-playbooks](/root/auto-healing/labs/fault-playbooks)
- Playbook legacy 仓库目录: [labs/fault-playbooks-legacy](/root/auto-healing/labs/fault-playbooks-legacy)

## 已推送的 Playbook

- `playbooks/fault_recovery_suite.yml`
- `playbooks/service_down_recover.yml`
- `playbooks/cpu_high_reset.yml`
- `playbooks/disk_full_reset.yml`

## 两套仓库的区别

- `fault-playbooks`
  带 `.auto-healing.yml`，用于演示 AHS 高级变量与按入口作用域控制
- `fault-playbooks-legacy`
  不带 `.auto-healing.yml`，用于模拟 AHS 部署前就已经存在的普通 Ansible 仓库

## AHS 中已注册的 Git 仓库

- 名称: `Local Fault Playbooks`
- 仓库 ID: `61066d72-2543-418d-b706-f99d2f5e3267`
- URL: `http://127.0.0.1:13000/gitadmin/fault-playbooks.git`
- 名称: `Local Fault Playbooks Legacy`
- 仓库 ID: `c0d9c63c-4ac8-4aec-a30c-7d9711d00e07`
- URL: `http://127.0.0.1:13000/gitadmin/fault-playbooks-legacy.git`

## AHS 中已创建并扫描的 Playbook

- `Fault Recovery Suite`
- `Service Down Recover`
- `CPU High Reset`
- `Disk Full Reset`
- `Fault Recovery Suite Legacy`
- `Service Down Recover Legacy`
- `CPU High Reset Legacy`
- `Disk Full Reset Legacy`

当前套件仓库下 4 个 Playbook 都已经是 `scanned`，并按高级参数作用域收敛为 `13/6/6/6`。

legacy 仓库下 4 个 Playbook 也已经是 `scanned`，因为没有 `.auto-healing.yml`，所以 4 个入口当前都暴露 `14` 个扫描变量。
