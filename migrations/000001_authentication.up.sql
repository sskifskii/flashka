CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login varchar(32) NOT NULL,
    login_normalized varchar(32) NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT users_login_format CHECK (login ~ '^[A-Za-z0-9][A-Za-z0-9_-]{2,31}$'),
    CONSTRAINT users_login_normalized CHECK (login_normalized = lower(login) AND login_normalized COLLATE "C" ~ '^[a-z0-9][a-z0-9_-]{2,31}$')
);

CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    CONSTRAINT auth_sessions_token_hash_length CHECK (octet_length(token_hash) = 32),
    CONSTRAINT auth_sessions_expiry CHECK (expires_at > created_at),
    CONSTRAINT auth_sessions_last_seen CHECK (last_seen_at >= created_at AND last_seen_at <= expires_at)
);
CREATE INDEX auth_sessions_expires_at_idx ON auth_sessions(expires_at);

CREATE TABLE folders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id uuid,
    name varchar(80) NOT NULL,
    name_normalized varchar(80) NOT NULL,
    is_inbox boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT folders_user_id_id_unique UNIQUE (user_id, id),
    CONSTRAINT folders_parent_owner_fk FOREIGN KEY (user_id, parent_id)
        REFERENCES folders(user_id, id),
    CONSTRAINT folders_inbox_canonical CHECK (
        NOT is_inbox OR (parent_id IS NULL AND name = 'Inbox' AND name_normalized = 'inbox')
    ),
    CONSTRAINT folders_only_inbox_root CHECK (parent_id IS NOT NULL OR is_inbox)
);
CREATE UNIQUE INDEX folders_one_root_per_user_idx ON folders(user_id) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX folders_one_inbox_per_user_idx ON folders(user_id) WHERE is_inbox;
CREATE UNIQUE INDEX folders_sibling_name_idx ON folders(user_id, parent_id, name_normalized) WHERE parent_id IS NOT NULL;
CREATE INDEX folders_user_parent_idx ON folders(user_id, parent_id);
