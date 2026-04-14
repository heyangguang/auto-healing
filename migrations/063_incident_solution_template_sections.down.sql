ALTER TABLE incident_solution_templates
    DROP COLUMN IF EXISTS step_output_max_length,
    DROP COLUMN IF EXISTS steps_max_count,
    DROP COLUMN IF EXISTS steps_render_mode,
    DROP COLUMN IF EXISTS conclusion_template,
    DROP COLUMN IF EXISTS verification_template,
    DROP COLUMN IF EXISTS solution_template,
    DROP COLUMN IF EXISTS problem_template;
