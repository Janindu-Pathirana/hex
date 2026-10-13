-- Times are Unix milliseconds; dates are 'YYYY-MM-DD' in Asia/Colombo.

CREATE TABLE securities (
    code       TEXT PRIMARY KEY,          -- JKH.N0000
    ticker     TEXT NOT NULL,
    series     TEXT NOT NULL,
    name       TEXT NOT NULL,
    stock_id   INTEGER,                   -- companyInfoSummery id, used for history
    isin       TEXT,
    issued     REAL,
    par_value  REAL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX securities_ticker ON securities (ticker);
CREATE INDEX securities_name ON securities (name);

CREATE TABLE quotes (
    code        TEXT PRIMARY KEY,
    traded_at   INTEGER,
    price       REAL NOT NULL,
    change      REAL,
    change_pct  REAL,
    open        REAL,
    high        REAL,
    low         REAL,
    prev_close  REAL,
    volume      REAL,
    trades      REAL,
    turnover    REAL,
    market_cap  REAL,
    fetched_at  INTEGER NOT NULL
);

CREATE TABLE quote_stats (
    code         TEXT PRIMARY KEY,
    high_52w     REAL,
    low_52w      REAL,
    ytd_high     REAL,
    ytd_low      REAL,
    all_high     REAL,
    all_low      REAL,
    volume_52w   REAL,
    beta_aspi    REAL,
    beta_spsl20  REAL,
    beta_period  TEXT,
    foreign_pct  REAL,
    fetched_at   INTEGER NOT NULL
);

CREATE TABLE price_bars (
    code   TEXT NOT NULL,
    date   TEXT NOT NULL,
    open   REAL,
    high   REAL,
    low    REAL,
    close  REAL NOT NULL,
    volume REAL,
    PRIMARY KEY (code, date)
) WITHOUT ROWID;

-- ASPI, SPSL20 and the GICS sector indices (index_id = sector symbol, e.g. EGY).
CREATE TABLE index_bars (
    index_id   TEXT NOT NULL,
    date       TEXT NOT NULL,
    name       TEXT,
    value      REAL NOT NULL,
    change     REAL,
    change_pct REAL,
    turnover   REAL,
    volume     REAL,
    trades     REAL,
    PRIMARY KEY (index_id, date)
) WITHOUT ROWID;

CREATE TABLE market_daily (
    date          TEXT PRIMARY KEY,
    turnover      REAL,
    trades        REAL,
    share_volume  REAL,
    market_cap    REAL,
    per           REAL,
    pbv           REAL,
    dy            REAL,
    asi           REAL,
    spsl20        REAL,
    domestic_buy  REAL,
    domestic_sell REAL,
    foreign_buy   REAL,
    foreign_sell  REAL,
    listed        REAL,
    traded        REAL
);

CREATE TABLE meta (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE profiles (
    code             TEXT PRIMARY KEY,
    name             TEXT,
    sector           TEXT,
    board            TEXT,
    fy_end           TEXT,                -- CSE code: "1" = March, "2" = December (observed)
    auditors         TEXT,
    secretaries      TEXT,
    established      TEXT,
    address          TEXT,
    web              TEXT,
    email            TEXT,
    business_summary TEXT,
    updated_at       INTEGER NOT NULL
);

CREATE TABLE directors (
    code     TEXT NOT NULL,
    position INTEGER NOT NULL,
    name     TEXT NOT NULL,
    role     TEXT,
    PRIMARY KEY (code, position)
);

CREATE TABLE corporate_actions (
    id           INTEGER PRIMARY KEY,     -- CSE announcement id
    code         TEXT,                    -- null when the company name is not matched
    company      TEXT NOT NULL,
    category     TEXT NOT NULL,
    kind         TEXT NOT NULL,           -- dividend | rights | split | bonus | agm | other
    announced    TEXT NOT NULL,
    xd           TEXT,
    record_date  TEXT,
    payment_date TEXT,
    remarks      TEXT,
    raw          TEXT NOT NULL
);
CREATE INDEX corporate_actions_code ON corporate_actions (code, announced);

CREATE TABLE reports (
    id            INTEGER PRIMARY KEY,    -- CSE report id
    code          TEXT NOT NULL,
    kind          TEXT NOT NULL,          -- annual | quarterly | other
    title         TEXT NOT NULL,
    period_date   TEXT,                   -- manualDate as a Colombo date
    uploaded_at   INTEGER,
    authorized_at INTEGER,
    path          TEXT NOT NULL,
    pdf_sha256    TEXT,
    bytes         INTEGER,
    pages         INTEGER,
    text_status   TEXT,                   -- ok | scanned | failed
    text_error    TEXT,
    parsed_at     INTEGER
);
CREATE INDEX reports_code ON reports (code, kind, period_date);

CREATE TABLE report_pages (
    report_id INTEGER NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    page      INTEGER NOT NULL,
    kind      TEXT NOT NULL DEFAULT 'other', -- pl | bs | cf | other
    score     REAL NOT NULL DEFAULT 0,
    text_gz   BLOB NOT NULL,              -- gzipped JSON of the page lines
    PRIMARY KEY (report_id, page)
) WITHOUT ROWID;

CREATE TABLE extraction_runs (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id         INTEGER NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
    method            TEXT NOT NULL,      -- rules | llm
    extractor_version TEXT NOT NULL,
    model             TEXT,
    started_at        INTEGER NOT NULL,
    confidence        REAL,
    status            TEXT NOT NULL,      -- ok | low_confidence | failed
    errors            TEXT
);

CREATE TABLE fundamentals (
    code        TEXT NOT NULL,
    period_end  TEXT NOT NULL,
    period_type TEXT NOT NULL,            -- 3M | 6M | 9M | 12M
    scope       TEXT NOT NULL,            -- group | company
    field       TEXT NOT NULL,
    source      TEXT NOT NULL,            -- pdf | llm | manual
    value       REAL NOT NULL,            -- in rupees (or per-share rupees)
    report_id   INTEGER,
    run_id      INTEGER,
    page        INTEGER,
    raw_label   TEXT,
    confidence  REAL NOT NULL,
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (code, period_end, period_type, scope, field, source)
) WITHOUT ROWID;

CREATE TABLE raw_responses (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    source     TEXT NOT NULL,
    endpoint   TEXT NOT NULL,
    key        TEXT NOT NULL,
    fetched_at INTEGER NOT NULL,
    body_gz    BLOB NOT NULL
);
CREATE INDEX raw_responses_key ON raw_responses (source, endpoint, key, fetched_at);

CREATE TABLE sync_jobs (
    key         TEXT PRIMARY KEY,         -- kind:target, e.g. history:JKH.N0000
    kind        TEXT NOT NULL,
    status      TEXT NOT NULL,            -- pending | running | done | failed
    attempts    INTEGER NOT NULL DEFAULT 0,
    last_error  TEXT,
    last_run_at INTEGER,
    last_ok_at  INTEGER,
    next_due_at INTEGER
);
CREATE INDEX sync_jobs_kind ON sync_jobs (kind);
