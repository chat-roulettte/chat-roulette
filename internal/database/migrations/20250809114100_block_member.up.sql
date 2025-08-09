CREATE TABLE IF NOT EXISTS blocked_members (
    id integer GENERATED ALWAYS AS IDENTITY NOT NULL,
    channel_id varchar NOT NULL,
    user_id varchar NOT NULL,    -- The Slack ID of the user who is doing the blocking
    member_id varchar NOT NULL,    -- The Slack ID of the user who is being blocked

    created_at timestamp without time zone DEFAULT NOW()::timestamp NOT NULL,

    CONSTRAINT blocked_members_pk_id PRIMARY KEY (id),
    CONSTRAINT blocked_members_fk_channel_id FOREIGN KEY (channel_id) REFERENCES channels(channel_id) ON DELETE CASCADE,
    CONSTRAINT blocked_members_fk_user_id FOREIGN KEY (channel_id, user_id) REFERENCES members(channel_id, user_id) ON DELETE CASCADE,
    CONSTRAINT blocked_members_unique_block UNIQUE (channel_id, user_id, member_id),
    CONSTRAINT blocked_members_no_self_block CHECK (user_id != member_id)
);

CREATE INDEX idx_blocked_members_lookup ON blocked_members(channel_id, user_id, member_id);

ALTER TYPE JOB_TYPE ADD VALUE 'BLOCK_MEMBER';
ALTER TYPE JOB_TYPE ADD VALUE 'UNBLOCK_MEMBER';

-- Speed up previous match history checks
CREATE INDEX idx_rounds_channel ON rounds(channel_id);
CREATE INDEX idx_matches_round ON matches(round_id);
CREATE INDEX idx_pairings_match ON pairings(match_id, member_id);

-- GetRandomMatchesV3() retrieves a randomized set of matches for a round of Chat Roulette
-- while respecting those members' who prefer to be matched with the same gender (has_gender_preference = true),
-- ensuring that users who have blocked each other are never matched together,
-- and preferring new matches over repeat matches from previous rounds.
-- 
-- To maximize the number of matches, the function prioritizes in this order:
-- 1. Users with gender preference matched to same gender (new partners first)
-- 2. Users with gender preference matched to same gender (previous partners as fallback)
-- 3. Remaining users matched to anyone (new partners first)
-- 4. Remaining users matched to anyone (previous partners as fallback)
-- 
-- Any participants who could not be matched are returned with the "partner" column set to null.
--
-- Example Usage:
-- SELECT * FROM GetRandomMatchesV3('C122315531');
CREATE OR REPLACE FUNCTION GetRandomMatchesV3(p_channel_id VARCHAR)
RETURNS TABLE(participant VARCHAR, partner VARCHAR)
AS $$
DECLARE
    v_user RECORD;
    v_partner VARCHAR;
    v_matched_users VARCHAR[] := ARRAY[]::VARCHAR[];
