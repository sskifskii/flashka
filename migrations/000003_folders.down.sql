ALTER TABLE folders
    DROP CONSTRAINT folders_updated_after_created,
    DROP CONSTRAINT folders_normalized_name_length,
    DROP CONSTRAINT folders_name_length;
