-- +goose Up
-- Organizations become isolated tenants. A person can belong to several; their company role (OWNER…
-- CANDIDATE, LEAD), whether they still work there and why they left are per organization, and so is
-- every piece of HR data. The org roles owner/admin/member are replaced by the company roles.

-- Everyone must belong somewhere: with no organization at all, the oldest account founds one, and
-- people outside every organization join the oldest one (before, the whole system was one company).
INSERT INTO organizations (name, created_by)
SELECT 'Company', (SELECT min(id) FROM users)
WHERE NOT EXISTS (SELECT 1 FROM organizations) AND EXISTS (SELECT 1 FROM users);

INSERT INTO organization_members (organization_id, user_id, role)
SELECT (SELECT min(id) FROM organizations), u.id, CASE WHEN u.role = 'OWNER' THEN 'owner' ELSE 'member' END
FROM users u
WHERE NOT EXISTS (SELECT 1 FROM organization_members m WHERE m.user_id = u.id);

-- Memberships carry the company role: org owners and admins keep that rank, members take their
-- former company role (never OWNER or ADMIN, which were company-wide).
ALTER TABLE organization_members DROP CONSTRAINT organization_members_role_check;
UPDATE organization_members m SET role = CASE
    WHEN m.role = 'owner' THEN 'OWNER'
    WHEN m.role = 'admin' THEN 'ADMIN'
    WHEN u.role IN ('OWNER', 'ADMIN') THEN 'EMPLOYEE'
    ELSE u.role END
FROM users u WHERE u.id = m.user_id;
ALTER TABLE organization_members
    ADD CONSTRAINT organization_members_role_check CHECK (role IN (
        'OWNER', 'ADMIN', 'MANAGER', 'RECRUITER', 'HR', 'SALES', 'EMPLOYEE', 'CONTRACTOR', 'ACCOUNTANT', 'CANDIDATE')),
    ADD COLUMN addition_roles   TEXT[]  NOT NULL DEFAULT '{}' CHECK (addition_roles <@ ARRAY['LEAD']),
    ADD COLUMN active           BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN dismissed_at     TIMESTAMPTZ,
    ADD COLUMN dismissal_reason TEXT    NOT NULL DEFAULT '';
UPDATE organization_members m SET addition_roles = u.addition_roles, active = u.active,
    dismissed_at = u.dismissed_at, dismissal_reason = u.dismissal_reason
FROM users u WHERE u.id = m.user_id;
CREATE INDEX organization_members_role_idx ON organization_members (organization_id, role);

ALTER TABLE organization_invitations DROP CONSTRAINT organization_invitations_role_check;
UPDATE organization_invitations SET role = CASE role WHEN 'owner' THEN 'ADMIN' WHEN 'admin' THEN 'ADMIN' ELSE 'EMPLOYEE' END;
ALTER TABLE organization_invitations ADD CONSTRAINT organization_invitations_role_check CHECK (role IN (
    'ADMIN', 'MANAGER', 'RECRUITER', 'HR', 'SALES', 'EMPLOYEE', 'CONTRACTOR', 'ACCOUNTANT', 'CANDIDATE'));

ALTER TABLE users DROP COLUMN role, DROP COLUMN addition_roles, DROP COLUMN active,
    DROP COLUMN dismissed_at, DROP COLUMN dismissal_reason;

-- Profiles are an organization's record of a person (each employer keeps its own HR data).
ALTER TABLE profiles ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
ALTER TABLE profiles DROP CONSTRAINT profiles_pkey;
INSERT INTO profiles (organization_id, user_id, name_cyrillic, birthday, start_date, trial_until, working_hours, billable, details, finance, updated_at)
SELECT m.organization_id, p.user_id, p.name_cyrillic, p.birthday, p.start_date, p.trial_until, p.working_hours, p.billable, p.details, p.finance, p.updated_at
FROM profiles p JOIN organization_members m ON m.user_id = p.user_id
WHERE p.organization_id IS NULL;
DELETE FROM profiles WHERE organization_id IS NULL;
ALTER TABLE profiles ALTER COLUMN organization_id SET NOT NULL, ADD PRIMARY KEY (organization_id, user_id);
CREATE INDEX profiles_user_idx ON profiles (user_id);

-- Rows about a person go to their oldest organization.
CREATE TEMP TABLE first_org AS
SELECT DISTINCT ON (user_id) user_id, organization_id FROM organization_members ORDER BY user_id, added_at, organization_id;

