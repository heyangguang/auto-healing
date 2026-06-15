ALTER TABLE incident_solution_templates
    ADD COLUMN IF NOT EXISTS resolution_template TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS work_notes_template TEXT NOT NULL DEFAULT '';

UPDATE incident_solution_templates
SET
    resolution_template = COALESCE(NULLIF(resolution_template, ''), conclusion_template, ''),
    work_notes_template = COALESCE(NULLIF(work_notes_template, ''), solution_template, '');
