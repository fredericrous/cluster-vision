-- Who asserted a dependency. The AI enricher re-infers dependencies on every
-- run and must be able to drop the ones it no longer infers without touching
-- anything a person recorded. NULL means "recorded before origin was
-- tracked" (the removed authoring API, or an earlier enricher run): unknown,
-- so it is never pruned automatically.
ALTER TABLE app_dependencies ADD COLUMN origin data_source;
