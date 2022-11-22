CREATE TYPE INTERVAL_TYPE AS ENUM (
    'weekly',
    'biweekly',
    'triweekly',
    'monthly'
);

CREATE TABLE IF NOT EXISTS channels (
    channel_id varchar NOT NULL,
    inviter varchar NOT NULL,
    interval INTERVAL_TYPE NOT NULL,
    weekday integer NOT NULL,
    hour integer NOT NULL,
    next_round timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,
    updated_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    CONSTRAINT channels_pk_channel_id PRIMARY KEY (channel_id)
);

CREATE TABLE IF NOT EXISTS members (
    id integer GENERATED ALWAYS AS IDENTITY NOT NULL,
    user_id varchar NOT NULL,
    channel_id varchar NOT NULL,
    is_active boolean DEFAULT false NOT NULL,
    country BYTEA,
    city BYTEA,
    timezone BYTEA,
    profile_type BYTEA,
    profile_link BYTEA,
    calendly_link BYTEA,
    created_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,
    updated_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    CONSTRAINT members_pk_id PRIMARY KEY (id),
    CONSTRAINT members_fk_channel_id FOREIGN KEY (channel_id) REFERENCES channels(channel_id) ON DELETE CASCADE,
    CONSTRAINT members_unique_user_id_per_channel UNIQUE (user_id, channel_id)
);

CREATE TABLE IF NOT EXISTS rounds (
    id integer GENERATED ALWAYS AS IDENTITY NOT NULL,
    channel_id varchar NOT NULL,
    has_ended boolean NOT NULL,
    created_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,
    updated_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    CONSTRAINT rounds_pk_id PRIMARY KEY (id),
    CONSTRAINT members_fk_channel_id FOREIGN KEY (channel_id) REFERENCES channels(channel_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS matches (
    id integer GENERATED ALWAYS AS IDENTITY NOT NULL,
    round_id integer NOT NULL,
    mpim_id varchar, -- the ID of the Slack group DM
    has_met boolean DEFAULT false NOT NULL,
    was_notified boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,
    updated_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    CONSTRAINT matches_pk_id PRIMARY KEY (id),
    CONSTRAINT matches_fk_round_id FOREIGN KEY (round_id) REFERENCES rounds(id) ON DELETE CASCADE
);

-- junction table
CREATE TABLE IF NOT EXISTS pairings (
    id integer GENERATED ALWAYS AS IDENTITY NOT NULL,
    match_id integer NOT NULL,
    member_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    CONSTRAINT pairings_pk_match_id_user_id PRIMARY KEY (
        match_id,
        member_id
    ),

    CONSTRAINT pairings_fk_match_id FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE,
    CONSTRAINT pairings_fk_member_id FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE CASCADE
);
