CREATE TYPE CONNECTION_MODE AS ENUM (
    'virtual',
    'physical',
    'hybrid'
);

ALTER TABLE channels ADD COLUMN connection_mode CONNECTION_MODE DEFAULT 'virtual';

ALTER TABLE channels ALTER COLUMN connection_mode SET NOT NULL;
