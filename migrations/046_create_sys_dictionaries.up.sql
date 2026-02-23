-- 字典值管理表
CREATE TABLE IF NOT EXISTS sys_dictionaries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dict_type VARCHAR(64) NOT NULL,
    dict_key VARCHAR(64) NOT NULL,
    label VARCHAR(128) NOT NULL,
    label_en VARCHAR(128),
    color VARCHAR(32),
    tag_color VARCHAR(32),
    badge VARCHAR(32),
    icon VARCHAR(64),
    bg VARCHAR(32),
    extra JSONB,
    sort_order INT DEFAULT 0,
    is_system BOOLEAN DEFAULT true,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT now(),
    updated_at TIMESTAMP DEFAULT now(),
    UNIQUE(dict_type, dict_key)
);

CREATE INDEX idx_sys_dictionaries_type ON sys_dictionaries(dict_type);
CREATE INDEX idx_sys_dictionaries_active ON sys_dictionaries(is_active);
