-- 059: Structured schools/colleges + departments (Phase 1 org foundation).
--
-- Until now `courses.school` and `courses.department` were free-text. This adds managed lists so an
-- admin adds schools/colleges and, under each, departments — and courses inherit from them. The
-- existing free-text columns are KEPT (and populated with the chosen names on create) so every
-- current query keeps working; new `school_id`/`department_id` FKs add the structure.

CREATE TABLE IF NOT EXISTS schools (
    school_id  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    name       VARCHAR(200) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS departments (
    department_id UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    school_id     UUID        NOT NULL REFERENCES schools(school_id) ON DELETE CASCADE,
    name          VARCHAR(200) NOT NULL,
    -- ACADEMIC (under a school) or SUPPORT (finance, admissions, ICT, library…). SUPPORT depts may
    -- attach to a synthetic "Support Services" school; kind lets dashboards tell them apart later.
    kind          VARCHAR(20) NOT NULL DEFAULT 'ACADEMIC',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, school_id, name)
);

-- Structured links on courses (nullable; the free-text school/department stay as the display value).
ALTER TABLE courses ADD COLUMN IF NOT EXISTS school_id     UUID REFERENCES schools(school_id)     ON DELETE SET NULL;
ALTER TABLE courses ADD COLUMN IF NOT EXISTS department_id UUID REFERENCES departments(department_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_departments_school ON departments(tenant_id, school_id);

ALTER TABLE schools     ENABLE ROW LEVEL SECURITY;
ALTER TABLE schools     FORCE  ROW LEVEL SECURITY;
ALTER TABLE departments ENABLE ROW LEVEL SECURITY;
ALTER TABLE departments FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON schools
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
CREATE POLICY "tenant_isolation" ON departments
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON schools     TO qaat_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON departments TO qaat_app;
