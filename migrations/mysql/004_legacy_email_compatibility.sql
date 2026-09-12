-- Historical identities may share an email. Phone remains globally unique.
-- Login must reject ambiguous email matches rather than selecting the first user.
ALTER TABLE users DROP INDEX uq_user_email, ADD INDEX ix_user_email (email_key);
