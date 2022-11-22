-- enums for the type of jobs
CREATE TYPE JOB_TYPE AS ENUM (
    'ADD_CHANNEL',
    'UPDATE_CHANNEL',
    'DELETE_CHANNEL',
    'SYNC_CHANNELS',
    'ADD_MEMBER',
    'GREET_MEMBER',
    'UPDATE_MEMBER',
    'DELETE_MEMBER',
    'SYNC_MEMBERS',
    'CREATE_ROUND',
    'END_ROUND',
    'CREATE_MATCHES',
    'UPDATE_MATCH',
    'CREATE_PAIR',
    'NOTIFY_PAIR',
    'CHECK_PAIR',
    'REPORT_STATS'
);

-- enums for the completion status of jobs
CREATE TYPE JOB_STATUS AS ENUM (
    'PENDING',
    'ERRORED',
    'CANCELED',
    'FAILED',
    'SUCCEEDED'
);

-- jobs table is a FIFO queue for performing background jobs
CREATE TABLE IF NOT EXISTS jobs (
    id integer GENERATED ALWAYS AS IDENTITY NOT NULL,
    job_id BYTEA NOT NULL,
    job_type JOB_TYPE NOT NULL,
    priority SMALLINT NOT NULL CHECK (priority > 0 AND priority < 11),
    status JOB_STATUS NOT NULL,
    is_completed boolean DEFAULT false NOT NULL,
    data JSONB,
    exec_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    created_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,
    updated_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    CONSTRAINT jobs_pk_id PRIMARY KEY (id)
);

-- GetNextJob() retrieves the next available job in the queue
CREATE OR REPLACE FUNCTION GetNextJob()
    RETURNS SETOF jobs
    AS $$
        SELECT *
        FROM jobs
        WHERE
            exec_at <= NOW()
            AND
            is_completed = false
            AND
            status = 'PENDING'
        ORDER BY priority DESC, created_at
        LIMIT 1
        FOR UPDATE SKIP LOCKED;
    $$
    language sql;
