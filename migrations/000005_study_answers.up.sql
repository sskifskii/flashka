CREATE TABLE review_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES study_sessions(id) ON DELETE CASCADE,
    card_id uuid REFERENCES cards(id) ON DELETE SET NULL,
    rating text NOT NULL,
    reviewed_early boolean NOT NULL,
    previous_interval_days integer,
    next_interval_days integer NOT NULL,
    previous_due_at timestamptz,
    next_due_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT review_events_rating_valid CHECK (rating IN ('correct', 'incorrect')),
    CONSTRAINT review_events_next_interval_positive CHECK (next_interval_days > 0),
    CONSTRAINT review_events_unique_answer UNIQUE (session_id, card_id)
);

CREATE INDEX review_events_user_created_idx ON review_events(user_id, created_at DESC);
