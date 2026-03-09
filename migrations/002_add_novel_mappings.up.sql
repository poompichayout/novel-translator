CREATE TABLE novel_mappings (
    id SERIAL PRIMARY KEY,
    source_novel_id INTEGER REFERENCES novels(id) ON DELETE CASCADE,
    target_novel_id INTEGER REFERENCES novels(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_novel_id, target_novel_id)
);
