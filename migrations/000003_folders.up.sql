ALTER TABLE folders
    ADD CONSTRAINT folders_name_length CHECK (char_length(name) BETWEEN 1 AND 80),
    ADD CONSTRAINT folders_normalized_name_length CHECK (char_length(name_normalized) BETWEEN 1 AND 80),
    ADD CONSTRAINT folders_updated_after_created CHECK (updated_at >= created_at);
