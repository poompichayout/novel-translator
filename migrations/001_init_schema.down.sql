DROP TRIGGER IF EXISTS update_entities_updated_at ON entities;
DROP TRIGGER IF EXISTS update_chapters_updated_at ON chapters;
DROP TRIGGER IF EXISTS update_novels_updated_at ON novels;
DROP FUNCTION IF EXISTS update_updated_at_column;

DROP TABLE IF EXISTS scrape_jobs;
DROP TABLE IF EXISTS translation_pairs;
DROP TABLE IF EXISTS entities;
DROP TABLE IF EXISTS chapters;
DROP TABLE IF EXISTS novels;

DROP TYPE IF EXISTS entity_type;
DROP TYPE IF EXISTS process_status;