BEGIN
    -- Match users with gender preference
    FOR v_user IN (
        SELECT user_id, gender
        FROM members
        WHERE channel_id = p_channel_id
            AND is_active
            AND has_gender_preference
        ORDER BY RANDOM()
    ) LOOP
        IF v_user.user_id = ANY(v_matched_users) THEN
            CONTINUE;
        END IF;

        -- Try to match with a user who has gender preference, is not blocked, and has not been matched with before
        SELECT user_id INTO v_partner
        FROM members m
        WHERE m.channel_id = p_channel_id
            AND m.is_active
            AND m.gender = v_user.gender
            AND m.user_id != v_user.user_id
            AND m.user_id != ALL(v_matched_users)
            -- Exclude users that the current user has blocked
            AND NOT EXISTS (
                SELECT 1 FROM blocked_members bm1
                WHERE bm1.channel_id = p_channel_id
                    AND bm1.user_id = v_user.user_id
                    AND bm1.member_id = m.user_id
            )
            -- Exclude users that have blocked the current user
            AND NOT EXISTS (
                SELECT 1 FROM blocked_members bm2
                WHERE bm2.channel_id = p_channel_id
                    AND bm2.user_id = m.user_id
                    AND bm2.member_id = v_user.user_id
            )
            -- Exclude users who have been previously matched with the current user (NEW partners only)
            AND NOT EXISTS (
                SELECT 1
                FROM rounds r
                INNER JOIN matches mt ON r.id = mt.round_id
                INNER JOIN pairings p1 ON mt.id = p1.match_id
                INNER JOIN pairings p2 ON mt.id = p2.match_id AND p1.member_id != p2.member_id
                INNER JOIN members m1 ON p1.member_id = m1.id
                INNER JOIN members m2 ON p2.member_id = m2.id
                WHERE r.channel_id = p_channel_id
                    AND ((m1.user_id = v_user.user_id AND m2.user_id = m.user_id)
                         OR (m1.user_id = m.user_id AND m2.user_id = v_user.user_id))
            )
        ORDER BY has_gender_preference DESC
        LIMIT 1;

        -- If no new partner found, try to match with a PREVIOUS partner (gender preference)
        IF v_partner IS NULL THEN
            SELECT user_id INTO v_partner
            FROM members m
            WHERE m.channel_id = p_channel_id
                AND m.is_active
                AND m.gender = v_user.gender
                AND m.user_id != v_user.user_id
                AND m.user_id != ALL(v_matched_users)
                -- Exclude users that current user has blocked
                AND NOT EXISTS (
                    SELECT 1 FROM blocked_members bm1
                    WHERE bm1.channel_id = p_channel_id
                        AND bm1.user_id = v_user.user_id
                        AND bm1.member_id = m.user_id
                )
                -- Exclude users that have blocked the current user
                AND NOT EXISTS (
                    SELECT 1 FROM blocked_members bm2
                    WHERE bm2.channel_id = p_channel_id
                        AND bm2.user_id = m.user_id
                        AND bm2.member_id = v_user.user_id
                )
            ORDER BY has_gender_preference DESC
            LIMIT 1;
        END IF;

        IF v_partner IS NOT NULL THEN
            v_matched_users := v_matched_users || v_user.user_id || v_partner;
            RETURN QUERY SELECT v_user.user_id::VARCHAR, v_partner::VARCHAR;
        ELSE
            v_matched_users := v_matched_users || v_user.user_id;
            RETURN QUERY SELECT v_user.user_id::VARCHAR, NULL::VARCHAR;
        END IF;
    END LOOP;

    -- Match remaining users who don't have gender preference
    FOR v_user IN (
        SELECT user_id
        FROM members
        WHERE channel_id = p_channel_id
            AND is_active
            AND user_id != ALL(v_matched_users)
        ORDER BY RANDOM()
    ) LOOP
        IF v_user.user_id = ANY(v_matched_users) THEN
            CONTINUE;
        END IF;

        -- Try to match with any available partner (prefer new, allow previous if needed)
        SELECT user_id INTO v_partner
        FROM members m
        WHERE m.channel_id = p_channel_id
            AND m.is_active
            AND m.user_id != v_user.user_id
            AND m.user_id != ALL(v_matched_users)
            -- Exclude users that the current user has blocked
            AND NOT EXISTS (
                SELECT 1 FROM blocked_members bm1
                WHERE bm1.channel_id = p_channel_id
                    AND bm1.user_id = v_user.user_id
                    AND bm1.member_id = m.user_id
            )
            -- Exclude users that have blocked the current user
            AND NOT EXISTS (
                SELECT 1 FROM blocked_members bm2
                WHERE bm2.channel_id = p_channel_id
                    AND bm2.user_id = m.user_id
                    AND bm2.member_id = v_user.user_id
            )
        ORDER BY
            -- Prefer users who haven't been matched before (NEW partners first)
            CASE WHEN EXISTS (
                SELECT 1
                FROM rounds r
                INNER JOIN matches mt ON r.id = mt.round_id
                INNER JOIN pairings p1 ON mt.id = p1.match_id
                INNER JOIN pairings p2 ON mt.id = p2.match_id AND p1.member_id != p2.member_id
                INNER JOIN members m1 ON p1.member_id = m1.id
                INNER JOIN members m2 ON p2.member_id = m2.id
                WHERE r.channel_id = p_channel_id
                    AND ((m1.user_id = v_user.user_id AND m2.user_id = m.user_id)
                         OR (m1.user_id = m.user_id AND m2.user_id = v_user.user_id))
            ) THEN 1 ELSE 0 END,
            RANDOM()
        LIMIT 1;

        IF v_partner IS NOT NULL THEN
            v_matched_users := v_matched_users || v_user.user_id || v_partner;
            RETURN QUERY SELECT v_user.user_id::VARCHAR, v_partner::VARCHAR;
        ELSE
            RETURN QUERY SELECT v_user.user_id::VARCHAR, NULL::VARCHAR;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

DROP FUNCTION GetRandomMatchesV2;
