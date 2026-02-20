-- 租户表添加 icon 字段（存图标名：bank, shop, team, cloud 等）
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS icon VARCHAR(50) DEFAULT '';
