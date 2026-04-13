-- Message audio storage for voice messages

CREATE TABLE IF NOT EXISTS message_audio (
    message_id BIGINT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    audio_data BYTEA NOT NULL,           -- OGG audio data
    duration_ms INT,                     -- Duration in milliseconds
    waveform JSONB,                      -- Waveform amplitudes for visualization [0.0-1.0]
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_message_audio_created ON message_audio(created_at);

-- Comment
COMMENT ON TABLE message_audio IS 'Stores audio data for voice messages with waveform visualization data';
COMMENT ON COLUMN message_audio.audio_data IS 'OGG-encoded audio data';
COMMENT ON COLUMN message_audio.waveform IS 'Array of normalized amplitude values for UI waveform visualization';
