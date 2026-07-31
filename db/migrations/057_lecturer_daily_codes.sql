-- 057: Daily 4-digit lecturer codes for the multi-coordinator case.
--
-- When ONE lecturer must cover several coordinators' concurrent sessions of the same content
-- (e.g. "Structured Programming" under coordinator A and "Programming Fundamentals" under
-- coordinator B, different course codes, same lecturer, same time), the lecturer can physically
-- START on only one hotspot. The system issues a 4-digit code — UNIQUE per tenant per day — that
-- the OTHER coordinators enter on their hub to mark that lecturer present so their students can
-- attend. The code is valid for that lecturer, that day only, and is delivered offline inside each
-- affected coordinator's daily manifest so the hub can validate it with no internet.
CREATE TABLE IF NOT EXISTS lecturer_daily_codes (
    tenant_id   uuid        NOT NULL,
    lecturer_id text        NOT NULL,   -- subject key: "lec:<staff_id>" (lecturer code) or "unit:<unit_id>" (session code)
    code        text        NOT NULL,
    valid_date  date        NOT NULL DEFAULT CURRENT_DATE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, lecturer_id, valid_date),
    UNIQUE (tenant_id, valid_date, code)   -- unique across the whole tenant for that day
);
