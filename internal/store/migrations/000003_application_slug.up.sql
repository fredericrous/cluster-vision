-- CI slug: the suite-wide join key (application-landscape, ticket-vision,
-- kb-vision all link configuration items by this slug, never by UUID).
-- Derived deterministically from the discovered name; collisions get a
-- short id suffix so the value is stable once assigned.
ALTER TABLE applications ADD COLUMN slug TEXT;

UPDATE applications
SET slug = trim(both '-' from regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g'));

UPDATE applications SET slug = left(id::text, 8) WHERE slug IS NULL OR slug = '';

WITH ranked AS (
    SELECT id, row_number() OVER (PARTITION BY slug ORDER BY created_at, id) AS rn
    FROM applications
)
UPDATE applications a
SET slug = a.slug || '-' || left(a.id::text, 8)
FROM ranked r
WHERE r.id = a.id AND r.rn > 1;

ALTER TABLE applications ALTER COLUMN slug SET NOT NULL;
CREATE UNIQUE INDEX idx_applications_slug ON applications (slug);
