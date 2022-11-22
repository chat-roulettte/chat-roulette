-- GetRandomMatches() retrieves a randomized set of possible matches for a round of chat roulette
--
-- Example Usage:
-- SELECT * FROM GetRandomMatches('CHANNEL-ID');
CREATE OR REPLACE FUNCTION GetRandomMatches(channel_id varchar)
    RETURNS TABLE (participant varchar, partner varchar)
    AS $$
        SELECT a.user_id AS participant, b.user_id AS partner
        FROM members a
        CROSS JOIN (
            SELECT id, user_id
            FROM members
            WHERE channel_id = $1 AND is_active = true
            ORDER BY created_at DESC
        ) AS b
        WHERE channel_id = $1 AND is_active = true AND a.id <> b.id
        ORDER BY RANDOM()
    $$
    language sql;
