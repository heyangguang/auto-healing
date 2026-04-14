ALTER TABLE healing_flows
    ADD COLUMN IF NOT EXISTS close_policy JSONB NOT NULL DEFAULT '{}'::jsonb;
