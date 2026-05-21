CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source TEXT NOT NULL,
    source_id TEXT NOT NULL,
    title TEXT,
    model TEXT,
    provider TEXT,
    directory TEXT,
    started_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    message_count INT DEFAULT 0,
    search_vector TSVECTOR,
    resume_cmd TEXT,
    UNIQUE(source, source_id)
);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES sessions(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT,
    seq INT,
    created_at TIMESTAMPTZ
);

CREATE INDEX idx_sessions_fts ON sessions USING GIN(search_vector);
CREATE INDEX idx_messages_session ON messages(session_id, seq);

-- Trigger to auto-update search_vector
CREATE OR REPLACE FUNCTION sessions_search_trigger() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', coalesce(NEW.title, '') || ' ' || coalesce(NEW.directory, '') || ' ' || coalesce(NEW.model, ''));
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trig_sessions_search
    BEFORE INSERT OR UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION sessions_search_trigger();
