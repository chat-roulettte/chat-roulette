CREATE TYPE GENDER AS ENUM (
    'male',
    'female'
);

ALTER TABLE members ADD COLUMN gender GENDER DEFAULT 'male';
ALTER TABLE members ALTER COLUMN gender SET NOT NULL;

ALTER TABLE members ADD COLUMN has_gender_preference boolean DEFAULT false;
ALTER TABLE members ALTER COLUMN has_gender_preference SET NOT NULL;

-- GetRandomMatchesV2() retrieves a randomized set of matches for a round of Chat Roulette
-- while respecting those members' who prefer to be matched with the same gender (has_gender_preference = true).
-- To maximize the number of matches, members who prefer being matched with the same gender are matched first
-- before others. If no match with gender preference is found, it tries to match with users without preference.
-- Any participants who could not be matched are returned with the "partner" column set to null.
--
-- Example Usage:
-- SELECT * FROM GetRandomMatchesV2('C122315531');
CREATE OR REPLACE FUNCTION GetRandomMatchesV2(p_channel_id VARCHAR)
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

        -- Try to match with a user who has gender preference
        SELECT user_id INTO v_partner
        FROM members
        WHERE channel_id = p_channel_id
            AND is_active
            AND gender = v_user.gender
            AND user_id != v_user.user_id
            AND user_id != ALL(v_matched_users)
        ORDER BY has_gender_preference DESC
        LIMIT 1;

        IF v_partner IS NOT NULL THEN
            v_matched_users := v_matched_users || v_user.user_id || v_partner;
            RETURN QUERY SELECT v_user.user_id::VARCHAR, v_partner::VARCHAR;
        ELSE
            v_matched_users := v_matched_users || v_user.user_id;
            RETURN QUERY SELECT v_user.user_id::VARCHAR, NULL::VARCHAR;
        END IF;
    END LOOP;

    -- Match remaining users who dont have gender preference
    FOR v_user IN (
        SELECT user_id
        FROM members
        WHERE channel_id = p_channel_id AND is_active AND user_id != ALL(v_matched_users)
        ORDER BY RANDOM()
    ) LOOP
        IF v_user.user_id = ANY(v_matched_users) THEN
            CONTINUE;
        END IF;

        SELECT user_id INTO v_partner
        FROM members
        WHERE channel_id = p_channel_id
            AND is_active
            AND user_id != v_user.user_id
            AND user_id != ALL(v_matched_users)
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
