-- 048_seed_command_blacklist.up.sql
-- 内置高危指令黑名单（is_system=true 不可删除，is_active=false 默认未启用）
-- tenant_id = NULL 表示平台级规则，对所有租户生效

INSERT INTO command_blacklist (name, pattern, match_type, severity, category, description, is_active, is_system)
VALUES
  -- ── 文件系统 ──────────────────────────────────────────────
  ('删除根目录', 'rm -rf /', 'contains', 'critical', 'filesystem',
   '删除根目录，会导致系统完全损坏', false, true),

  ('强制删除系统目录', 'rm -rf /*', 'contains', 'critical', 'filesystem',
   '删除所有文件，会导致系统无法恢复', false, true),

  ('格式化磁盘(mkfs)', 'mkfs', 'contains', 'critical', 'filesystem',
   '格式化磁盘分区，会导致数据全部丢失', false, true),

  ('覆盖磁盘(dd)', 'dd if=/dev/zero of=/dev/', 'contains', 'critical', 'filesystem',
   '向磁盘设备写零，会彻底清除数据', false, true),

  ('写入 /dev/sda', 'of=/dev/sda', 'contains', 'critical', 'filesystem',
   '直接写入系统磁盘，极度危险', false, true),

  ('删除 /etc', 'rm -rf /etc', 'contains', 'critical', 'filesystem',
   '删除系统配置目录，系统将无法正常运行', false, true),

  ('删除 /boot', 'rm -rf /boot', 'contains', 'critical', 'filesystem',
   '删除引导分区，系统将无法启动', false, true),

  ('删除 /var', 'rm -rf /var', 'contains', 'critical', 'filesystem',
   '删除系统运行时目录，服务全部停止', false, true),

  -- ── 数据库 ────────────────────────────────────────────────
  ('删除所有数据库', 'DROP DATABASE', 'contains', 'critical', 'database',
   '删除数据库，所有数据将永久丢失', false, true),

  ('删除所有表', 'DROP TABLE', 'contains', 'high', 'database',
   '删除数据表，该表数据将永久丢失', false, true),

  ('清空表数据', 'TRUNCATE TABLE', 'contains', 'high', 'database',
   '清空表中所有数据', false, true),

  -- ── 网络 ─────────────────────────────────────────────────
  ('关闭防火墙(iptables)', 'iptables -F', 'contains', 'high', 'network',
   '清空防火墙规则，系统将暴露在网络攻击中', false, true),

  ('关闭 firewalld', 'systemctl stop firewalld', 'contains', 'high', 'network',
   '停止防火墙服务', false, true),

  ('禁用 firewalld 自启', 'systemctl disable firewalld', 'contains', 'high', 'network',
   '禁止防火墙开机自启', false, true),

  -- ── 系统 ─────────────────────────────────────────────────
  ('关机', 'shutdown -h now', 'contains', 'critical', 'system',
   '立即关闭系统', false, true),

  ('重启', 'reboot', 'exact', 'high', 'system',
   '立即重启系统', false, true),

  ('修改 root 密码', 'passwd root', 'contains', 'critical', 'system',
   '修改 root 账户密码', false, true),

  ('关闭 SELinux', 'setenforce 0', 'contains', 'high', 'system',
   '临时关闭 SELinux 强制模式', false, true),

  ('停用 SELinux', 'SELINUX=disabled', 'contains', 'high', 'system',
   '永久关闭 SELinux', false, true),

  ('Fork 炸弹', ':(){ :|:& };:', 'contains', 'critical', 'system',
   'Fork 炸弹，会耗尽系统资源导致宕机', false, true),

  ('覆盖系统文件', '> /etc/passwd', 'contains', 'critical', 'system',
   '覆盖用户数据库文件，导致所有账号失效', false, true),

  ('删除历史记录', 'history -c', 'contains', 'high', 'system',
   '清空 shell 历史记录，破坏审计追踪', false, true)

ON CONFLICT DO NOTHING;