ALTER TABLE user_documents ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
ALTER TABLE user_comments ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
ALTER TABLE user_feedback ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
ALTER TABLE vacations ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
ALTER TABLE hr_events ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
UPDATE user_documents t SET organization_id = f.organization_id FROM first_org f WHERE f.user_id = t.user_id;
UPDATE user_comments t SET organization_id = f.organization_id FROM first_org f WHERE f.user_id = t.user_id;
UPDATE user_feedback t SET organization_id = f.organization_id FROM first_org f WHERE f.user_id = t.user_id;
UPDATE vacations t SET organization_id = f.organization_id FROM first_org f WHERE f.user_id = t.user_id;
UPDATE hr_events t SET organization_id = f.organization_id FROM first_org f WHERE f.user_id = t.user_id;
DELETE FROM user_documents WHERE organization_id IS NULL;
DELETE FROM user_comments WHERE organization_id IS NULL;
DELETE FROM user_feedback WHERE organization_id IS NULL;
DELETE FROM vacations WHERE organization_id IS NULL;
DELETE FROM hr_events WHERE organization_id IS NULL;
ALTER TABLE user_documents ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE user_comments ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE user_feedback ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE vacations ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE hr_events ALTER COLUMN organization_id SET NOT NULL;
CREATE INDEX user_documents_org_idx ON user_documents (organization_id, user_id);
CREATE INDEX user_comments_org_idx ON user_comments (organization_id, user_id);
CREATE INDEX user_feedback_org_idx ON user_feedback (organization_id, user_id);
CREATE INDEX vacations_org_idx ON vacations (organization_id, start_day);
DROP INDEX hr_events_day_idx;
CREATE INDEX hr_events_org_day_idx ON hr_events (organization_id, day);

-- Candidates: a person can be a candidate in one organization and an employee in another.
ALTER TABLE candidates ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
ALTER TABLE candidates DROP CONSTRAINT candidates_pkey;
INSERT INTO candidates (organization_id, user_id, status, attributes, channel, min_salary, max_salary, min_rate, max_rate,
    hiring_commission, commission_receiver, trello_link, recruiter_id, rejection_reason, updated_at)
SELECT m.organization_id, c.user_id, c.status, c.attributes, c.channel, c.min_salary, c.max_salary, c.min_rate, c.max_rate,
    c.hiring_commission, c.commission_receiver, c.trello_link, c.recruiter_id, c.rejection_reason, c.updated_at
FROM candidates c JOIN organization_members m ON m.user_id = c.user_id AND m.role = 'CANDIDATE'
WHERE c.organization_id IS NULL;
DELETE FROM candidates WHERE organization_id IS NULL;
ALTER TABLE candidates ALTER COLUMN organization_id SET NOT NULL, ADD PRIMARY KEY (organization_id, user_id);

-- Reference lists and public holidays: each organization keeps its own copy.
ALTER TABLE directory_items ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
DROP INDEX directory_items_name_key;
INSERT INTO directory_items (organization_id, kind, name)
SELECT o.id, d.kind, d.name FROM directory_items d CROSS JOIN organizations o WHERE d.organization_id IS NULL;
DELETE FROM directory_items WHERE organization_id IS NULL;
ALTER TABLE directory_items ALTER COLUMN organization_id SET NOT NULL;
CREATE UNIQUE INDEX directory_items_name_key ON directory_items (organization_id, kind, lower(name));

ALTER TABLE holidays ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
DROP INDEX holidays_day_key;
INSERT INTO holidays (organization_id, day, name_latin, name_cyrillic)
SELECT o.id, h.day, h.name_latin, h.name_cyrillic FROM holidays h CROSS JOIN organizations o WHERE h.organization_id IS NULL;
DELETE FROM holidays WHERE organization_id IS NULL;
ALTER TABLE holidays ALTER COLUMN organization_id SET NOT NULL;
CREATE UNIQUE INDEX holidays_day_key ON holidays (organization_id, day, lower(name_latin));

-- Wiki: articles and groups belong to the organization of their author (project articles: the project's).
ALTER TABLE wiki_articles ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
ALTER TABLE wiki_groups ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;
UPDATE wiki_articles a SET organization_id = p.organization_id FROM projects p WHERE p.id = a.project_id;
UPDATE wiki_articles a SET organization_id = f.organization_id FROM first_org f WHERE f.user_id = a.owner_id AND a.organization_id IS NULL;
UPDATE wiki_articles SET organization_id = (SELECT min(id) FROM organizations) WHERE organization_id IS NULL;
UPDATE wiki_groups g SET organization_id = COALESCE(
    (SELECT f.organization_id FROM first_org f WHERE f.user_id = g.owner_id), (SELECT min(id) FROM organizations));
DELETE FROM wiki_articles WHERE organization_id IS NULL;
DELETE FROM wiki_groups WHERE organization_id IS NULL;
ALTER TABLE wiki_articles ALTER COLUMN organization_id SET NOT NULL;
ALTER TABLE wiki_groups ALTER COLUMN organization_id SET NOT NULL;
CREATE INDEX wiki_articles_org_idx ON wiki_articles (organization_id);
CREATE INDEX wiki_groups_org_idx ON wiki_groups (organization_id);
-- Sub-articles stay with their parent's organization.
UPDATE wiki_articles a SET parent_id = NULL FROM wiki_articles p
WHERE p.id = a.parent_id AND p.organization_id <> a.organization_id;

DROP TABLE first_org;

-- +goose Down
SELECT 'organizations as tenants cannot be undone automatically';
