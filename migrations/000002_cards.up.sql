CREATE TABLE cards (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id uuid NOT NULL,
    front text NOT NULL,
    back text NOT NULL,
    correct_count integer NOT NULL DEFAULT 0,
    incorrect_count integer NOT NULL DEFAULT 0,
    correct_streak integer NOT NULL DEFAULT 0,
    interval_days integer,
    last_reviewed_at timestamptz,
    due_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT cards_user_id_id_unique UNIQUE (user_id, id),
    CONSTRAINT cards_folder_owner_fk FOREIGN KEY (user_id, folder_id)
        REFERENCES folders(user_id, id),
    CONSTRAINT cards_front_length CHECK (char_length(front) BETWEEN 1 AND 2000),
    CONSTRAINT cards_back_length CHECK (char_length(back) BETWEEN 1 AND 2000),
    CONSTRAINT cards_counts_nonnegative CHECK (
        correct_count >= 0 AND incorrect_count >= 0 AND correct_streak >= 0
    ),
    CONSTRAINT cards_streak_bounded CHECK (correct_streak <= correct_count),
    CONSTRAINT cards_interval_positive CHECK (interval_days IS NULL OR interval_days > 0),
    CONSTRAINT cards_schedule_state CHECK (
        (correct_count + incorrect_count = 0 AND interval_days IS NULL AND last_reviewed_at IS NULL AND due_at IS NULL)
        OR
        (correct_count + incorrect_count > 0 AND interval_days IS NOT NULL AND last_reviewed_at IS NOT NULL AND due_at IS NOT NULL)
    ),
    CONSTRAINT cards_updated_after_created CHECK (updated_at >= created_at)
);

CREATE INDEX cards_user_folder_created_idx
    ON cards(user_id, folder_id, created_at DESC, id DESC);
CREATE INDEX cards_user_due_idx ON cards(user_id, due_at);
