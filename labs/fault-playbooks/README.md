# Fault Lab Playbooks

本仓库是一套针对故障实验室的自愈 Playbook 套件，不只是单个修复脚本，而是按一条完整恢复流水线组织：

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
- `.auto-healing.yml`
  给 AHS 扫描用的变量定义。

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
