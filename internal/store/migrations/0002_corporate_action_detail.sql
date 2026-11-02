-- Corporate actions now come from the announcement search (/api/smd), which
-- carries the dates and proportions the old list left empty. The old rows were
-- keyed by a different id and had no dates, so they are dropped and the
-- actions job is made due again.
DELETE FROM corporate_actions;
DELETE FROM sync_jobs WHERE key = 'actions';

ALTER TABLE corporate_actions ADD COLUMN ticker TEXT;
ALTER TABLE corporate_actions ADD COLUMN sec_id INTEGER;
ALTER TABLE corporate_actions ADD COLUMN ex_date TEXT;               -- the date prices adjust: xd, xr, xc or trading commencement
ALTER TABLE corporate_actions ADD COLUMN xr TEXT;
ALTER TABLE corporate_actions ADD COLUMN xc TEXT;
ALTER TABLE corporate_actions ADD COLUMN trading_commencement TEXT;
ALTER TABLE corporate_actions ADD COLUMN trading_suspended TEXT;
ALTER TABLE corporate_actions ADD COLUMN allotment_date TEXT;
ALTER TABLE corporate_actions ADD COLUMN entitlement_date TEXT;      -- sub-division: holdings as at
ALTER TABLE corporate_actions ADD COLUMN agm_date TEXT;
ALTER TABLE corporate_actions ADD COLUMN proportion TEXT;            -- the text as published
ALTER TABLE corporate_actions ADD COLUMN ratio_num REAL;             -- parsed: new shares (split: shares after)
ALTER TABLE corporate_actions ADD COLUMN ratio_den REAL;             -- parsed: existing shares (split: shares before)
ALTER TABLE corporate_actions ADD COLUMN price REAL;                 -- rights issue price or scrip reference price
ALTER TABLE corporate_actions ADD COLUMN dps REAL;                   -- cash dividend per voting share
ALTER TABLE corporate_actions ADD COLUMN child_id INTEGER;           -- follow-up announcement

CREATE INDEX corporate_actions_ex ON corporate_actions (code, ex_date);
