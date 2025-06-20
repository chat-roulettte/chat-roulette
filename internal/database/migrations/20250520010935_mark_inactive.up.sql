ALTER TYPE JOB_TYPE ADD VALUE 'MARK_INACTIVE';

ALTER TABLE rounds ADD COLUMN inactive_users smallint DEFAULT 0;
ALTER TABLE rounds ALTER COLUMN inactive_users SET NOT NULL;
ALTER TABLE rounds ADD CONSTRAINT rounds_ck_inactive_users_non_negative CHECK (inactive_users >= 0);
