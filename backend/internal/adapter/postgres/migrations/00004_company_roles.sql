-- +goose Up
-- Company-wide roles (OmiTeam): one per account, plus addition roles such as LEAD.
ALTER TABLE users
    ADD COLUMN role TEXT NOT NULL DEFAULT 'EMPLOYEE' CHECK (role IN (
        'OWNER', 'ADMIN', 'MANAGER', 'RECRUITER', 'HR', 'SALES', 'EMPLOYEE', 'CONTRACTOR', 'ACCOUNTANT', 'CANDIDATE')),
    ADD COLUMN addition_roles TEXT[] NOT NULL DEFAULT '{}' CHECK (addition_roles <@ ARRAY['LEAD']);

-- The founding account owns the company.
UPDATE users SET role = 'OWNER' WHERE id = (SELECT min(id) FROM users);

-- +goose Down
ALTER TABLE users DROP COLUMN role, DROP COLUMN addition_roles;
