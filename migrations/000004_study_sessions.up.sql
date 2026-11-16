CREATE TABLE study_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope_type text NOT NULL,
    scope_folder_id uuid,
    requested_count smallint NOT NULL,
    actual_count smallint NOT NULL,
    status text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    CONSTRAINT study_sessions_user_id_id_unique UNIQUE (user_id, id),
    CONSTRAINT study_sessions_scope_fk FOREIGN KEY (user_id, scope_folder_id)
        REFERENCES folders(user_id, id),
    CONSTRAINT study_sessions_scope_valid CHECK (
        (scope_type = 'all' AND scope_folder_id IS NULL)
        OR (scope_type = 'folder' AND scope_folder_id IS NOT NULL)
    ),
    CONSTRAINT study_sessions_count_valid CHECK (
        requested_count BETWEEN 1 AND 100
        AND actual_count BETWEEN 1 AND requested_count
    ),
    CONSTRAINT study_sessions_status_valid CHECK (status IN ('active', 'completed', 'abandoned')),
    CONSTRAINT study_sessions_completion_valid CHECK (
        (status = 'active' AND completed_at IS NULL)
        OR (status <> 'active' AND completed_at IS NOT NULL)
    )
);

CREATE TABLE study_session_cards (
    session_id uuid NOT NULL REFERENCES study_sessions(id) ON DELETE CASCADE,
    card_id uuid REFERENCES cards(id) ON DELETE SET NULL,
    position smallint NOT NULL,
    selection_bucket text NOT NULL,
    revealed_at timestamptz,
    result text,
    answered_at timestamptz,
    PRIMARY KEY (session_id, position),
    CONSTRAINT study_session_cards_unique_card UNIQUE (session_id, card_id),
    CONSTRAINT study_session_cards_position_valid CHECK (position > 0),
    CONSTRAINT study_session_cards_bucket_valid CHECK (selection_bucket IN ('due', 'new', 'early')),
    CONSTRAINT study_session_cards_result_valid CHECK (result IS NULL OR result IN ('correct', 'incorrect')),
    CONSTRAINT study_session_cards_answer_valid CHECK (
        (result IS NULL AND answered_at IS NULL)
        OR (result IS NOT NULL AND revealed_at IS NOT NULL AND answered_at IS NOT NULL)
    )
);

CREATE INDEX study_sessions_user_status_created_idx
    ON study_sessions(user_id, status, created_at DESC);
