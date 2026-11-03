-- +goose Up
-- Candidates (OmiTeam recruiting): the hiring funnel of accounts with the CANDIDATE role. A missing row
-- means a new application with nothing filled in.
CREATE TABLE candidates (
    user_id             BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    status              TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'hr_interview', 'english_check',
                            'client_interview', 'onboarding', 'in_team', 'blacklist', 'ping_again', 'contractor')),
    attributes          TEXT[] NOT NULL DEFAULT '{}',
    channel             TEXT NOT NULL DEFAULT '',
    min_salary          NUMERIC(12, 2),
    max_salary          NUMERIC(12, 2),
    min_rate            NUMERIC(12, 2),
    max_rate            NUMERIC(12, 2),
    hiring_commission   NUMERIC(12, 2),
    commission_receiver TEXT NOT NULL DEFAULT '',
    trello_link         TEXT NOT NULL DEFAULT '',
    recruiter_id        BIGINT REFERENCES users (id) ON DELETE SET NULL,
    rejection_reason    TEXT NOT NULL DEFAULT '',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX candidates_recruiter_idx ON candidates (recruiter_id);

-- Hires, dismissals and rejected candidates, for the HR statistics. Rows can be removed from the
-- statistics without touching the person.
CREATE TABLE hr_events (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id        BIGINT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind           TEXT    NOT NULL CHECK (kind IN ('hired', 'dismissed', 'rejected')),
    day            DATE    NOT NULL DEFAULT CURRENT_DATE,
    from_candidate BOOLEAN NOT NULL DEFAULT FALSE,
    reason         TEXT    NOT NULL DEFAULT '',
    actor_id       BIGINT  REFERENCES users (id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX hr_events_day_idx ON hr_events (day);

INSERT INTO hr_events (user_id, kind, day, reason)
SELECT id, 'dismissed', dismissed_at::date, dismissal_reason FROM users WHERE dismissed_at IS NOT NULL;

-- +goose Down
DROP TABLE hr_events, candidates;
