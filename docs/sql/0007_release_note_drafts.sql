-- 发布说明草稿表（支持审计）
CREATE TABLE IF NOT EXISTS release_note_drafts (
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(200) NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    version     VARCHAR(50)  NOT NULL DEFAULT '',
    created_by  VARCHAR(100) NOT NULL DEFAULT '',
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_release_note_drafts_created_at ON release_note_drafts (created_at DESC);
