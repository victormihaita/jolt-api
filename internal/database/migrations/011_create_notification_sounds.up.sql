-- Notification Sounds table for app sound library
CREATE TABLE IF NOT EXISTS notification_sounds (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL,
    filename   VARCHAR(255) UNIQUE NOT NULL,
    is_free    BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Index for filtering by free/premium
CREATE INDEX IF NOT EXISTS idx_notification_sounds_is_free ON notification_sounds(is_free) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_notification_sounds_deleted_at ON notification_sounds(deleted_at);

-- Seed data: 3 free, 5 premium
INSERT INTO notification_sounds (name, filename, is_free) VALUES
    ('Ambient', 'ambient.wav', TRUE),
    ('Ambient Soft', 'ambient2.wav', FALSE),
    ('Hop', 'hop.wav', TRUE),
    ('Progressive', 'progressive.wav', FALSE),
    ('Reverb', 'reverb.wav', FALSE),
    ('Rock', 'rock.wav', TRUE),
    ('Synth Pop', 'syntpop.wav', FALSE),
    ('Techno', 'techno.wav', FALSE);
