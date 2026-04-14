# Fault Lab Playbooks Legacy

这是 legacy 版本的故障恢复仓库，用来模拟 AHS 部署前就已经存在的普通 Ansible 仓库。

它保留和主套件相同的 Playbook/roles 结构，但刻意不提供 `.auto-healing.yml`，用于验证：

- AHS 在没有高级参数配置时也能正常导入入口 Playbook
- 变量可以完全靠入口文件和递归依赖扫描得到
- 旧仓库也能被 Gitea + AHS 正常注册、扫描和展示

本仓库依然按一条完整恢复流水线组织：

1. 前置上下文采集
2. 故障诊断
3. 场景恢复
4. 恢复校验
5. 报告落盘

## Playbook 入口

- `playbooks/fault_recovery_suite.yml`
  通用恢复主流程，通过 `fault_type` 分支切换具体场景。
- `playbooks/service_down_recover.yml`
  服务故障专用入口。
- `playbooks/cpu_high_reset.yml`
  CPU 高负载故障专用入口。
- `playbooks/disk_full_reset.yml`
  磁盘占满故障专用入口。

## 套件结构

- `playbooks/roles/fault_lab_context`
  采集主机上下文、初始化报告目录与基础事实。
- `playbooks/roles/fault_lab_diagnose`
  按故障场景记录诊断证据。
- `playbooks/roles/fault_lab_service`
  服务故障恢复动作。
- `playbooks/roles/fault_lab_cpu`
  CPU 高负载恢复动作。
- `playbooks/roles/fault_lab_disk`
  磁盘占满恢复动作。
- `playbooks/roles/fault_lab_verify`
  按场景校验恢复结果。
- `playbooks/roles/fault_lab_report`
  输出恢复报告和证据索引。
- 无 `.auto-healing.yml`
  这正是 legacy 仓库和新仓库的区别。

## 推荐用法

按场景执行：

```bash
ansible-playbook -i inventory service_down_recover.yml
ansible-playbook -i inventory cpu_high_reset.yml
ansible-playbook -i inventory disk_full_reset.yml
```

走统一入口：

```bash
ansible-playbook -i inventory fault_recovery_suite.yml -e fault_type=service_down
ansible-playbook -i inventory fault_recovery_suite.yml -e fault_type=cpu_high
ansible-playbook -i inventory fault_recovery_suite.yml -e fault_type=disk_full
```

## 故障矩阵

- `192.168.31.100`
  - `service_down`
  - `cpu_high`
- `192.168.31.101`
  - `service_down`
  - `disk_full`
- `192.168.31.77`
  - `cpu_high`
