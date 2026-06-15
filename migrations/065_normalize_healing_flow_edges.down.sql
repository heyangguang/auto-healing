UPDATE healing_flows
SET edges = COALESCE(
    (
        SELECT jsonb_agg(
            jsonb_set(
                jsonb_set(
                    elem,
                    '{from}',
                    COALESCE(elem->'source', '""'::jsonb),
                    true
                ),
                '{to}',
                COALESCE(elem->'target', '""'::jsonb),
                true
            )
            ORDER BY ord
        )
        FROM jsonb_array_elements(edges::jsonb) WITH ORDINALITY AS t(elem, ord)
    ),
    '[]'::jsonb
)
WHERE edges IS NOT NULL;
