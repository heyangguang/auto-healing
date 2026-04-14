# 故障注入实验说明

这套脚本用于在三台实验机上稳定制造和恢复 3 类故障：

- `service_down`
- `cpu_high`
- `disk_full`

## 故障矩阵

- `192.168.31.100`
  - `service_down`
  - `cpu_high`
- `192.168.31.101`
  - `service_down`
  - `disk_full`
- `192.168.31.77`
  - `cpu_high`

## 文件

- 远端脚本: [tools/fault_lab/auto_healing_fault_lab.sh](/root/auto-healing/tools/fault_lab/auto_healing_fault_lab.sh)
- 测试服务 unit: [tools/fault_lab/auto-healing-lab-http.service](/root/auto-healing/tools/fault_lab/auto-healing-lab-http.service)
- 部署脚本: [tools/fault_lab/deploy_fault_lab.sh](/root/auto-healing/tools/fault_lab/deploy_fault_lab.sh)
- 本机控制脚本: [tools/fault_lab/control_faults.sh](/root/auto-healing/tools/fault_lab/control_faults.sh)

## 部署

```bash
cd /root/auto-healing
FAULT_HOST_100_PASSWORD='Heyang2015.' \
FAULT_HOST_101_PASSWORD='123' \
tools/fault_lab/deploy_fault_lab.sh
```

部署完成后，三台机会安装一个专门的测试服务：

- systemd 名称: `auto-healing-lab-http.service`
- 端口: `19081`

`service_down` 只会停掉这一个测试服务，不会碰业务服务。

## 本机控制

控制脚本会自动读取本地配置：

- 示例: [tools/fault_lab/.fault_lab.env.example](/root/auto-healing/tools/fault_lab/.fault_lab.env.example)
- 本地实际配置: `tools/fault_lab/.fault_lab.env`（已加入本地忽略）

常用命令：

```bash
cd /root/auto-healing
tools/fault_lab/control_faults.sh matrix
tools/fault_lab/control_faults.sh status all
tools/fault_lab/control_faults.sh inject matrix
tools/fault_lab/control_faults.sh reset matrix
tools/fault_lab/control_faults.sh inject 100 service_down
tools/fault_lab/control_faults.sh inject 100 cpu_high 2
tools/fault_lab/control_faults.sh inject 101 disk_full 92
tools/fault_lab/control_faults.sh reset 100 service_down
tools/fault_lab/control_faults.sh reset 100 cpu_high
tools/fault_lab/control_faults.sh reset 101 disk_full
tools/fault_lab/control_faults.sh inject 77 cpu_high 2
tools/fault_lab/control_faults.sh reset 77 cpu_high
```

## 远端命令

脚本路径：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh
```

查看全部状态：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh status all
```

### service_down

注入：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh inject service_down
```

恢复：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh reset service_down
```

### cpu_high

注入 2 个 CPU 压测进程：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh inject cpu_high 2
```

恢复：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh reset cpu_high
```

### disk_full

把根分区打到 92%：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh inject disk_full 92
```

恢复：

```bash
/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh reset disk_full
```

## 说明

- `cpu_high` 会记录 PID，`reset` 时统一回收。
- `disk_full` 会在 `/opt/auto-healing-fault-lab/disk-fill.bin` 创建占位文件，并保留至少 `512MB` 安全余量。
- 所有状态文件都放在 `/opt/auto-healing-fault-lab/state/`。
