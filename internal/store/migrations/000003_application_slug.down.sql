DROP INDEX IF EXISTS idx_applications_slug;
ALTER TABLE applications DROP COLUMN IF EXISTS slug;
