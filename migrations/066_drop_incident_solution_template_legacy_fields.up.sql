UPDATE incident_solution_templates
SET
    conclusion_template = COALESCE(NULLIF(conclusion_template, ''), resolution_template, ''),
    solution_template = COALESCE(NULLIF(solution_template, ''), work_notes_template, '')
WHERE (conclusion_template = '' OR solution_template = '')
  AND (resolution_template <> '' OR work_notes_template <> '');

ALTER TABLE incident_solution_templates
    DROP COLUMN IF EXISTS resolution_template,
    DROP COLUMN IF EXISTS work_notes_template;
