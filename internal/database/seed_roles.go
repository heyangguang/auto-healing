package database

// SystemRoles 系统预置角色及其默认权限
var SystemRoles = append(append([]RoleSeed{}, PlatformSystemRoles...), TenantSystemRoles...)
