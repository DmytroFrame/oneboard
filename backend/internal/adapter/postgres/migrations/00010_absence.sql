-- +goose Up
-- Public holidays and absence requests (OmiTeam vacations).
CREATE TABLE holidays (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    day           DATE NOT NULL,
    name_latin    TEXT NOT NULL,
    name_cyrillic TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX holidays_day_key ON holidays (day, lower(name_latin));

CREATE TABLE vacations (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type       TEXT   NOT NULL CHECK (type IN ('vacation', 'sick_day', 'day_off')),
    start_day  DATE   NOT NULL,
    end_day    DATE   NOT NULL,
    days       INT    NOT NULL, -- working days in the period, counted when it was saved
    comment    TEXT   NOT NULL DEFAULT '',
    status     TEXT   NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'declined')),
    decided_by BIGINT REFERENCES users (id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_day >= start_day)
);
CREATE INDEX vacations_user_idx ON vacations (user_id, start_day);
CREATE INDEX vacations_period_idx ON vacations (start_day, end_day);

CREATE TABLE vacation_reviewers (
    vacation_id BIGINT NOT NULL REFERENCES vacations (id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (vacation_id, user_id)
);
CREATE INDEX vacation_reviewers_user_idx ON vacation_reviewers (user_id);

-- +goose Down
DROP TABLE vacation_reviewers, vacations, holidays;
