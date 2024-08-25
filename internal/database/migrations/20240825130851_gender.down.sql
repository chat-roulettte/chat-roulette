DROP FUNCTION GetRandomMatchesV2;

ALTER TABLE members DROP COLUMN has_gender_preference;

ALTER TABLE members DROP COLUMN gender;

DROP TYPE GENDER;
