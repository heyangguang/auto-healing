UPDATE healing_flows
SET edges = COALESCE(
    (
        SELECT jsonb_agg(
            jsonb_set(
                jsonb_set(
                    elem - 'from' - 'to',
                    '{source}',
                    COALESCE(elem->'source', elem->'from', '""'::jsonb),
                    true
                ),
                '{target}',
                COALESCE(elem->'target', elem->'to', '""'::jsonb),
                true
            )
            ORDER BY ord
        )
        FROM jsonb_array_elements(edges::jsonb) WITH ORDINALITY AS t(elem, ord)
    ),
    '[]'::jsonb
)
WHERE edges IS NOT NULL
  AND (edges::text LIKE '%"from"%' OR edges::text LIKE '%"to"%');
