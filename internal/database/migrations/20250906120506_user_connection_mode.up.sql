ALTER TABLE members ADD COLUMN connection_mode CONNECTION_MODE DEFAULT 'hybrid';
ALTER TABLE members ALTER COLUMN connection_mode SET NOT NULL;

-- Speed up connection_mode matching
CREATE INDEX idx_members_connection_mode ON members(connection_mode);


-- GetRandomMatchesV4() retrieves a randomized set of matches for a round of Chat Roulette
-- while respecting those members' who prefer to be matched with the same gender (has_gender_preference = true),
-- preferring compatible connection modes (virtual/physical/hybrid),
-- ensuring that users who have blocked each other are never matched together,
-- and preferring new matches over repeat matches from previous rounds.
--
-- To maximize the number of matches, the function prioritizes in this order:
-- 1. Users with gender preference: same gender + compatible connection mode + new partners first
-- 2. Users with gender preference: same gender + any connection mode + allow previous partners
-- 3. Remaining users: compatible connection mode + new partners first, then allow previous + any mode
-- Any participants who could not be matched are returned with the "partner" column set to null.
--
-- Example Usage:
-- SELECT * FROM GetRandomMatchesV4('C122315531');
CREATE OR REPLACE FUNCTION GetRandomMatchesV4(p_channel_id VARCHAR)
RETURNS TABLE(participant VARCHAR, partner VARCHAR)
AS $$
DECLARE
    v_user RECORD;
    v_partner VARCHAR;
    v_matched_users VARCHAR[] := ARRAY[]::VARCHAR[];
BEGIN
    -- Match users with gender preference
    FOR v_user IN (
        SELECT user_id, gender, connection_mode
        FROM members
        WHERE channel_id = p_channel_id
            AND is_active
            AND has_gender_preference
        ORDER BY RANDOM()
    ) LOOP
        IF v_user.user_id = ANY(v_matched_users) THEN
            CONTINUE;
        END IF;

        -- Single query with smart ordering to handle all gender preference matching
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
        ORDER BY
            -- 1. Prioritize compatible connection modes
            CASE WHEN (v_user.connection_mode = 'hybrid' OR m.connection_mode = 'hybrid'
                       OR v_user.connection_mode = m.connection_mode) THEN 0 ELSE 1 END,
            -- 2. Prefer users who haven't been matched before (NEW partners)
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
            -- 3. Among users with same preference, prefer those who also have gender preference
            has_gender_preference DESC,
            -- 4. Random selection within same priority group
            RANDOM()
        LIMIT 1;

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
        SELECT user_id, connection_mode
        FROM members
        WHERE channel_id = p_channel_id
            AND is_active
            AND user_id != ALL(v_matched_users)
        ORDER BY RANDOM()
    ) LOOP
        IF v_user.user_id = ANY(v_matched_users) THEN
            CONTINUE;
        END IF;

        -- Try to match with any available partner (prioritize compatible connection mode + new partners)
        SELECT user_id INTO v_partner
        FROM members m
        WHERE m.channel_id = p_channel_id
            AND m.is_active
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
        ORDER BY
            -- Prioritize compatible connection modes (hybrid is compatible with all)
            CASE WHEN (v_user.connection_mode = 'hybrid' OR m.connection_mode = 'hybrid'
                       OR v_user.connection_mode = m.connection_mode) THEN 0 ELSE 1 END,
            -- Prefer users who haven't been matched before (NEW partners)
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

DROP FUNCTION GetRandomMatchesV3;
