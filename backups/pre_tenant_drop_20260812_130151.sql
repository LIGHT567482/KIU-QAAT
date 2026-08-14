--
-- PostgreSQL database dump
--

\restrict hAxV8ecjA3LpYF7W35XEL3Co3ztrguG5fwPwHHYjUvAvggieUeNtuE9Mcstf8Ap

-- Dumped from database version 15.18
-- Dumped by pg_dump version 15.18

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

DROP POLICY IF EXISTS tenant_isolation ON public.venues;
DROP POLICY IF EXISTS tenant_isolation ON public.users;
DROP POLICY IF EXISTS tenant_isolation ON public.user_schools;
DROP POLICY IF EXISTS tenant_isolation ON public.timetable_slots;
DROP POLICY IF EXISTS tenant_isolation ON public.sync_uploads;
DROP POLICY IF EXISTS tenant_isolation ON public.students_extended;
DROP POLICY IF EXISTS tenant_isolation ON public.student_device_bindings;
DROP POLICY IF EXISTS tenant_isolation ON public.student_attendance_summary;
DROP POLICY IF EXISTS tenant_isolation ON public.sessions;
DROP POLICY IF EXISTS tenant_isolation ON public.semester_archives;
DROP POLICY IF EXISTS tenant_isolation ON public.schools;
DROP POLICY IF EXISTS tenant_isolation ON public.qa_rep_submissions;
DROP POLICY IF EXISTS tenant_isolation ON public.qa_messages;
DROP POLICY IF EXISTS tenant_isolation ON public.qa_message_reads;
DROP POLICY IF EXISTS tenant_isolation ON public.patroller_pins;
DROP POLICY IF EXISTS tenant_isolation ON public.patroller_device_bindings;
DROP POLICY IF EXISTS tenant_isolation ON public.offering_unit_schedules;
DROP POLICY IF EXISTS tenant_isolation ON public.notification_recipients;
DROP POLICY IF EXISTS tenant_isolation ON public.notification_log;
DROP POLICY IF EXISTS tenant_isolation ON public.monitor_log_units;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturers;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturer_webauthn_credentials;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturer_presence_claims;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturer_patrol_logs;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturer_daily_codes;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturer_biometric_templates;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturer_attendance_logs;
DROP POLICY IF EXISTS tenant_isolation ON public.lecturer_assignments;
DROP POLICY IF EXISTS tenant_isolation ON public.hardware_vault;
DROP POLICY IF EXISTS tenant_isolation ON public.employees;
DROP POLICY IF EXISTS tenant_isolation ON public.employee_attendance_logs;
DROP POLICY IF EXISTS tenant_isolation ON public.employee_attendance_days;
DROP POLICY IF EXISTS tenant_isolation ON public.departments;
DROP POLICY IF EXISTS tenant_isolation ON public.courses;
DROP POLICY IF EXISTS tenant_isolation ON public.course_units;
DROP POLICY IF EXISTS tenant_isolation ON public.course_offerings;
DROP POLICY IF EXISTS tenant_isolation ON public.coordinator_delegations;
DROP POLICY IF EXISTS tenant_isolation ON public.attendance_logs;
DROP POLICY IF EXISTS tenant_isolation ON public.app_notifications;
DROP POLICY IF EXISTS tenant_isolation ON public.admin_audit_log;
DROP POLICY IF EXISTS no_update_audit ON public.admin_audit_log;
DROP POLICY IF EXISTS no_update_attendance ON public.attendance_logs;
DROP POLICY IF EXISTS no_delete_audit ON public.admin_audit_log;
DROP POLICY IF EXISTS no_delete_attendance ON public.attendance_logs;
ALTER TABLE IF EXISTS ONLY public.venues DROP CONSTRAINT IF EXISTS venues_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.venues DROP CONSTRAINT IF EXISTS venues_school_id_fkey;
ALTER TABLE IF EXISTS ONLY public.venues DROP CONSTRAINT IF EXISTS venues_department_id_fkey;
ALTER TABLE IF EXISTS ONLY public.users DROP CONSTRAINT IF EXISTS users_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.user_schools DROP CONSTRAINT IF EXISTS user_schools_user_id_fkey;
ALTER TABLE IF EXISTS ONLY public.user_schools DROP CONSTRAINT IF EXISTS user_schools_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.user_schools DROP CONSTRAINT IF EXISTS user_schools_school_id_fkey;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_venue_id_fkey;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_unit_id_fkey;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_offering_id_fkey;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_lecturer_id_fkey;
ALTER TABLE IF EXISTS ONLY public.sync_uploads DROP CONSTRAINT IF EXISTS sync_uploads_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.students_extended DROP CONSTRAINT IF EXISTS students_extended_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.students_extended DROP CONSTRAINT IF EXISTS students_extended_offering_id_fkey;
ALTER TABLE IF EXISTS ONLY public.students_extended DROP CONSTRAINT IF EXISTS students_extended_course_id_fkey;
ALTER TABLE IF EXISTS ONLY public.student_device_bindings DROP CONSTRAINT IF EXISTS student_device_bindings_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.student_device_bindings DROP CONSTRAINT IF EXISTS student_device_bindings_student_id_fkey;
ALTER TABLE IF EXISTS ONLY public.sessions DROP CONSTRAINT IF EXISTS sessions_venue_id_fkey;
ALTER TABLE IF EXISTS ONLY public.sessions DROP CONSTRAINT IF EXISTS sessions_unit_id_fkey;
ALTER TABLE IF EXISTS ONLY public.sessions DROP CONSTRAINT IF EXISTS sessions_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.sessions DROP CONSTRAINT IF EXISTS sessions_offering_id_fkey;
ALTER TABLE IF EXISTS ONLY public.semester_archives DROP CONSTRAINT IF EXISTS semester_archives_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.schools DROP CONSTRAINT IF EXISTS schools_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.qa_rep_submissions DROP CONSTRAINT IF EXISTS qa_rep_submissions_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.qa_rep_submissions DROP CONSTRAINT IF EXISTS qa_rep_submissions_submitted_by_fkey;
ALTER TABLE IF EXISTS ONLY public.qa_messages DROP CONSTRAINT IF EXISTS qa_messages_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.qa_message_reads DROP CONSTRAINT IF EXISTS qa_message_reads_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.qa_message_reads DROP CONSTRAINT IF EXISTS qa_message_reads_message_id_fkey;
ALTER TABLE IF EXISTS ONLY public.patroller_pins DROP CONSTRAINT IF EXISTS patroller_pins_user_id_fkey;
ALTER TABLE IF EXISTS ONLY public.patroller_pins DROP CONSTRAINT IF EXISTS patroller_pins_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.patroller_device_bindings DROP CONSTRAINT IF EXISTS patroller_device_bindings_user_id_fkey;
ALTER TABLE IF EXISTS ONLY public.patroller_device_bindings DROP CONSTRAINT IF EXISTS patroller_device_bindings_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.offering_unit_schedules DROP CONSTRAINT IF EXISTS offering_unit_schedules_unit_id_fkey;
ALTER TABLE IF EXISTS ONLY public.offering_unit_schedules DROP CONSTRAINT IF EXISTS offering_unit_schedules_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.offering_unit_schedules DROP CONSTRAINT IF EXISTS offering_unit_schedules_offering_id_fkey;
ALTER TABLE IF EXISTS ONLY public.notification_recipients DROP CONSTRAINT IF EXISTS notification_recipients_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.notification_recipients DROP CONSTRAINT IF EXISTS notification_recipients_notification_id_fkey;
ALTER TABLE IF EXISTS ONLY public.notification_log DROP CONSTRAINT IF EXISTS notification_log_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.notification_log DROP CONSTRAINT IF EXISTS notification_log_recipient_user_id_fkey;
ALTER TABLE IF EXISTS ONLY public.monitor_log_units DROP CONSTRAINT IF EXISTS monitor_log_units_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.monitor_log_units DROP CONSTRAINT IF EXISTS monitor_log_units_patrol_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturers DROP CONSTRAINT IF EXISTS lecturers_user_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturers DROP CONSTRAINT IF EXISTS lecturers_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturers DROP CONSTRAINT IF EXISTS lecturers_school_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_webauthn_credentials DROP CONSTRAINT IF EXISTS lecturer_webauthn_credentials_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_webauthn_credentials DROP CONSTRAINT IF EXISTS lecturer_webauthn_credentials_lecturer_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_presence_claims DROP CONSTRAINT IF EXISTS lecturer_presence_claims_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_patrol_logs DROP CONSTRAINT IF EXISTS lecturer_patrol_logs_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_patrol_logs DROP CONSTRAINT IF EXISTS lecturer_patrol_logs_submission_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_biometric_templates DROP CONSTRAINT IF EXISTS lecturer_biometric_templates_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_biometric_templates DROP CONSTRAINT IF EXISTS lecturer_biometric_templates_lecturer_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_attendance_logs DROP CONSTRAINT IF EXISTS lecturer_attendance_logs_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_attendance_logs DROP CONSTRAINT IF EXISTS lecturer_attendance_logs_session_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_assignments DROP CONSTRAINT IF EXISTS lecturer_assignments_unit_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_assignments DROP CONSTRAINT IF EXISTS lecturer_assignments_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_assignments DROP CONSTRAINT IF EXISTS lecturer_assignments_lecturer_id_fkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_assignments DROP CONSTRAINT IF EXISTS lecturer_assignments_course_id_fkey;
ALTER TABLE IF EXISTS ONLY public.hardware_vault DROP CONSTRAINT IF EXISTS hardware_vault_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.employees DROP CONSTRAINT IF EXISTS employees_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.employee_attendance_logs DROP CONSTRAINT IF EXISTS employee_attendance_logs_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.employee_attendance_days DROP CONSTRAINT IF EXISTS employee_attendance_days_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.departments DROP CONSTRAINT IF EXISTS departments_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.departments DROP CONSTRAINT IF EXISTS departments_school_id_fkey;
ALTER TABLE IF EXISTS ONLY public.courses DROP CONSTRAINT IF EXISTS courses_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.courses DROP CONSTRAINT IF EXISTS courses_school_id_fkey;
ALTER TABLE IF EXISTS ONLY public.courses DROP CONSTRAINT IF EXISTS courses_department_id_fkey;
ALTER TABLE IF EXISTS ONLY public.course_units DROP CONSTRAINT IF EXISTS course_units_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.course_units DROP CONSTRAINT IF EXISTS course_units_default_venue_id_fkey;
ALTER TABLE IF EXISTS ONLY public.course_units DROP CONSTRAINT IF EXISTS course_units_course_id_fkey;
ALTER TABLE IF EXISTS ONLY public.course_offerings DROP CONSTRAINT IF EXISTS course_offerings_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.course_offerings DROP CONSTRAINT IF EXISTS course_offerings_course_id_fkey;
ALTER TABLE IF EXISTS ONLY public.coordinator_delegations DROP CONSTRAINT IF EXISTS coordinator_delegations_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.coordinator_delegations DROP CONSTRAINT IF EXISTS coordinator_delegations_offering_id_fkey;
ALTER TABLE IF EXISTS ONLY public.attendance_logs DROP CONSTRAINT IF EXISTS attendance_logs_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.attendance_logs DROP CONSTRAINT IF EXISTS attendance_logs_session_id_fkey;
ALTER TABLE IF EXISTS ONLY public.app_notifications DROP CONSTRAINT IF EXISTS app_notifications_tenant_id_fkey;
ALTER TABLE IF EXISTS ONLY public.admin_audit_log DROP CONSTRAINT IF EXISTS admin_audit_log_tenant_id_fkey;
DROP TRIGGER IF EXISTS trg_offering_delivery_mode ON public.course_offerings;
DROP INDEX IF EXISTS public.ux_users_tenant_coordinator_code;
DROP INDEX IF EXISTS public.ux_schools_abbreviation;
DROP INDEX IF EXISTS public.ux_patrol_logs_slot;
DROP INDEX IF EXISTS public.ux_offerings_tenant_coordinator;
DROP INDEX IF EXISTS public.ux_lecturers_tenant_staffid;
DROP INDEX IF EXISTS public.ux_departments_standalone_name;
DROP INDEX IF EXISTS public.uq_tenants_institution_id;
DROP INDEX IF EXISTS public.uq_device_one_student;
DROP INDEX IF EXISTS public.uq_attendance_vector_clock;
DROP INDEX IF EXISTS public.uq_attendance_session_student_auth;
DROP INDEX IF EXISTS public.uq_attendance_session_student;
DROP INDEX IF EXISTS public.student_attendance_summary_tenant_id_unit_id_idx;
DROP INDEX IF EXISTS public.student_attendance_summary_student_id_unit_id_tenant_id_idx;
DROP INDEX IF EXISTS public.ix_webauthn_lecturer;
DROP INDEX IF EXISTS public.ix_users_tlc_department;
DROP INDEX IF EXISTS public.ix_timetable_slots_room_day;
DROP INDEX IF EXISTS public.ix_timetable_slots_offering;
DROP INDEX IF EXISTS public.ix_sessions_venue_date;
DROP INDEX IF EXISTS public.ix_sessions_unscheduled;
DROP INDEX IF EXISTS public.ix_sessions_provision;
DROP INDEX IF EXISTS public.ix_sessions_online;
DROP INDEX IF EXISTS public.ix_semester_archives_tenant;
DROP INDEX IF EXISTS public.ix_patrol_logs_manual;
DROP INDEX IF EXISTS public.ix_patrol_logs_compensation;
DROP INDEX IF EXISTS public.ix_monitor_log_units_unit;
DROP INDEX IF EXISTS public.ix_monitor_log_units_log;
DROP INDEX IF EXISTS public.ix_lecturer_assignments_unit;
DROP INDEX IF EXISTS public.ix_employees_tenant;
DROP INDEX IF EXISTS public.ix_emp_att_tenant_staff;
DROP INDEX IF EXISTS public.ix_coord_deleg_coordinator;
DROP INDEX IF EXISTS public.ix_coord_deleg_code;
DROP INDEX IF EXISTS public.ix_bio_tmpl_lecturer;
DROP INDEX IF EXISTS public.ix_app_notifications_action;
DROP INDEX IF EXISTS public.idx_venues_tenant;
DROP INDEX IF EXISTS public.idx_users_tenant_role;
DROP INDEX IF EXISTS public.idx_users_tenant_email;
DROP INDEX IF EXISTS public.idx_users_dept_school;
DROP INDEX IF EXISTS public.idx_user_schools_tenant_user;
DROP INDEX IF EXISTS public.idx_units_course;
DROP INDEX IF EXISTS public.idx_timetable_slots_venue;
DROP INDEX IF EXISTS public.idx_timetable_slots_unit;
DROP INDEX IF EXISTS public.idx_timetable_slots_day_lecturer;
DROP INDEX IF EXISTS public.idx_sync_uploads_coord;
DROP INDEX IF EXISTS public.idx_summary_tenant_unit;
DROP INDEX IF EXISTS public.idx_students_tenant;
DROP INDEX IF EXISTS public.idx_sessions_unit;
DROP INDEX IF EXISTS public.idx_sessions_status;
DROP INDEX IF EXISTS public.idx_sessions_coordinator;
DROP INDEX IF EXISTS public.idx_sdb_tenant;
DROP INDEX IF EXISTS public.idx_qa_rep_submissions_scope;
DROP INDEX IF EXISTS public.idx_qa_rep_submissions_by;
DROP INDEX IF EXISTS public.idx_qa_messages_tenant_created;
DROP INDEX IF EXISTS public.idx_qa_messages_audience;
DROP INDEX IF EXISTS public.idx_presence_claims_tenant_time;
DROP INDEX IF EXISTS public.idx_presence_claims_lecturer;
DROP INDEX IF EXISTS public.idx_patroller_pins_tenant;
DROP INDEX IF EXISTS public.idx_patrol_logs_submission;
DROP INDEX IF EXISTS public.idx_patrol_logs_lecturer;
DROP INDEX IF EXISTS public.idx_patrol_logs_date;
DROP INDEX IF EXISTS public.idx_patrol_logs_changed;
DROP INDEX IF EXISTS public.idx_patrol_device_fingerprint;
DROP INDEX IF EXISTS public.idx_patrol_bindings_tenant;
DROP INDEX IF EXISTS public.idx_notification_log_tenant_date;
DROP INDEX IF EXISTS public.idx_notif_recipients_visible;
DROP INDEX IF EXISTS public.idx_notif_recipient;
DROP INDEX IF EXISTS public.idx_lecturers_staffid_lower;
DROP INDEX IF EXISTS public.idx_lecturers_school;
DROP INDEX IF EXISTS public.idx_lecturer_assignments_unit;
DROP INDEX IF EXISTS public.idx_lal_unit;
DROP INDEX IF EXISTS public.idx_lal_session;
DROP INDEX IF EXISTS public.idx_lal_lecturer;
DROP INDEX IF EXISTS public.idx_emp_days_tenant_date;
DROP INDEX IF EXISTS public.idx_emp_days_name;
DROP INDEX IF EXISTS public.idx_emp_days_flags;
DROP INDEX IF EXISTS public.idx_emp_days_department;
DROP INDEX IF EXISTS public.idx_departments_school;
DROP INDEX IF EXISTS public.idx_courses_tenant;
DROP INDEX IF EXISTS public.idx_course_units_roadmap;
DROP INDEX IF EXISTS public.idx_course_units_id_lower;
DROP INDEX IF EXISTS public.idx_audit_tenant_actor;
DROP INDEX IF EXISTS public.idx_attendance_student_unit;
DROP INDEX IF EXISTS public.idx_attendance_student;
DROP INDEX IF EXISTS public.idx_attendance_session;
DROP INDEX IF EXISTS public.idx_app_notifications_tenant_created;
ALTER TABLE IF EXISTS ONLY public.venues DROP CONSTRAINT IF EXISTS venues_pkey;
ALTER TABLE IF EXISTS ONLY public.course_offerings DROP CONSTRAINT IF EXISTS ux_offerings_cohort;
ALTER TABLE IF EXISTS ONLY public.users DROP CONSTRAINT IF EXISTS users_tenant_id_email_key;
ALTER TABLE IF EXISTS ONLY public.users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE IF EXISTS ONLY public.user_schools DROP CONSTRAINT IF EXISTS user_schools_pkey;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_pkey;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_offering_id_unit_id_day_of_week_start_time_key;
ALTER TABLE IF EXISTS ONLY public.timetable_slots DROP CONSTRAINT IF EXISTS timetable_slots_no_room_double_booking;
ALTER TABLE IF EXISTS ONLY public.tenants DROP CONSTRAINT IF EXISTS tenants_pkey;
ALTER TABLE IF EXISTS ONLY public.tenants DROP CONSTRAINT IF EXISTS tenants_domain_key;
ALTER TABLE IF EXISTS ONLY public.sync_uploads DROP CONSTRAINT IF EXISTS sync_uploads_pkey;
ALTER TABLE IF EXISTS ONLY public.students_extended DROP CONSTRAINT IF EXISTS students_extended_pkey;
ALTER TABLE IF EXISTS ONLY public.student_device_bindings DROP CONSTRAINT IF EXISTS student_device_bindings_pkey;
ALTER TABLE IF EXISTS ONLY public.student_attendance_summary DROP CONSTRAINT IF EXISTS student_attendance_summary_pkey;
ALTER TABLE IF EXISTS ONLY public.sessions DROP CONSTRAINT IF EXISTS sessions_pkey;
ALTER TABLE IF EXISTS ONLY public.semester_archives DROP CONSTRAINT IF EXISTS semester_archives_pkey;
ALTER TABLE IF EXISTS ONLY public.schools DROP CONSTRAINT IF EXISTS schools_tenant_id_name_key;
ALTER TABLE IF EXISTS ONLY public.schools DROP CONSTRAINT IF EXISTS schools_pkey;
ALTER TABLE IF EXISTS ONLY public.schema_migrations DROP CONSTRAINT IF EXISTS schema_migrations_pkey;
ALTER TABLE IF EXISTS ONLY public.scheduled_job_runs DROP CONSTRAINT IF EXISTS scheduled_job_runs_pkey;
ALTER TABLE IF EXISTS ONLY public.qa_rep_submissions DROP CONSTRAINT IF EXISTS qa_rep_submissions_pkey;
ALTER TABLE IF EXISTS ONLY public.qa_messages DROP CONSTRAINT IF EXISTS qa_messages_pkey;
ALTER TABLE IF EXISTS ONLY public.qa_message_reads DROP CONSTRAINT IF EXISTS qa_message_reads_pkey;
ALTER TABLE IF EXISTS ONLY public.patroller_pins DROP CONSTRAINT IF EXISTS patroller_pins_pkey;
ALTER TABLE IF EXISTS ONLY public.patroller_device_bindings DROP CONSTRAINT IF EXISTS patroller_device_bindings_pkey;
ALTER TABLE IF EXISTS ONLY public.offering_unit_schedules DROP CONSTRAINT IF EXISTS offering_unit_schedules_pkey;
ALTER TABLE IF EXISTS ONLY public.notification_recipients DROP CONSTRAINT IF EXISTS notification_recipients_pkey;
ALTER TABLE IF EXISTS ONLY public.notification_log DROP CONSTRAINT IF EXISTS notification_log_tenant_id_kind_subject_key_subject_date_key;
ALTER TABLE IF EXISTS ONLY public.notification_log DROP CONSTRAINT IF EXISTS notification_log_pkey;
ALTER TABLE IF EXISTS ONLY public.monitor_log_units DROP CONSTRAINT IF EXISTS monitor_log_units_pkey;
ALTER TABLE IF EXISTS ONLY public.monitor_log_units DROP CONSTRAINT IF EXISTS monitor_log_units_patrol_id_unit_id_key;
ALTER TABLE IF EXISTS ONLY public.lecturers DROP CONSTRAINT IF EXISTS lecturers_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_webauthn_credentials DROP CONSTRAINT IF EXISTS lecturer_webauthn_credentials_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_presence_claims DROP CONSTRAINT IF EXISTS lecturer_presence_claims_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_patrol_logs DROP CONSTRAINT IF EXISTS lecturer_patrol_logs_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_daily_codes DROP CONSTRAINT IF EXISTS lecturer_daily_codes_tenant_id_valid_date_code_key;
ALTER TABLE IF EXISTS ONLY public.lecturer_daily_codes DROP CONSTRAINT IF EXISTS lecturer_daily_codes_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_biometric_templates DROP CONSTRAINT IF EXISTS lecturer_biometric_templates_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_attendance_logs DROP CONSTRAINT IF EXISTS lecturer_attendance_logs_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_assignments DROP CONSTRAINT IF EXISTS lecturer_assignments_pkey;
ALTER TABLE IF EXISTS ONLY public.lecturer_assignments DROP CONSTRAINT IF EXISTS lecturer_assignments_lecturer_id_unit_id_academic_year_inta_key;
ALTER TABLE IF EXISTS ONLY public.hardware_vault DROP CONSTRAINT IF EXISTS hardware_vault_pkey;
ALTER TABLE IF EXISTS ONLY public.employees DROP CONSTRAINT IF EXISTS employees_tenant_id_staff_id_key;
ALTER TABLE IF EXISTS ONLY public.employees DROP CONSTRAINT IF EXISTS employees_pkey;
ALTER TABLE IF EXISTS ONLY public.employee_attendance_logs DROP CONSTRAINT IF EXISTS employee_attendance_logs_tenant_id_staff_id_event_time_even_key;
ALTER TABLE IF EXISTS ONLY public.employee_attendance_logs DROP CONSTRAINT IF EXISTS employee_attendance_logs_pkey;
ALTER TABLE IF EXISTS ONLY public.employee_attendance_days DROP CONSTRAINT IF EXISTS employee_attendance_days_tenant_id_ac_no_work_date_key;
ALTER TABLE IF EXISTS ONLY public.employee_attendance_days DROP CONSTRAINT IF EXISTS employee_attendance_days_pkey;
ALTER TABLE IF EXISTS ONLY public.departments DROP CONSTRAINT IF EXISTS departments_tenant_id_school_id_name_key;
ALTER TABLE IF EXISTS ONLY public.departments DROP CONSTRAINT IF EXISTS departments_pkey;
ALTER TABLE IF EXISTS ONLY public.courses DROP CONSTRAINT IF EXISTS courses_pkey;
ALTER TABLE IF EXISTS ONLY public.course_units DROP CONSTRAINT IF EXISTS course_units_pkey;
ALTER TABLE IF EXISTS ONLY public.course_offerings DROP CONSTRAINT IF EXISTS course_offerings_pkey;
ALTER TABLE IF EXISTS ONLY public.coordinator_delegations DROP CONSTRAINT IF EXISTS coordinator_delegations_tenant_id_code_key;
ALTER TABLE IF EXISTS ONLY public.coordinator_delegations DROP CONSTRAINT IF EXISTS coordinator_delegations_pkey;
ALTER TABLE IF EXISTS ONLY public.attendance_logs DROP CONSTRAINT IF EXISTS attendance_logs_pkey;
ALTER TABLE IF EXISTS ONLY public.app_notifications DROP CONSTRAINT IF EXISTS app_notifications_pkey;
ALTER TABLE IF EXISTS ONLY public.admin_audit_log DROP CONSTRAINT IF EXISTS admin_audit_log_pkey;
DROP TABLE IF EXISTS public.venues;
DROP TABLE IF EXISTS public.users;
DROP TABLE IF EXISTS public.user_schools;
DROP TABLE IF EXISTS public.timetable_slots;
DROP TABLE IF EXISTS public.tenants;
DROP TABLE IF EXISTS public.sync_uploads;
DROP TABLE IF EXISTS public.students_extended;
DROP TABLE IF EXISTS public.student_device_bindings;
DROP TABLE IF EXISTS public.student_attendance_summary;
DROP TABLE IF EXISTS public.sessions;
DROP TABLE IF EXISTS public.semester_archives;
DROP TABLE IF EXISTS public.schools;
DROP TABLE IF EXISTS public.schema_migrations;
DROP TABLE IF EXISTS public.scheduled_job_runs;
DROP TABLE IF EXISTS public.qa_rep_submissions;
DROP TABLE IF EXISTS public.qa_messages;
DROP TABLE IF EXISTS public.qa_message_reads;
DROP TABLE IF EXISTS public.patroller_pins;
DROP TABLE IF EXISTS public.patroller_device_bindings;
DROP TABLE IF EXISTS public.offering_unit_schedules;
DROP TABLE IF EXISTS public.notification_recipients;
DROP TABLE IF EXISTS public.notification_log;
DROP TABLE IF EXISTS public.monitor_log_units;
DROP TABLE IF EXISTS public.lecturers;
DROP TABLE IF EXISTS public.lecturer_webauthn_credentials;
DROP TABLE IF EXISTS public.lecturer_presence_claims;
DROP TABLE IF EXISTS public.lecturer_patrol_logs;
DROP TABLE IF EXISTS public.lecturer_daily_codes;
DROP TABLE IF EXISTS public.lecturer_biometric_templates;
DROP TABLE IF EXISTS public.lecturer_attendance_logs;
DROP TABLE IF EXISTS public.lecturer_assignments;
DROP TABLE IF EXISTS public.hardware_vault;
DROP TABLE IF EXISTS public.employees;
DROP TABLE IF EXISTS public.employee_attendance_logs;
DROP TABLE IF EXISTS public.employee_attendance_days;
DROP TABLE IF EXISTS public.departments;
DROP TABLE IF EXISTS public.courses;
DROP TABLE IF EXISTS public.course_units;
DROP TABLE IF EXISTS public.course_offerings;
DROP TABLE IF EXISTS public.coordinator_delegations;
DROP TABLE IF EXISTS public.attendance_logs;
DROP TABLE IF EXISTS public.app_notifications;
DROP TABLE IF EXISTS public.admin_audit_log;
DROP FUNCTION IF EXISTS public.refresh_attendance_summary(p_tenant uuid);
DROP FUNCTION IF EXISTS public.offering_delivery_mode_default();
DROP TYPE IF EXISTS public.user_role_enum;
DROP TYPE IF EXISTS public.sync_status_enum;
DROP TYPE IF EXISTS public.session_status_enum;
DROP TYPE IF EXISTS public.intake_session_enum;
DROP TYPE IF EXISTS public.entry_method_enum;
DROP TYPE IF EXISTS public.enrollment_status_enum;
DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS btree_gist;
--
-- Name: btree_gist; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS btree_gist WITH SCHEMA public;


--
-- Name: EXTENSION btree_gist; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION btree_gist IS 'support for indexing common datatypes in GiST';


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: enrollment_status_enum; Type: TYPE; Schema: public; Owner: qaat
--

CREATE TYPE public.enrollment_status_enum AS ENUM (
    'ACTIVE',
    'SUSPENDED',
    'GRADUATED',
    'WITHDRAWN'
);


ALTER TYPE public.enrollment_status_enum OWNER TO qaat;

--
-- Name: entry_method_enum; Type: TYPE; Schema: public; Owner: qaat
--

CREATE TYPE public.entry_method_enum AS ENUM (
    'QR_SCAN',
    'MANUAL_OVERRIDE',
    'AUTHENTICATED'
);


ALTER TYPE public.entry_method_enum OWNER TO qaat;

--
-- Name: intake_session_enum; Type: TYPE; Schema: public; Owner: qaat
--

CREATE TYPE public.intake_session_enum AS ENUM (
    'Morning',
    'Evening',
    'Weekend',
    'Distance'
);


ALTER TYPE public.intake_session_enum OWNER TO qaat;

--
-- Name: session_status_enum; Type: TYPE; Schema: public; Owner: qaat
--

CREATE TYPE public.session_status_enum AS ENUM (
    'PENDING_LECTURER',
    'ACTIVE',
    'CLOSED',
    'AUTO_CLOSED'
);


ALTER TYPE public.session_status_enum OWNER TO qaat;

--
-- Name: sync_status_enum; Type: TYPE; Schema: public; Owner: qaat
--

CREATE TYPE public.sync_status_enum AS ENUM (
    'PENDING',
    'UPLOADING',
    'SYNCED',
    'FAILED'
);


ALTER TYPE public.sync_status_enum OWNER TO qaat;

--
-- Name: user_role_enum; Type: TYPE; Schema: public; Owner: qaat
--

CREATE TYPE public.user_role_enum AS ENUM (
    'COORDINATOR',
    'QA_OFFICER',
    'DQA_DIRECTOR',
    'VC',
    'DVC',
    'ADMIN',
    'STUDENT',
    'LECTURER',
    'QA_PATROLLER',
    'HOD',
    'DEAN',
    'QA_SCHOOL_HANDLER',
    'QA_DEPT_REP',
    'TLC'
);


ALTER TYPE public.user_role_enum OWNER TO qaat;

--
-- Name: offering_delivery_mode_default(); Type: FUNCTION; Schema: public; Owner: qaat
--

CREATE FUNCTION public.offering_delivery_mode_default() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF NEW.delivery_mode = 'IN_PERSON'
       AND NEW.session_type ~* '(distance|e[[:space:]._-]*learning|online|virtual|remote)' THEN
        NEW.delivery_mode := 'ONLINE';
    END IF;
    RETURN NEW;
END;
$$;


ALTER FUNCTION public.offering_delivery_mode_default() OWNER TO qaat;

--
-- Name: FUNCTION offering_delivery_mode_default(); Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON FUNCTION public.offering_delivery_mode_default() IS 'Sets delivery_mode = ONLINE for a newly created cohort whose session_type names it as a distance / e-learning run. INSERT only: an explicit UPDATE from the admin screen always wins.';


--
-- Name: refresh_attendance_summary(uuid); Type: FUNCTION; Schema: public; Owner: qaat
--

CREATE FUNCTION public.refresh_attendance_summary(p_tenant uuid) RETURNS void
    LANGUAGE plpgsql SECURITY DEFINER
    AS $$
BEGIN
    DELETE FROM student_attendance_summary WHERE tenant_id = p_tenant;

    INSERT INTO student_attendance_summary
        (student_id, unit_id, unit_name, course_id, tenant_id,
         sessions_held, sessions_attended, attendance_percentage)
    SELECT
        se.student_id,
        cu.unit_id,
        cu.name,
        cu.course_id,
        cu.tenant_id,
        h.n_held,
        COUNT(DISTINCT al.session_id),
        ROUND(COUNT(DISTINCT al.session_id)::DECIMAL / NULLIF(h.n_held, 0) * 100, 2)
    FROM students_extended se
    JOIN course_units cu
        ON  cu.course_id = se.course_id
        AND cu.tenant_id = se.tenant_id
    -- how many sessions the unit actually HELD (independent of any one student)
    JOIN LATERAL (
        SELECT COUNT(*) AS n_held
        FROM sessions s
        WHERE s.unit_id = cu.unit_id
          AND s.tenant_id = cu.tenant_id
          AND s.session_status IN ('CLOSED', 'AUTO_CLOSED')
    ) h ON h.n_held > 0
    -- which of those the student attended
    LEFT JOIN attendance_logs al
        ON  al.tenant_id  = cu.tenant_id
        AND al.student_id = se.student_id
        AND al.session_id IN (
            SELECT s2.session_id FROM sessions s2
            WHERE s2.unit_id = cu.unit_id
              AND s2.tenant_id = cu.tenant_id
              AND s2.session_status IN ('CLOSED', 'AUTO_CLOSED')
        )
    WHERE se.tenant_id = p_tenant
      AND se.enrollment_status = 'ACTIVE'
    GROUP BY se.student_id, cu.unit_id, cu.name, cu.course_id, cu.tenant_id, h.n_held;
END;
$$;


ALTER FUNCTION public.refresh_attendance_summary(p_tenant uuid) OWNER TO qaat;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: admin_audit_log; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.admin_audit_log (
    audit_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    actor_id character varying(50) NOT NULL,
    actor_role character varying(30) NOT NULL,
    action character varying(100) NOT NULL,
    target_type character varying(50),
    target_id character varying(100),
    payload jsonb,
    ip_address inet,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.admin_audit_log OWNER TO qaat;

--
-- Name: app_notifications; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.app_notifications (
    notification_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    sender_id uuid,
    sender_name text NOT NULL,
    sender_role text NOT NULL,
    audience text NOT NULL,
    unit_id character varying(50),
    subject text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    action text,
    action_ref text
);


ALTER TABLE public.app_notifications OWNER TO qaat;

--
-- Name: COLUMN app_notifications.sender_id; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.app_notifications.sender_id IS 'The user who sent this, or NULL when the institution did: scheduler alerts and QA patrol observations have no personal sender. The inbox query LEFT JOINs users on this to prefix a title, so NULL is what keeps an impersonal sender_name impersonal.';


--
-- Name: COLUMN app_notifications.action; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.app_notifications.action IS 'What the recipient can DO about this message, if anything. APPEAL_NOT_TAUGHT means a QA monitor recorded the lecturer as not teaching and the lecturer may file their own account of it. NULL for the ordinary message that is only to be read.';


--
-- Name: COLUMN app_notifications.action_ref; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.app_notifications.action_ref IS 'Which lecture the action is about, as unit_id|YYYY-MM-DD|HH:MM — the same key the round and lecturer_patrol_logs use, so a reply can be matched to the tick it answers.';


--
-- Name: attendance_logs; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.attendance_logs (
    log_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    session_id uuid NOT NULL,
    student_id character varying(50) NOT NULL,
    checkin_timestamp timestamp with time zone NOT NULL,
    device_fingerprint_hash character varying(128),
    sequence_number integer NOT NULL,
    entry_method public.entry_method_enum DEFAULT 'QR_SCAN'::public.entry_method_enum NOT NULL,
    override_officer_id character varying(50),
    override_reason text,
    audit_flags text[] DEFAULT '{}'::text[] NOT NULL,
    coordinator_id character varying(50)
);


ALTER TABLE public.attendance_logs OWNER TO qaat;

--
-- Name: coordinator_delegations; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.coordinator_delegations (
    delegation_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    offering_id uuid NOT NULL,
    coordinator_id character varying(50) NOT NULL,
    deputy_student_id character varying(50) NOT NULL,
    deputy_name text,
    code character varying(16) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone
);


ALTER TABLE public.coordinator_delegations OWNER TO qaat;

--
-- Name: course_offerings; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.course_offerings (
    offering_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    course_id character varying(50) NOT NULL,
    session_type character varying(40) NOT NULL,
    coordinator_id character varying(50),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    study_year smallint DEFAULT 1 NOT NULL,
    semester smallint DEFAULT 1 NOT NULL,
    level character varying(40) DEFAULT ''::character varying NOT NULL,
    intake character varying(64) DEFAULT ''::character varying NOT NULL,
    delivery_mode text DEFAULT 'IN_PERSON'::text NOT NULL,
    CONSTRAINT course_offerings_delivery_mode_check CHECK ((delivery_mode = ANY (ARRAY['IN_PERSON'::text, 'ONLINE'::text])))
);


ALTER TABLE public.course_offerings OWNER TO qaat;

--
-- Name: COLUMN course_offerings.delivery_mode; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.course_offerings.delivery_mode IS 'IN_PERSON (default) or ONLINE. ONLINE cohorts are distance / e-learning runs: they have no room and no hotspot, so their sessions take the online check-in path. Only a cohort marked ONLINE may have an online session opened for it — that restriction is what stops the remote path being used to mark a campus class present from off-site.';


--
-- Name: course_units; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.course_units (
    unit_id character varying(50) NOT NULL,
    tenant_id uuid NOT NULL,
    course_id character varying(50) NOT NULL,
    name character varying(200) NOT NULL,
    year smallint,
    semester smallint,
    academic_year character varying(20),
    default_venue_id character varying(50),
    session_start time without time zone,
    session_duration_minutes integer,
    schedule_locked boolean DEFAULT false NOT NULL,
    level character varying(40) DEFAULT ''::character varying NOT NULL,
    CONSTRAINT course_units_semester_check CHECK ((semester = ANY (ARRAY[1, 2])))
);


ALTER TABLE public.course_units OWNER TO qaat;

--
-- Name: courses; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.courses (
    course_id character varying(50) NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(200) NOT NULL,
    coordinator_id character varying(50),
    department character varying(100),
    school character varying(100),
    total_years smallint DEFAULT 3,
    level character varying(40),
    course_group character varying(160),
    level_years jsonb DEFAULT '{}'::jsonb NOT NULL,
    school_id uuid,
    department_id uuid,
    CONSTRAINT courses_total_years_check CHECK (((total_years >= 1) AND (total_years <= 8)))
);


ALTER TABLE public.courses OWNER TO qaat;

--
-- Name: departments; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.departments (
    department_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    school_id uuid,
    name character varying(200) NOT NULL,
    kind character varying(20) DEFAULT 'ACADEMIC'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.departments OWNER TO qaat;

--
-- Name: employee_attendance_days; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.employee_attendance_days (
    day_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    emp_no character varying(40),
    ac_no character varying(40) NOT NULL,
    seq_no character varying(40),
    full_name text DEFAULT ''::text NOT NULL,
    auto_assign text,
    department character varying(160),
    work_date date NOT NULL,
    timetable character varying(60),
    on_duty character varying(10),
    off_duty character varying(10),
    clock_in character varying(10),
    clock_out character varying(10),
    normal numeric(8,2),
    real_time numeric(8,2),
    late character varying(10),
    early character varying(10),
    absent boolean DEFAULT false NOT NULL,
    ot_time character varying(10),
    work_time character varying(10),
    exception text,
    must_cin boolean DEFAULT false NOT NULL,
    must_cout boolean DEFAULT false NOT NULL,
    ndays numeric(8,2),
    weekend numeric(8,2),
    holiday numeric(8,2),
    att_time character varying(10),
    ndays_ot numeric(8,2),
    weekend_ot numeric(8,2),
    holiday_ot numeric(8,2),
    checked_in_late boolean DEFAULT false NOT NULL,
    checked_out_early boolean DEFAULT false NOT NULL,
    source_file text,
    imported_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.employee_attendance_days OWNER TO qaat;

--
-- Name: employee_attendance_logs; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.employee_attendance_logs (
    log_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    staff_id character varying(60) NOT NULL,
    event_time timestamp with time zone NOT NULL,
    event_type character varying(8) DEFAULT 'PUNCH'::character varying NOT NULL,
    source character varying(20) DEFAULT 'TABLET'::character varying NOT NULL,
    device_id character varying(80),
    comment text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.employee_attendance_logs OWNER TO qaat;

--
-- Name: employees; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.employees (
    employee_pk uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    staff_id character varying(60) NOT NULL,
    title character varying(40),
    full_name text NOT NULL,
    department character varying(160),
    job_title character varying(160),
    email character varying(200),
    phone character varying(40),
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.employees OWNER TO qaat;

--
-- Name: hardware_vault; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.hardware_vault (
    student_id character varying(50) NOT NULL,
    tenant_id uuid NOT NULL,
    fingerprint_hash character varying(128) NOT NULL,
    first_bound_at timestamp with time zone NOT NULL,
    last_verified_at timestamp with time zone,
    academic_year character varying(20) NOT NULL
);


ALTER TABLE public.hardware_vault OWNER TO qaat;

--
-- Name: lecturer_assignments; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturer_assignments (
    assignment_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    lecturer_id uuid NOT NULL,
    unit_id character varying(50) NOT NULL,
    course_id character varying(50) NOT NULL,
    academic_year text NOT NULL,
    year smallint DEFAULT 1 NOT NULL,
    semester smallint DEFAULT 1 NOT NULL,
    intake_session character varying(64) DEFAULT 'Morning'::public.intake_session_enum NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.lecturer_assignments OWNER TO qaat;

--
-- Name: lecturer_attendance_logs; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturer_attendance_logs (
    log_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    session_id uuid NOT NULL,
    lecturer_id character varying(50) NOT NULL,
    gate_open_time timestamp with time zone NOT NULL,
    gate_close_time timestamp with time zone,
    contact_hours numeric(4,2),
    unit_id character varying(50) NOT NULL,
    venue_id character varying(50),
    session_date date NOT NULL,
    lecturer_scanned_at timestamp with time zone,
    lecturer_fingerprint_hash character varying(64),
    lecturer_ended_at timestamp with time zone,
    lecturer_end_fingerprint_hash character varying(128)
);


ALTER TABLE public.lecturer_attendance_logs OWNER TO qaat;

--
-- Name: lecturer_biometric_templates; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturer_biometric_templates (
    template_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    lecturer_id uuid NOT NULL,
    template bytea NOT NULL,
    template_format text DEFAULT 'ISO_19794_2'::text NOT NULL,
    finger_position smallint,
    reader_model text,
    enrolled_by character varying(50),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.lecturer_biometric_templates OWNER TO qaat;

--
-- Name: lecturer_daily_codes; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturer_daily_codes (
    tenant_id uuid NOT NULL,
    lecturer_id text NOT NULL,
    code text NOT NULL,
    valid_date date DEFAULT CURRENT_DATE NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.lecturer_daily_codes OWNER TO qaat;

--
-- Name: lecturer_patrol_logs; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturer_patrol_logs (
    patrol_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    unit_id character varying(50) NOT NULL,
    unit_name character varying(200),
    course_code character varying(50),
    lecturer_id character varying(50),
    lecturer_name character varying(200),
    room text,
    session_date date NOT NULL,
    scheduled_time text,
    taught boolean NOT NULL,
    patroller_id uuid NOT NULL,
    patroller_name character varying(200),
    patroller_staff_id character varying(50),
    taken_at timestamp with time zone DEFAULT now() NOT NULL,
    entry_method text DEFAULT 'PATROL'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    submission_id uuid,
    remarks text,
    patroller_device_hash character varying(128),
    found_venue text,
    found_start_time text,
    found_date date,
    venue_changed boolean DEFAULT false NOT NULL,
    offering_id uuid,
    is_compensation boolean DEFAULT false NOT NULL,
    compensation_for date,
    students_counted integer,
    class_group text,
    school text,
    department text,
    compensation_for_at timestamp with time zone,
    end_time text
);


ALTER TABLE public.lecturer_patrol_logs OWNER TO qaat;

--
-- Name: COLUMN lecturer_patrol_logs.is_compensation; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.is_compensation IS 'The monitor found this lecture being taught to make good an earlier one. Set by the monitor in the round, because they are the only witness present when it can be established.';


--
-- Name: COLUMN lecturer_patrol_logs.compensation_for; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.compensation_for IS 'The date of the lecture being made good, when the monitor was told which one. NULL means the compensation is recorded but unattributed — still true, and better than discarding it.';


--
-- Name: COLUMN lecturer_patrol_logs.students_counted; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.students_counted IS 'Heads counted in the room by the monitor. NULL for a timetabled observation, where the coordinator''s check-in ledger is the authority. Never merged with that ledger: an estimate and a register are different claims.';


--
-- Name: COLUMN lecturer_patrol_logs.class_group; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.class_group IS 'The cohort as the monitor wrote it, for a lecture with no offering to read it from.';


--
-- Name: COLUMN lecturer_patrol_logs.school; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.school IS 'The college, inherited from the course unit when it is a known one and typed when it is not.';


--
-- Name: COLUMN lecturer_patrol_logs.department; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.department IS 'The department owning the primary course unit. Inherited from the curriculum when the unit was picked, typed when it was not.';


--
-- Name: COLUMN lecturer_patrol_logs.compensation_for_at; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.compensation_for_at IS 'The date AND START TIME of the lecture this one makes good. Required whenever is_compensation is true — without the time a compensation cannot be matched to the lecture it claims to replace. compensation_for holds its date, for readers written before this existed.';


--
-- Name: COLUMN lecturer_patrol_logs.end_time; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.lecturer_patrol_logs.end_time IS 'HH:MM the lecture was due to end. scheduled_time is when it began; the pair is what the monitor is shown and what contact-hour reporting needs. Nullable: older records have no end.';


--
-- Name: lecturer_presence_claims; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturer_presence_claims (
    claim_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    lecturer_user_id uuid NOT NULL,
    lecturer_staff_id character varying(50) DEFAULT ''::character varying NOT NULL,
    lecturer_name text DEFAULT ''::text NOT NULL,
    latitude double precision,
    longitude double precision,
    accuracy_metres double precision,
    location_status text DEFAULT 'OK'::text NOT NULL,
    captured_at timestamp with time zone NOT NULL,
    unit_id character varying(50),
    unit_name text DEFAULT ''::text NOT NULL,
    room text DEFAULT ''::text NOT NULL,
    day_of_week smallint,
    scheduled_time character varying(5) DEFAULT ''::character varying NOT NULL,
    session_date date,
    match_kind text DEFAULT 'NONE'::text NOT NULL,
    minutes_from_start integer,
    note text DEFAULT ''::text NOT NULL,
    device_hash text DEFAULT ''::text NOT NULL,
    received_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT lecturer_presence_claims_location_status_check CHECK ((location_status = ANY (ARRAY['OK'::text, 'NO_FIX'::text, 'PERMISSION_DENIED'::text, 'DISABLED'::text]))),
    CONSTRAINT lecturer_presence_claims_match_kind_check CHECK ((match_kind = ANY (ARRAY['IN_SLOT'::text, 'NEAR_SLOT'::text, 'NEAREST'::text, 'NONE'::text])))
);


ALTER TABLE public.lecturer_presence_claims OWNER TO qaat;

--
-- Name: TABLE lecturer_presence_claims; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON TABLE public.lecturer_presence_claims IS 'A lecturer''s own contemporaneous record of being in the room: GPS fix, phone clock, and the timetabled slot the phone matched them to. Filed offline, synced later, read beside the QA patrol log for the same slot. Append-only.';


--
-- Name: lecturer_webauthn_credentials; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturer_webauthn_credentials (
    credential_id text NOT NULL,
    tenant_id uuid NOT NULL,
    lecturer_id uuid NOT NULL,
    public_key bytea NOT NULL,
    attestation_type text DEFAULT ''::text NOT NULL,
    aaguid bytea,
    sign_count bigint DEFAULT 0 NOT NULL,
    transports text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone,
    credential jsonb
);


ALTER TABLE public.lecturer_webauthn_credentials OWNER TO qaat;

--
-- Name: lecturers; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.lecturers (
    lecturer_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    full_name text NOT NULL,
    email text,
    phone text,
    department text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    staff_id character varying(64),
    title character varying(40),
    gender character varying(20),
    user_id uuid,
    school_id uuid
);


ALTER TABLE public.lecturers OWNER TO qaat;

--
-- Name: monitor_log_units; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.monitor_log_units (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    patrol_id uuid NOT NULL,
    unit_id character varying(50) NOT NULL,
    unit_name character varying(200),
    course_code character varying(50),
    class_group text,
    school text,
    department text,
    resolved boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE ONLY public.monitor_log_units FORCE ROW LEVEL SECURITY;


ALTER TABLE public.monitor_log_units OWNER TO qaat;

--
-- Name: TABLE monitor_log_units; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON TABLE public.monitor_log_units IS 'The ADDITIONAL course units covered by one observed lecture. The same hour of teaching often satisfies several unit codes belonging to different programmes and departments; the first is on lecturer_patrol_logs, the rest are here, so every one of them can be counted.';


--
-- Name: notification_log; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.notification_log (
    log_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    kind character varying(40) NOT NULL,
    subject_key character varying(128) NOT NULL,
    subject_date date NOT NULL,
    recipient_user_id uuid,
    channels text DEFAULT 'APP'::text NOT NULL,
    sent_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.notification_log OWNER TO qaat;

--
-- Name: notification_recipients; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.notification_recipients (
    notification_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    recipient_user_id uuid NOT NULL,
    read_at timestamp with time zone,
    dismissed_at timestamp with time zone
);


ALTER TABLE public.notification_recipients OWNER TO qaat;

--
-- Name: offering_unit_schedules; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.offering_unit_schedules (
    offering_id uuid NOT NULL,
    unit_id character varying(50) NOT NULL,
    tenant_id uuid NOT NULL,
    day_of_week smallint,
    session_start time without time zone,
    session_duration_minutes integer,
    schedule_locked boolean DEFAULT false NOT NULL,
    CONSTRAINT offering_unit_schedules_day_of_week_check CHECK (((day_of_week >= 1) AND (day_of_week <= 7)))
);


ALTER TABLE public.offering_unit_schedules OWNER TO qaat;

--
-- Name: patroller_device_bindings; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.patroller_device_bindings (
    user_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    device_fingerprint_hash character varying(128) NOT NULL,
    bound_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.patroller_device_bindings OWNER TO qaat;

--
-- Name: patroller_pins; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.patroller_pins (
    user_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    pin_hash text NOT NULL,
    set_at timestamp with time zone DEFAULT now() NOT NULL,
    last_verified_at timestamp with time zone,
    failed_attempts integer DEFAULT 0 NOT NULL,
    locked_until timestamp with time zone
);


ALTER TABLE public.patroller_pins OWNER TO qaat;

--
-- Name: qa_message_reads; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.qa_message_reads (
    message_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    read_at timestamp with time zone DEFAULT now(),
    dismissed_at timestamp with time zone
);


ALTER TABLE public.qa_message_reads OWNER TO qaat;

--
-- Name: qa_messages; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.qa_messages (
    message_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    sender_id uuid NOT NULL,
    sender_name text NOT NULL,
    sender_role text NOT NULL,
    audience text NOT NULL,
    audience_value text,
    subject text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    attachment_name text,
    attachment_mime text,
    attachment_data bytea,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT qa_messages_audience_check CHECK ((audience = ANY (ARRAY['ALL_QA'::text, 'DEPARTMENT'::text, 'SCHOOL'::text, 'DQA'::text])))
);


ALTER TABLE public.qa_messages OWNER TO qaat;

--
-- Name: qa_rep_submissions; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.qa_rep_submissions (
    submission_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    submitted_by uuid NOT NULL,
    submitter_name character varying(200),
    submitter_role character varying(40) NOT NULL,
    scope_kind character varying(20) NOT NULL,
    department character varying(120),
    school character varying(120),
    period_label character varying(60),
    period_from date,
    period_to date,
    notes text,
    file_name character varying(255) NOT NULL,
    file_size integer NOT NULL,
    file_bytes bytea NOT NULL,
    total_rows integer DEFAULT 0 NOT NULL,
    parsed_rows integer DEFAULT 0 NOT NULL,
    skipped_rows integer DEFAULT 0 NOT NULL,
    parse_errors text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.qa_rep_submissions OWNER TO qaat;

--
-- Name: scheduled_job_runs; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.scheduled_job_runs (
    job_name character varying(64) NOT NULL,
    last_run_at timestamp with time zone NOT NULL,
    last_status character varying(20) DEFAULT 'OK'::character varying NOT NULL,
    last_error text,
    last_duration_ms integer,
    windows_caught_up integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.scheduled_job_runs OWNER TO qaat;

--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.schema_migrations (
    version text NOT NULL,
    name text NOT NULL,
    checksum text NOT NULL,
    adopted boolean DEFAULT false NOT NULL,
    statements integer DEFAULT 0 NOT NULL,
    skipped integer DEFAULT 0 NOT NULL,
    applied_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.schema_migrations OWNER TO qaat;

--
-- Name: schools; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.schools (
    school_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(200) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    abbreviation character varying(32)
);


ALTER TABLE public.schools OWNER TO qaat;

--
-- Name: semester_archives; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.semester_archives (
    archive_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    label text NOT NULL,
    intakes text[] DEFAULT '{}'::text[] NOT NULL,
    academic_year text,
    semester integer,
    filename text NOT NULL,
    content bytea NOT NULL,
    size_bytes bigint DEFAULT 0 NOT NULL,
    attendance_rows integer DEFAULT 0 NOT NULL,
    session_rows integer DEFAULT 0 NOT NULL,
    lecturer_rows integer DEFAULT 0 NOT NULL,
    created_by text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.semester_archives OWNER TO qaat;

--
-- Name: sessions; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.sessions (
    session_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    coordinator_id character varying(50) NOT NULL,
    unit_id character varying(50) NOT NULL,
    lecturer_id character varying(50),
    venue_id character varying(50),
    session_date date NOT NULL,
    gate_open_time timestamp with time zone,
    gate_close_time timestamp with time zone,
    checkin_window_start timestamp with time zone,
    checkin_window_end timestamp with time zone,
    coordinator_end_time timestamp with time zone,
    auto_close_time timestamp with time zone,
    session_status public.session_status_enum DEFAULT 'PENDING_LECTURER'::public.session_status_enum NOT NULL,
    warden_id character varying(50),
    sync_status public.sync_status_enum DEFAULT 'PENDING'::public.sync_status_enum NOT NULL,
    audit_flags text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    checkin_secret bytea,
    planned_start time without time zone,
    planned_duration_minutes integer,
    offering_id uuid,
    coordinator_ip character varying(64),
    room_is_provision boolean DEFAULT false NOT NULL,
    provision_note text,
    unscheduled boolean DEFAULT false NOT NULL,
    delivery_mode text DEFAULT 'IN_PERSON'::text NOT NULL,
    CONSTRAINT sessions_delivery_mode_check CHECK ((delivery_mode = ANY (ARRAY['IN_PERSON'::text, 'ONLINE'::text])))
);


ALTER TABLE public.sessions OWNER TO qaat;

--
-- Name: COLUMN sessions.room_is_provision; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.sessions.room_is_provision IS 'The session ran in a room other than its timetabled one. Set by the coordinator when they pick a free room; notifies the QA monitors so the round is redirected before the visit.';


--
-- Name: COLUMN sessions.provision_note; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.sessions.provision_note IS 'The coordinator''s reason for the substitution, in their own words. Optional by design.';


--
-- Name: COLUMN sessions.unscheduled; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.sessions.unscheduled IS 'The unit was not on the timetable for this weekday when the session was opened. Students check in through the identical flow; the flag exists so the attendance record says the lecture was off-timetable rather than leaving a reader to infer it from a missing slot.';


--
-- Name: COLUMN sessions.delivery_mode; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.sessions.delivery_mode IS 'How this session was actually delivered. ONLINE sessions have no venue and no coordinator IP: the LAN proximity gate is not applied to them, the ROTATING code is required instead of the static one, and the student must be enrolled in the session''s cohort. Recorded on the day so a later edit to the cohort cannot rewrite what an attendance record meant when it was taken.';


--
-- Name: student_attendance_summary; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.student_attendance_summary (
    student_id character varying(50) NOT NULL,
    unit_id character varying(50) NOT NULL,
    unit_name character varying(200),
    course_id character varying(50),
    tenant_id uuid NOT NULL,
    sessions_held integer DEFAULT 0 NOT NULL,
    sessions_attended integer DEFAULT 0 NOT NULL,
    attendance_percentage numeric(5,2) DEFAULT 0 NOT NULL,
    refreshed_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.student_attendance_summary OWNER TO qaat;

--
-- Name: student_device_bindings; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.student_device_bindings (
    student_id character varying(50) NOT NULL,
    tenant_id uuid NOT NULL,
    device_fingerprint_hash character varying(128) NOT NULL,
    attend_block_until timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.student_device_bindings OWNER TO qaat;

--
-- Name: students_extended; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.students_extended (
    student_id character varying(50) NOT NULL,
    tenant_id uuid NOT NULL,
    full_name character varying(200) NOT NULL,
    email character varying(255) NOT NULL,
    course_id character varying(50) NOT NULL,
    intake_session character varying(64),
    current_year smallint,
    semester smallint,
    academic_year character varying(20) NOT NULL,
    enrollment_status public.enrollment_status_enum DEFAULT 'ACTIVE'::public.enrollment_status_enum NOT NULL,
    hardware_fingerprint character varying(128),
    rebind_count smallint DEFAULT 0 NOT NULL,
    last_rebind_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    offering_id uuid,
    level character varying(40),
    course_group character varying(160),
    CONSTRAINT students_extended_current_year_check CHECK (((current_year >= 1) AND (current_year <= 6))),
    CONSTRAINT students_extended_rebind_count_check CHECK ((rebind_count <= 2)),
    CONSTRAINT students_extended_semester_check CHECK ((semester = ANY (ARRAY[1, 2])))
);


ALTER TABLE public.students_extended OWNER TO qaat;

--
-- Name: sync_uploads; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.sync_uploads (
    upload_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    coordinator_id character varying(50) NOT NULL,
    session_ids uuid[] NOT NULL,
    total_chunks smallint NOT NULL,
    received_chunks integer[] DEFAULT '{}'::integer[] NOT NULL,
    package_checksum character varying(64) NOT NULL,
    status public.sync_status_enum DEFAULT 'PENDING'::public.sync_status_enum NOT NULL,
    chunk_size_bytes integer DEFAULT 65536 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    package_hmac character varying(64)
);


ALTER TABLE public.sync_uploads OWNER TO qaat;

--
-- Name: tenants; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.tenants (
    tenant_id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(200) NOT NULL,
    domain character varying(100) NOT NULL,
    rsa_key_id character varying(100) NOT NULL,
    attendance_threshold smallint DEFAULT 75 NOT NULL,
    checkin_window_minutes smallint DEFAULT 120 NOT NULL,
    auto_kill_minutes smallint DEFAULT 180 NOT NULL,
    logo_url text,
    brand_color character varying(7),
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    student_hash_key character varying(64) DEFAULT encode(public.gen_random_bytes(32), 'hex'::text) NOT NULL,
    active_academic_year text DEFAULT ''::text NOT NULL,
    active_semester smallint DEFAULT 0 NOT NULL,
    motto text,
    slogan text,
    address text,
    sidebar_color character varying(7),
    background_color character varying(7),
    footer_color character varying(7),
    institution_id character varying(40),
    intakes text[] DEFAULT ARRAY['January Intake'::text, 'May Intake'::text, 'August Intake'::text] NOT NULL,
    levels text[] DEFAULT ARRAY['Certificate'::text, 'Diploma'::text, 'Degree'::text, 'Masters'::text, 'PhD'::text] NOT NULL,
    study_sessions text[] DEFAULT ARRAY['Morning'::text, 'Day'::text, 'Evening'::text, 'Distance'::text, 'Weekend'::text] NOT NULL,
    staff_id_prefix character varying(16),
    titles text[] DEFAULT ARRAY['Prof.'::text, 'Dr.'::text, 'Eng.'::text, 'Mr.'::text, 'Mrs.'::text, 'Ms.'::text] NOT NULL,
    lecturer_attendance_ratio numeric DEFAULT 0.5 NOT NULL,
    session_window_start time without time zone DEFAULT '08:00:00'::time without time zone NOT NULL,
    session_window_end time without time zone DEFAULT '17:00:00'::time without time zone NOT NULL,
    session_active_days smallint[] DEFAULT '{1,2,3,4,5,6}'::smallint[] NOT NULL,
    users_passcode_hash text,
    require_lan_proximity boolean DEFAULT true NOT NULL,
    level_years jsonb DEFAULT '{}'::jsonb NOT NULL,
    background_image text,
    background_blur smallint DEFAULT 0 NOT NULL,
    background_brightness smallint DEFAULT 100 NOT NULL,
    background_contrast smallint DEFAULT 100 NOT NULL,
    background_overlay_color character varying(9),
    background_overlay_opacity smallint DEFAULT 0 NOT NULL,
    text_color_light character varying(7),
    text_color_dark character varying(7),
    CONSTRAINT tenants_active_semester_check CHECK ((active_semester = ANY (ARRAY[0, 1, 2]))),
    CONSTRAINT tenants_attendance_threshold_check CHECK (((attendance_threshold >= 1) AND (attendance_threshold <= 100))),
    CONSTRAINT tenants_auto_kill_minutes_check CHECK ((auto_kill_minutes > 0)),
    CONSTRAINT tenants_checkin_window_minutes_check CHECK ((checkin_window_minutes > 0))
);


ALTER TABLE public.tenants OWNER TO qaat;

--
-- Name: timetable_slots; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.timetable_slots (
    slot_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    offering_id uuid NOT NULL,
    unit_id character varying(50) NOT NULL,
    day_of_week smallint NOT NULL,
    start_time time without time zone NOT NULL,
    duration_minutes integer DEFAULT 60 NOT NULL,
    room text,
    lecturer_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    venue_id character varying(50),
    CONSTRAINT timetable_slots_day_of_week_check CHECK (((day_of_week >= 1) AND (day_of_week <= 7)))
);


ALTER TABLE public.timetable_slots OWNER TO qaat;

--
-- Name: user_schools; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.user_schools (
    user_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    school_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.user_schools OWNER TO qaat;

--
-- Name: users; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.users (
    user_id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(72) NOT NULL,
    role public.user_role_enum NOT NULL,
    full_name character varying(200) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    totp_secret_enc text,
    totp_enabled boolean DEFAULT false NOT NULL,
    totp_backup_codes_enc text,
    device_binding_key_enc text,
    last_login_at timestamp with time zone,
    failed_login_count smallint DEFAULT 0 NOT NULL,
    locked_until timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    coordinator_code character varying(32),
    phone character varying(40),
    whatsapp character varying(40),
    registration_number character varying(64),
    title character varying(40),
    gender character varying(20),
    force_password_change boolean DEFAULT false NOT NULL,
    department character varying(120),
    school character varying(120),
    staff_id character varying(50)
);


ALTER TABLE public.users OWNER TO qaat;

--
-- Name: COLUMN users.department; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON COLUMN public.users.department IS 'The org unit this account is confined to. For HOD, and now TLC, it is the department they are responsible for; a TLC with no department is institution-wide, which is what TLC accounts created before migration 083 are.';


--
-- Name: venues; Type: TABLE; Schema: public; Owner: qaat
--

CREATE TABLE public.venues (
    venue_id character varying(50) NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(200) NOT NULL,
    building character varying(100),
    floor smallint,
    capacity smallint,
    gps_latitude numeric(10,8),
    gps_longitude numeric(11,8),
    geofence_radius_meters smallint DEFAULT 50 NOT NULL,
    school_id uuid,
    department_id uuid,
    room_type character varying(30) DEFAULT 'LECTURE_HALL'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL
);


ALTER TABLE public.venues OWNER TO qaat;

--
-- Data for Name: admin_audit_log; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.admin_audit_log (audit_id, tenant_id, actor_id, actor_role, action, target_type, target_id, payload, ip_address, occurred_at) FROM stdin;
ab385f22-61d8-42bf-93ce-8c03b9c7f72b	13ab41a8-0a50-401c-a095-23203a8e41be	bb6873f4-f907-4d40-95cb-b8572b192cc2	QA_OFFICER	PATROL_DEVICE_RELEASED	user	05d7dc2c-daec-410d-974e-1d6aabea5421	{"note": "handset binding cleared; the patroller may claim a new phone on next sign-in"}	172.18.0.1	2026-08-05 20:43:44.336536+00
1155e379-2165-4882-8356-65421ddf2eeb	13ab41a8-0a50-401c-a095-23203a8e41be	20acba0b-0c24-4486-9eac-db646b5229eb	TLC	LECTURER_ASSIGNED	lecturer_assignments	f8e39b1c-efbf-4e15-bfe9-3d68c62c8264	{}	172.18.0.1	2026-08-08 15:03:52.979247+00
db055adb-81ae-4afc-9981-ab50ee18135e	13ab41a8-0a50-401c-a095-23203a8e41be	20acba0b-0c24-4486-9eac-db646b5229eb	TLC	LECTURER_ASSIGNED	lecturer_assignments	4cc966a9-c647-4029-a178-78851a844cf9	{}	172.18.0.1	2026-08-08 15:03:53.202829+00
6d293922-d88f-47b2-ad04-b3fdb25d9f13	13ab41a8-0a50-401c-a095-23203a8e41be	20acba0b-0c24-4486-9eac-db646b5229eb	TLC	LECTURER_UNASSIGNED	lecturer_assignments	f8e39b1c-efbf-4e15-bfe9-3d68c62c8264	{}	172.18.0.1	2026-08-08 15:03:53.628097+00
79fc9dfa-c943-4543-a24b-67c718f8b829	13ab41a8-0a50-401c-a095-23203a8e41be	44993acf-0538-4e5e-bbbd-a3f4280b3a0e	TLC	LECTURER_ASSIGNED	lecturer_assignments	3dbc2ae9-0a57-4fc6-a346-41a82d51a451	{}	172.18.0.1	2026-08-08 15:04:01.478982+00
6cb5e58f-ca6e-4d00-a9de-fdf01d411651	13ab41a8-0a50-401c-a095-23203a8e41be	44993acf-0538-4e5e-bbbd-a3f4280b3a0e	TLC	LECTURER_ASSIGNED	lecturer_assignments	e242a8f5-71b4-4eb0-8ae0-61c07e50947e	{}	172.18.0.1	2026-08-08 15:04:01.88169+00
38c5be50-4bb5-457f-ac78-4e7d0b064266	13ab41a8-0a50-401c-a095-23203a8e41be	44993acf-0538-4e5e-bbbd-a3f4280b3a0e	TLC	LECTURER_UNASSIGNED	lecturer_assignments	3dbc2ae9-0a57-4fc6-a346-41a82d51a451	{}	172.18.0.1	2026-08-08 15:04:02.407603+00
7d63781b-8cb5-44d4-9bcd-8925cf3d96ce	13ab41a8-0a50-401c-a095-23203a8e41be	90c9b41c-011a-4b25-8fcb-28767fe20c58	TLC	LECTURER_ASSIGNED	lecturer_assignments	29459c65-8f5a-453c-8ef9-ed2faeafd39d	{}	172.18.0.1	2026-08-08 15:05:25.147271+00
5125f52c-562d-4312-93e9-c7c2a87bd182	13ab41a8-0a50-401c-a095-23203a8e41be	90c9b41c-011a-4b25-8fcb-28767fe20c58	TLC	LECTURER_ASSIGNED	lecturer_assignments	755b4764-8c57-4bce-a5f1-e2c559fa894d	{}	172.18.0.1	2026-08-08 15:05:25.563276+00
2cdcbf0f-943c-4176-a0bf-49b14753d57a	13ab41a8-0a50-401c-a095-23203a8e41be	90c9b41c-011a-4b25-8fcb-28767fe20c58	TLC	LECTURER_UNASSIGNED	lecturer_assignments	29459c65-8f5a-453c-8ef9-ed2faeafd39d	{}	172.18.0.1	2026-08-08 15:05:26.300771+00
72fdce19-7595-4ecd-9728-5ef564246080	13ab41a8-0a50-401c-a095-23203a8e41be	fbd2cd42-5085-47e2-9674-608b676deffc	TLC	LECTURER_ASSIGNED	lecturer_assignments	7bc0fb08-1ef8-4886-9eef-4b32aeb3da7b	{}	172.18.0.1	2026-08-08 15:10:34.108277+00
f789e401-a163-42c3-b441-e5a2facb6b78	13ab41a8-0a50-401c-a095-23203a8e41be	fbd2cd42-5085-47e2-9674-608b676deffc	TLC	LECTURER_UNASSIGNED	lecturer_assignments	7bc0fb08-1ef8-4886-9eef-4b32aeb3da7b	{}	172.18.0.1	2026-08-08 15:10:34.905439+00
fbe43b91-9ace-4466-a4c0-515c2149f9ef	13ab41a8-0a50-401c-a095-23203a8e41be	28f9f58f-3551-4897-bdc1-e80d178bb87d	TLC	LECTURER_ASSIGNED	lecturer_assignments	08403b14-a34b-48e2-a382-7d970a86cd9c	{}	172.18.0.1	2026-08-08 15:11:06.976546+00
0e9e6eeb-3952-4730-a77d-e910ada70e21	13ab41a8-0a50-401c-a095-23203a8e41be	28f9f58f-3551-4897-bdc1-e80d178bb87d	TLC	LECTURER_UNASSIGNED	lecturer_assignments	08403b14-a34b-48e2-a382-7d970a86cd9c	{}	172.18.0.1	2026-08-08 15:11:07.714342+00
53af5176-962d-4d39-8257-44b7d98d4033	13ab41a8-0a50-401c-a095-23203a8e41be	8d9d646c-6d6f-4420-95dd-9e614f9b8911	TLC	LECTURER_ASSIGNED	lecturer_assignments	d210589f-06eb-4e80-9685-cac29d8a4a36	{}	172.18.0.1	2026-08-08 15:12:15.602446+00
be8b1ada-2f6a-406e-911e-2d75c5d0204c	13ab41a8-0a50-401c-a095-23203a8e41be	8d9d646c-6d6f-4420-95dd-9e614f9b8911	TLC	LECTURER_UNASSIGNED	lecturer_assignments	d210589f-06eb-4e80-9685-cac29d8a4a36	{}	172.18.0.1	2026-08-08 15:12:16.738636+00
1a138eed-e993-4e8b-9dff-32a0ba109e07	13ab41a8-0a50-401c-a095-23203a8e41be	aca98449-f905-4b68-9233-d23b51c73899	TLC	LECTURER_ASSIGNED	lecturer_assignments	21caee4a-da0d-4a64-a53e-a995003861a2	{}	172.18.0.1	2026-08-10 19:23:46.539195+00
7b045468-8046-48f1-b4cd-23f639cb57ae	13ab41a8-0a50-401c-a095-23203a8e41be	aca98449-f905-4b68-9233-d23b51c73899	TLC	LECTURER_UNASSIGNED	lecturer_assignments	21caee4a-da0d-4a64-a53e-a995003861a2	{}	172.18.0.1	2026-08-10 19:23:47.033256+00
3df3e1e3-eb6c-453d-bd58-d35e63ee48e3	13ab41a8-0a50-401c-a095-23203a8e41be	88bceac7-5045-4443-9dee-69044b60c8f8	TLC	LECTURER_ASSIGNED	lecturer_assignments	0c868cf3-05fa-47b4-81c2-12fce4c9305b	{}	172.18.0.1	2026-08-10 19:50:10.84555+00
dfb831e5-c503-49c6-aa7d-e686f00ec052	13ab41a8-0a50-401c-a095-23203a8e41be	88bceac7-5045-4443-9dee-69044b60c8f8	TLC	LECTURER_UNASSIGNED	lecturer_assignments	0c868cf3-05fa-47b4-81c2-12fce4c9305b	{}	172.18.0.1	2026-08-10 19:50:11.339339+00
1d2e494d-fe76-428e-8b32-f9150538c171	13ab41a8-0a50-401c-a095-23203a8e41be	80b3942e-2656-4931-9180-a1a7c07d6fbf	TLC	LECTURER_ASSIGNED	lecturer_assignments	30eb526c-05f1-41c0-8750-923774690b4d	{}	172.18.0.1	2026-08-10 20:15:47.910196+00
7f91611a-c449-461d-945a-b6806ce53ded	13ab41a8-0a50-401c-a095-23203a8e41be	80b3942e-2656-4931-9180-a1a7c07d6fbf	TLC	LECTURER_UNASSIGNED	lecturer_assignments	30eb526c-05f1-41c0-8750-923774690b4d	{}	172.18.0.1	2026-08-10 20:15:48.416531+00
ece4455f-7330-4dcb-ae6a-69d8a1c1fda6	13ab41a8-0a50-401c-a095-23203a8e41be	b95ea854-4562-46b3-8e47-b4c391c1ae0a	TLC	LECTURER_ASSIGNED	lecturer_assignments	b5812848-1b0e-4ca9-b112-7c89d139579f	{}	172.18.0.1	2026-08-10 20:26:48.493937+00
1e395d14-54e1-41b0-8ed8-025e12e4c0bf	13ab41a8-0a50-401c-a095-23203a8e41be	b95ea854-4562-46b3-8e47-b4c391c1ae0a	TLC	LECTURER_UNASSIGNED	lecturer_assignments	b5812848-1b0e-4ca9-b112-7c89d139579f	{}	172.18.0.1	2026-08-10 20:26:49.0818+00
b5dd2ebe-d2e6-40ba-ae3b-83783c68df9e	13ab41a8-0a50-401c-a095-23203a8e41be	7f302e8c-01e0-4bb8-9af7-f05b0a3ef101	TLC	LECTURER_ASSIGNED	lecturer_assignments	711e98fd-ef3a-4298-bbf1-2a296a50dce8	{}	172.18.0.1	2026-08-10 20:41:21.644786+00
6b178ae5-051e-4e78-a93a-9cc52dd7752a	13ab41a8-0a50-401c-a095-23203a8e41be	7f302e8c-01e0-4bb8-9af7-f05b0a3ef101	TLC	LECTURER_UNASSIGNED	lecturer_assignments	711e98fd-ef3a-4298-bbf1-2a296a50dce8	{}	172.18.0.1	2026-08-10 20:41:22.219569+00
38debc21-973b-47a5-bfda-8924eb022d74	13ab41a8-0a50-401c-a095-23203a8e41be	1050f7ca-c11d-4337-a4d2-7edc1ec0f428	TLC	LECTURER_ASSIGNED	lecturer_assignments	2a6b45c3-7e8d-41fa-b963-ed3c2366b553	{}	172.18.0.1	2026-08-10 21:34:43.530167+00
d21f3b5b-96b9-40ed-9c9a-d728fdd92eb7	13ab41a8-0a50-401c-a095-23203a8e41be	1050f7ca-c11d-4337-a4d2-7edc1ec0f428	TLC	LECTURER_UNASSIGNED	lecturer_assignments	2a6b45c3-7e8d-41fa-b963-ed3c2366b553	{}	172.18.0.1	2026-08-10 21:34:44.044233+00
8a5169d2-77f1-40eb-a703-9fa31dbb476a	13ab41a8-0a50-401c-a095-23203a8e41be	36dbee74-e60a-4812-a2cb-b3275d10959a	TLC	LECTURER_ASSIGNED	lecturer_assignments	1b97f8b0-627b-4efd-812e-83143a40f581	{}	172.18.0.1	2026-08-10 21:35:10.82595+00
840f5234-af9b-4b89-9fce-0c2126ed7171	13ab41a8-0a50-401c-a095-23203a8e41be	36dbee74-e60a-4812-a2cb-b3275d10959a	TLC	LECTURER_UNASSIGNED	lecturer_assignments	1b97f8b0-627b-4efd-812e-83143a40f581	{}	172.18.0.1	2026-08-10 21:35:11.407348+00
2b0a041b-23a4-4274-b1f6-bd39f334f5dd	13ab41a8-0a50-401c-a095-23203a8e41be	5ee8cdf3-138b-45ee-9561-722728d11194	TLC	LECTURER_ASSIGNED	lecturer_assignments	751b3fd4-fdef-49bb-b4f3-fc6a73669177	{}	172.18.0.1	2026-08-10 21:36:28.935387+00
12fd8a68-c1ca-41bc-bade-d81fd6922c24	13ab41a8-0a50-401c-a095-23203a8e41be	5ee8cdf3-138b-45ee-9561-722728d11194	TLC	LECTURER_UNASSIGNED	lecturer_assignments	751b3fd4-fdef-49bb-b4f3-fc6a73669177	{}	172.18.0.1	2026-08-10 21:36:29.501357+00
9b9d68c6-7f47-4207-95e6-44254fa3e54b	13ab41a8-0a50-401c-a095-23203a8e41be	9535d448-c6e6-40f9-b028-c83c0cc0d43d	TLC	LECTURER_ASSIGNED	lecturer_assignments	5e5129c3-820d-412a-8974-b35c3b7b0caf	{}	172.18.0.1	2026-08-10 21:36:51.403172+00
8305962a-8d2c-4457-a1bd-fead77c13734	13ab41a8-0a50-401c-a095-23203a8e41be	9535d448-c6e6-40f9-b028-c83c0cc0d43d	TLC	LECTURER_UNASSIGNED	lecturer_assignments	5e5129c3-820d-412a-8974-b35c3b7b0caf	{}	172.18.0.1	2026-08-10 21:36:51.968881+00
281011bd-7ff7-4073-af08-1170488bc9fd	13ab41a8-0a50-401c-a095-23203a8e41be	61f3b2d1-15c3-4d9c-8e4f-f4326d093534	TLC	LECTURER_ASSIGNED	lecturer_assignments	6d06fe85-eaeb-4727-bb13-b7c69df9ce52	{}	172.18.0.1	2026-08-10 21:37:53.626462+00
44bdf81c-88f4-4656-af74-17039f459a07	13ab41a8-0a50-401c-a095-23203a8e41be	61f3b2d1-15c3-4d9c-8e4f-f4326d093534	TLC	LECTURER_UNASSIGNED	lecturer_assignments	6d06fe85-eaeb-4727-bb13-b7c69df9ce52	{}	172.18.0.1	2026-08-10 21:37:54.106564+00
7a3c4853-7bc2-4d89-a634-252ac22c9c99	13ab41a8-0a50-401c-a095-23203a8e41be	4238c596-a729-4b18-bb8c-aaadb8ee3ffe	TLC	LECTURER_ASSIGNED	lecturer_assignments	2e25aaea-ea77-4610-bdb1-e7258911282b	{}	172.18.0.1	2026-08-10 21:39:13.950606+00
212a66e1-ba8b-421f-b094-a4708830b62b	13ab41a8-0a50-401c-a095-23203a8e41be	4238c596-a729-4b18-bb8c-aaadb8ee3ffe	TLC	LECTURER_UNASSIGNED	lecturer_assignments	2e25aaea-ea77-4610-bdb1-e7258911282b	{}	172.18.0.1	2026-08-10 21:39:14.44506+00
6f6e48ce-5987-4221-804b-5f4e4f107d0d	13ab41a8-0a50-401c-a095-23203a8e41be	8c482a48-f4b8-4800-a796-5bc9deb124cc	TLC	LECTURER_ASSIGNED	lecturer_assignments	ca2203ef-d8e7-4f1f-84c3-a3aba413913f	{}	172.18.0.1	2026-08-10 21:52:11.671725+00
35988158-87a2-47a5-ae39-a5ae694b887a	13ab41a8-0a50-401c-a095-23203a8e41be	8c482a48-f4b8-4800-a796-5bc9deb124cc	TLC	LECTURER_UNASSIGNED	lecturer_assignments	ca2203ef-d8e7-4f1f-84c3-a3aba413913f	{}	172.18.0.1	2026-08-10 21:52:12.163218+00
3a54cc27-5636-4809-9fc2-0574cf2a2776	13ab41a8-0a50-401c-a095-23203a8e41be	05d742f2-4d95-4648-92b8-b7dd462e2470	TLC	LECTURER_ASSIGNED	lecturer_assignments	5dfcdfbb-a840-4911-9551-da2f13f76945	{}	172.18.0.1	2026-08-10 23:18:00.039002+00
d435c06a-df9f-4317-b2d5-b1f4dd8d6e94	13ab41a8-0a50-401c-a095-23203a8e41be	05d742f2-4d95-4648-92b8-b7dd462e2470	TLC	LECTURER_UNASSIGNED	lecturer_assignments	5dfcdfbb-a840-4911-9551-da2f13f76945	{}	172.18.0.1	2026-08-10 23:18:00.550205+00
26e2d962-48f6-4223-aae3-a5b75753a0f7	13ab41a8-0a50-401c-a095-23203a8e41be	cdad16df-c4d0-4d2e-b7f3-c0de6f4ca9f1	TLC	LECTURER_ASSIGNED	lecturer_assignments	911daecc-6c80-44ed-9768-6800fe77386b	{}	172.18.0.1	2026-08-10 23:18:26.468401+00
81998806-4a2d-4903-8d52-bba292706cdd	13ab41a8-0a50-401c-a095-23203a8e41be	cdad16df-c4d0-4d2e-b7f3-c0de6f4ca9f1	TLC	LECTURER_UNASSIGNED	lecturer_assignments	911daecc-6c80-44ed-9768-6800fe77386b	{}	172.18.0.1	2026-08-10 23:18:27.014979+00
9dd5eb28-8f79-4077-99c7-49bcde9c3b81	13ab41a8-0a50-401c-a095-23203a8e41be	edac9fdc-a99d-4be7-ab28-687aeca93855	TLC	LECTURER_ASSIGNED	lecturer_assignments	146c9626-8686-43d6-bc33-986c1d3bd847	{}	172.18.0.1	2026-08-10 23:19:04.822151+00
c1546b33-c884-48c9-8510-3444a2e1d25e	13ab41a8-0a50-401c-a095-23203a8e41be	edac9fdc-a99d-4be7-ab28-687aeca93855	TLC	LECTURER_UNASSIGNED	lecturer_assignments	146c9626-8686-43d6-bc33-986c1d3bd847	{}	172.18.0.1	2026-08-10 23:19:05.308112+00
d6f816c0-6014-49f7-862a-b14659267c59	13ab41a8-0a50-401c-a095-23203a8e41be	34f8d844-e9e3-4af7-bb7a-b5361d8f159f	TLC	LECTURER_ASSIGNED	lecturer_assignments	c7519a1f-7d3f-4302-8e04-505092cf459d	{}	172.18.0.1	2026-08-10 23:22:56.474372+00
ef423101-96bb-4e79-82de-c5832f9c2973	13ab41a8-0a50-401c-a095-23203a8e41be	34f8d844-e9e3-4af7-bb7a-b5361d8f159f	TLC	LECTURER_UNASSIGNED	lecturer_assignments	c7519a1f-7d3f-4302-8e04-505092cf459d	{}	172.18.0.1	2026-08-10 23:22:57.063633+00
3d0e0ef8-f25a-4efc-a3ac-738f27f4a380	13ab41a8-0a50-401c-a095-23203a8e41be	8a17b6cb-8da7-4ff9-bf7b-af94490cd867	TLC	LECTURER_ASSIGNED	lecturer_assignments	6ed3805e-5868-4ac9-9d98-3e7054fdef33	{}	172.18.0.1	2026-08-10 23:53:52.841735+00
087bbc99-ce66-426f-8029-7b73b760742c	13ab41a8-0a50-401c-a095-23203a8e41be	8a17b6cb-8da7-4ff9-bf7b-af94490cd867	TLC	LECTURER_UNASSIGNED	lecturer_assignments	6ed3805e-5868-4ac9-9d98-3e7054fdef33	{}	172.18.0.1	2026-08-10 23:53:53.390749+00
58d38a66-2237-491e-9a47-eeba663a3193	13ab41a8-0a50-401c-a095-23203a8e41be	a0d62d9e-6ab6-4d8f-a03d-7ac4367c45d6	TLC	LECTURER_ASSIGNED	lecturer_assignments	cad73d73-7ee1-40e0-8778-fb63f113edd2	{}	172.18.0.1	2026-08-11 00:18:01.548371+00
b13925ab-e562-4ea8-b1cd-9e9f00d7b90d	13ab41a8-0a50-401c-a095-23203a8e41be	a0d62d9e-6ab6-4d8f-a03d-7ac4367c45d6	TLC	LECTURER_UNASSIGNED	lecturer_assignments	cad73d73-7ee1-40e0-8778-fb63f113edd2	{}	172.18.0.1	2026-08-11 00:18:02.06909+00
eaaa6578-1954-48c1-a2dd-3d060ed79a7e	13ab41a8-0a50-401c-a095-23203a8e41be	285991b7-2e84-4f93-8f3d-29032dfd50fd	TLC	LECTURER_ASSIGNED	lecturer_assignments	be34a6b7-35a4-4734-a1ee-0a2c35529842	{}	172.18.0.1	2026-08-11 00:25:19.911087+00
5463706b-b8a3-4d71-9a92-c9c93526d217	13ab41a8-0a50-401c-a095-23203a8e41be	285991b7-2e84-4f93-8f3d-29032dfd50fd	TLC	LECTURER_UNASSIGNED	lecturer_assignments	be34a6b7-35a4-4734-a1ee-0a2c35529842	{}	172.18.0.1	2026-08-11 00:25:20.414468+00
708f3810-5fea-4d79-885a-51d19a666dd6	13ab41a8-0a50-401c-a095-23203a8e41be	50396f67-7508-48c0-a5f3-b7ae54a140fa	TLC	LECTURER_ASSIGNED	lecturer_assignments	b8d95fac-989c-42bc-bd65-e640833610cb	{}	172.18.0.1	2026-08-11 00:45:28.183534+00
91e6b933-8b9d-4ee8-a25f-56522790d46e	13ab41a8-0a50-401c-a095-23203a8e41be	50396f67-7508-48c0-a5f3-b7ae54a140fa	TLC	LECTURER_UNASSIGNED	lecturer_assignments	b8d95fac-989c-42bc-bd65-e640833610cb	{}	172.18.0.1	2026-08-11 00:45:28.750608+00
4c9e9cb8-6f5b-4a6d-8c4b-e230614fca24	13ab41a8-0a50-401c-a095-23203a8e41be	c060a338-c537-4b82-9afc-1df645e1e5cf	TLC	LECTURER_ASSIGNED	lecturer_assignments	8394b163-ab60-48a5-bcb1-aa2c787aa026	{}	172.18.0.1	2026-08-11 00:51:44.232196+00
562eed97-d53d-4b5f-9cdb-da80551798d2	13ab41a8-0a50-401c-a095-23203a8e41be	c060a338-c537-4b82-9afc-1df645e1e5cf	TLC	LECTURER_UNASSIGNED	lecturer_assignments	8394b163-ab60-48a5-bcb1-aa2c787aa026	{}	172.18.0.1	2026-08-11 00:51:44.790912+00
7b911850-3666-4cdd-b90d-3702ff33895a	13ab41a8-0a50-401c-a095-23203a8e41be	bea9a0eb-44f3-47e2-a535-9461e136a98e	TLC	LECTURER_ASSIGNED	lecturer_assignments	342efce1-53c2-4dff-9ed8-b4024be0c418	{}	172.18.0.1	2026-08-11 09:34:54.365608+00
c8117894-0f05-4836-a9ed-b8fff6727a31	13ab41a8-0a50-401c-a095-23203a8e41be	bea9a0eb-44f3-47e2-a535-9461e136a98e	TLC	LECTURER_UNASSIGNED	lecturer_assignments	342efce1-53c2-4dff-9ed8-b4024be0c418	{}	172.18.0.1	2026-08-11 09:34:55.008764+00
\.


--
-- Data for Name: app_notifications; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.app_notifications (notification_id, tenant_id, sender_id, sender_name, sender_role, audience, unit_id, subject, body, created_at, action, action_ref) FROM stdin;
d658c914-45c1-4fa8-be21-9369191cf82e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 23:17:29.591074+00	\N	\N
eb427ce4-c6a1-4d3d-955d-795448c04a8b	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Structures — Thu 6 Aug 2026 at 14:00	You were recorded as NOT TAUGHT for Data Structures (BCS1201), Thu 6 Aug 2026 at 14:00, in LR3.	2026-08-06 12:28:09.472392+00	\N	\N
2900fd9e-793a-4cc1-89dc-c6c4ff99fa0c	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Operating Systems — Thu 6 Aug 2026 at 16:00	You were recorded as TAUGHT for Operating Systems (BCS2201), Thu 6 Aug 2026 at 16:00, in LR3.	2026-08-06 12:28:09.484908+00	\N	\N
a248e564-48f1-4223-bba9-b3ba9d91ffca	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	NPTEST-UNIT-2	Lecture moved: Operating Systems (BCS2201) — Thu 6 Aug 2026 at 16:00	QA patrol found this lecture away from its published slot (Thu 6 Aug 2026 at 16:00), LR3 — room: LR3 → LR7; time: 16:00 → 16:30.\n\nThis change was not recorded on the timetable. If it is permanent, ask the Teaching & Learning Centre to update the published week; students are still being sent to the timetabled room.\n\nNote recorded on patrol: Projector failed in LR3	2026-08-06 12:28:09.491153+00	\N	\N
0f8fde10-5b12-43a7-a185-64f0da25bafc	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Structures — Thu 6 Aug 2026 at 14:00	You were recorded as NOT TAUGHT for Data Structures (BCS1201), Thu 6 Aug 2026 at 14:00, in LR3.	2026-08-06 12:31:08.597302+00	\N	\N
b7468c85-c0da-4202-b4f2-f19fb829304d	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Operating Systems — Thu 6 Aug 2026 at 16:00	You were recorded as TAUGHT for Operating Systems (BCS2201), Thu 6 Aug 2026 at 16:00, in LR3.	2026-08-06 12:31:08.622506+00	\N	\N
de118a76-eb40-43cf-9ec1-7631ea41e4ef	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	NPTEST-UNIT-2	Lecture moved: Operating Systems (BCS2201) — Thu 6 Aug 2026 at 16:00	QA patrol found this lecture away from its published slot (Thu 6 Aug 2026 at 16:00, LR3) — room: LR3 → LR7; time: 16:00 → 16:30.\n\nThis change was not recorded on the timetable. If it is permanent, ask the Teaching & Learning Centre to update the published week; students are still being sent to the timetabled room.\n\nNote recorded on patrol: Projector failed in LR3	2026-08-06 12:31:08.679246+00	\N	\N
50af5396-34ce-4fff-9b26-a08fe6343814	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Structures — Thu 6 Aug 2026 at 14:00	You were recorded as NOT TAUGHT for Data Structures (BCS1201), Thu 6 Aug 2026 at 14:00, in LR3.	2026-08-06 12:31:29.25992+00	\N	\N
400c57d4-8b58-4030-9c0a-e19b85d976b4	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Operating Systems — Thu 6 Aug 2026 at 16:00	You were recorded as TAUGHT for Operating Systems (BCS2201), Thu 6 Aug 2026 at 16:00, in LR3.	2026-08-06 12:31:29.267567+00	\N	\N
07f93359-e875-4ee2-ab4d-e706db71960e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	NPTEST-UNIT-2	Lecture moved: Operating Systems (BCS2201) — Thu 6 Aug 2026 at 16:00	QA patrol found this lecture away from its published slot (Thu 6 Aug 2026 at 16:00, LR3) — room: LR3 → LR7; time: 16:00 → 16:30.\n\nThis change was not recorded on the timetable. If it is permanent, ask the Teaching & Learning Centre to update the published week; students are still being sent to the timetabled room.\n\nNote recorded on patrol: Projector failed in LR3	2026-08-06 12:31:29.27506+00	\N	\N
0270a64a-1e16-4431-879d-d2f676803adc	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Structures — Thu 6 Aug 2026 at 14:00	You were recorded as NOT TAUGHT for Data Structures (BCS1201), Thu 6 Aug 2026 at 14:00, in LR3.	2026-08-06 13:20:33.824599+00	\N	\N
2b4a1792-ddbc-420b-8d62-ab4ac092aa60	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Operating Systems — Thu 6 Aug 2026 at 16:00	You were recorded as TAUGHT for Operating Systems (BCS2201), Thu 6 Aug 2026 at 16:00, in LR3.	2026-08-06 13:20:33.834222+00	\N	\N
4057ed44-48e3-4f62-add6-b8b2428e2e96	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	NPTEST-UNIT-2	Lecture moved: Operating Systems (BCS2201) — Thu 6 Aug 2026 at 16:00	QA patrol found this lecture away from its published slot (Thu 6 Aug 2026 at 16:00, LR3) — room: LR3 → LR7; time: 16:00 → 16:30.\n\nThis change was not recorded on the timetable. If it is permanent, ask the Teaching & Learning Centre to update the published week; students are still being sent to the timetabled room.\n\nNote recorded on patrol: Projector failed in LR3	2026-08-06 13:20:33.847927+00	\N	\N
a4d69b1e-921b-4407-9995-58549a046c05	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Structures — Thu 6 Aug 2026 at 14:00	You were recorded as NOT TAUGHT for Data Structures (BCS1201), Thu 6 Aug 2026 at 14:00, in LR3.	2026-08-06 16:15:00.652668+00	\N	\N
a8654198-852a-4f09-8b3a-643947cbd75b	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Operating Systems — Thu 6 Aug 2026 at 16:00	You were recorded as TAUGHT for Operating Systems (BCS2201), Thu 6 Aug 2026 at 16:00, in LR3.	2026-08-06 16:15:00.661988+00	\N	\N
4af36035-e210-4ebf-9a5f-7018c70bc228	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	NPTEST-UNIT-2	Lecture moved: Operating Systems (BCS2201) — Thu 6 Aug 2026 at 16:00	QA patrol found this lecture away from its published slot (Thu 6 Aug 2026 at 16:00, LR3) — room: LR3 → LR7; time: 16:00 → 16:30.\n\nThis change was not recorded on the timetable. If it is permanent, ask the Teaching & Learning Centre to update the published week; students are still being sent to the timetabled room.\n\nNote recorded on patrol: Projector failed in LR3	2026-08-06 16:15:00.681186+00	\N	\N
9c4214e3-1e95-4af3-aa77-9551fdb342cc	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QAAT	SYSTEM	DIRECT	CSE 2420	Starts in 10 minutes: English Language skills	English Language skills (SE) begins at 08:00 in B09.	2026-08-08 04:52:29.991898+00	\N	\N
fc8b3eb1-44ff-4a6d-bfc4-2965c36f318f	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QAAT	SYSTEM	DIRECT	CSE 2420	Attendance not recorded: English Language skills at 08:00	Your English Language skills lecture (SE) was due to start at 08:00 in B09, and neither the coordinator nor a QA patroller has recorded it.\n\nIf you are teaching, ask the coordinator to open the session.	2026-08-08 05:11:30.04481+00	\N	\N
b2b0da49-619f-457d-bb59-76783b162c0f	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QAAT	SYSTEM	DIRECT	CSE 2420	Please visit the QA office: English Language skills at 08:00	Your English Language skills lecture (SE) at 08:00 in B09 has no attendance record — neither the coordinator nor a QA patroller captured it.\n\nIf you taught this lecture, please go to the Quality Assurance office within 20 minutes so the record can be corrected. Left as it is, this counts as a lecture that did not happen.	2026-08-08 05:21:29.993912+00	\N	\N
739bdd74-6ae1-40e9-89f8-6d7cf645f737	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 06:56:35.790898+00	\N	\N
4ea86d73-3edf-4bdf-bbf8-692ca8465e0b	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 06:56:35.812632+00	\N	\N
f3b4014d-9da9-4994-8505-5458c236cabd	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 06:57:21.428299+00	\N	\N
e19fdb3b-642b-4217-8eff-b3e7ce89e981	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 06:57:21.444997+00	\N	\N
8d137fe7-f99c-4808-ac90-e7d702353af7	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.520537+00	\N	\N
ddff6144-d248-4672-bcc7-eaad60c68cce	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.527421+00	\N	\N
f998e12b-5ab2-49ac-8a8d-228c039d332d	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.531984+00	\N	\N
27b59a57-0b25-4a3e-8fde-bb0f0cf9c3df	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.551212+00	\N	\N
9338d4ce-9b0c-45bc-9cd4-873bb8661314	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.558203+00	\N	\N
bf0a1369-9ef7-4212-accf-1a5b918377b0	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.573691+00	\N	\N
eabb559c-264e-43c9-a8a6-4e2650bdc214	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.575888+00	\N	\N
c72edcbe-f491-45dc-a2b8-e74560ebb908	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.595889+00	\N	\N
a97f02e6-9e7b-4631-b993-c1a44dc35d1b	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.603777+00	\N	\N
ad36cee9-4e9c-40e7-9526-a03f34fb1526	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.609159+00	\N	\N
5956c279-2bc5-4ba6-aeb6-be7de30a1eca	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.611242+00	\N	\N
6aa79c82-3610-4d00-941e-a90300fdcb3a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.614075+00	\N	\N
01176eac-940e-4dca-9cde-c8dc86c3c58e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.618244+00	\N	\N
dcda57cf-471a-4d92-b7b5-7b50415cc4e6	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.621224+00	\N	\N
09e6323e-967a-4226-922c-9c57ee0fd822	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.632761+00	\N	\N
35f1601e-f604-45ef-b544-9b6e298a19cf	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.63319+00	\N	\N
6b6dd0f3-8d96-47bf-b9b8-c353d4f745e6	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.633745+00	\N	\N
99c12bb2-38bc-4bf8-bc9c-5dd42840c786	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.637086+00	\N	\N
53f89896-309a-4d2a-a97d-c9f0ca343280	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.642348+00	\N	\N
7201f543-cbab-4672-baf2-2849a6d723b0	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:15:12.649483+00	\N	\N
943bb7dd-0ba2-4330-90d2-28362dbfa5ed	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:57.962625+00	\N	\N
019be5eb-ab9e-4f9e-ad1d-8acc59091408	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:57.970499+00	\N	\N
569df5fe-a867-477d-b3c2-c56fed1162ba	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:57.978977+00	\N	\N
6195df60-d391-4887-8629-8dc2a2beb0c5	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:57.992277+00	\N	\N
0d75cfac-46bd-4d7c-9204-413957f8e063	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.003297+00	\N	\N
224436fe-60d0-4c97-8f6e-55685b47eef8	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.010954+00	\N	\N
d93d7c98-2b65-4c22-8d45-7f91abfed642	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.014104+00	\N	\N
d82cab62-edde-4b74-b623-0de1f35470ce	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.017792+00	\N	\N
190b3b65-410d-449f-821b-4e6da4a623ac	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.043798+00	\N	\N
5b16355f-566c-41b1-8ee7-744dfbbcba91	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.064338+00	\N	\N
c057b15b-651c-4dc9-bd5b-2e69bee50907	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.084955+00	\N	\N
1e6c9eb0-594a-4d75-a064-f6c8be621c9c	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.028307+00	\N	\N
02ac1900-bfe1-4acf-9864-3e2a92664a25	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.05723+00	\N	\N
969daf21-7498-44a4-8731-180ea22de4f3	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.074943+00	\N	\N
66e4d699-5764-4bdf-a4d9-29cf4e4ce4c7	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-11 00:51:14.250688+00	\N	\N
ff2b1245-d479-4027-8cce-92100d67cf7c	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-11 09:34:09.142455+00	\N	\N
64dc3268-4d2d-4abc-a0f9-dd1cc2bbdfbc	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.043367+00	\N	\N
ca74ac76-c57b-48d4-a121-f915125889bb	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.063288+00	\N	\N
8f52e1ec-4b0e-47ae-917e-a25a2ba07dfe	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.090605+00	\N	\N
833a85c6-a959-4b1d-b626-bb23a2c4e025	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Sat 8 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Sat 8 Aug 2026 at 10:00, in LR-COLT.	2026-08-08 13:54:59.144613+00	\N	\N
365422fd-fad0-4411-ae60-b91da763c4fa	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Mon 10 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Mon 10 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 21:34:13.744017+00	\N	\N
fc2350a0-ca43-4e8c-9dc5-b0a2b32562a5	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 21:38:44.332463+00	\N	\N
efa5f643-b133-45a0-b487-83e76fa2d507	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 07:51:42.622132+00	\N	\N
398efd7f-020c-43bc-96a5-25b0e18f7586	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 07:51:42.639574+00	\N	\N
ba68bfe1-5d4f-46b3-8590-86f4520361d2	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.927563+00	\N	\N
7ef9346c-8ff5-4949-8285-c72722cbe5f7	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.954621+00	\N	\N
e3ed2126-6e5c-4190-938a-f468f15f9f58	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.973049+00	\N	\N
41884603-d937-4162-82d6-6d3e40ee2310	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.01068+00	\N	\N
8b0baf69-95d2-4e1c-a2d7-1a458b05419a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.026215+00	\N	\N
40c62b3a-9315-42df-81fc-1191e5235682	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.04659+00	\N	\N
900c4644-4adc-4c3a-a5a5-9fdccfe0060e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Sat 8 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Sat 8 Aug 2026 at 10:00, in LR-COLT.	2026-08-08 13:57:49.320616+00	\N	\N
c400fe10-8bfc-4b6c-bf9c-27be9e49782e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Sat 8 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Sat 8 Aug 2026 at 10:00, in LR-COLT.	2026-08-08 13:58:03.890647+00	\N	\N
32d9d2a8-a9a1-4e32-b2e0-a2f64974d188	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.913294+00	\N	\N
8a0ad404-8076-4ff4-8fb8-9903764788db	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.951999+00	\N	\N
a92ed287-97f4-4b3b-92c3-24a1939a079e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.968759+00	\N	\N
bcbe4aa3-19bb-4bbc-a887-37b21d1981d6	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.006466+00	\N	\N
b60a869a-4262-4a01-8833-cd1e58ed5700	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.019114+00	\N	\N
278b7454-121a-4a8c-9759-7103cfc13b6d	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.035099+00	\N	\N
5b6916c4-8e09-4d03-a4a8-d24307406e30	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.056618+00	\N	\N
49e4b7d3-d8df-4bad-86a8-2fbe81217378	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Sat 8 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Sat 8 Aug 2026 at 10:00, in LR-COLT.	2026-08-08 14:02:35.37254+00	\N	\N
4ac46d65-6e63-4c3e-be03-0fc5ea44c6a7	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Sat 8 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Sat 8 Aug 2026 at 10:00, in LR-COLT.	2026-08-08 14:03:01.984794+00	\N	\N
73bde585-c8bb-4eed-9775-b9f75aabd859	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Mon 10 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Mon 10 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 19:23:41.791434+00	\N	\N
55940d61-9554-4e0d-b60d-1631d0a6f5fc	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 21:38:01.091013+00	\N	\N
b62f7608-1b0a-4a28-92b1-9bdbf5f01255	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 21:51:41.511782+00	\N	\N
8d4d358f-35a7-4c3d-9554-28a65b13181e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.964416+00	\N	\N
8f150a95-c72c-4193-b045-bbefa9813508	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.016105+00	\N	\N
b1dcdc18-aaa6-4b2c-9946-4a6f69625d11	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.039899+00	\N	\N
15402f0d-f191-47c8-8658-e8cf4edd8283	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 23:22:25.982252+00	\N	\N
1be9c75b-b7be-4ac9-812e-4c1259ab4e81	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:23.968465+00	\N	\N
0b3a19f8-e024-40e3-8aaa-fd17f2004090	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.014349+00	\N	\N
b2fd6eca-9006-4918-85f6-025e6bba23e0	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.029993+00	\N	\N
65c69d1d-ec0d-44c1-8a8c-0703a89e0a0a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:54:24.051076+00	\N	\N
0169195a-2e59-4b53-b522-8d9d595e48ef	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Sat 8 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Sat 8 Aug 2026 at 10:00, in LR-COLT.	2026-08-08 15:12:09.337762+00	\N	\N
3266f507-6c47-42a5-aaea-725bcd8f8e63	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Mon 10 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Mon 10 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 19:50:05.932344+00	\N	\N
bd980259-09eb-4b6c-81ab-e71d50651e9f	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 11:41:15.388929+00	\N	\N
d9602da6-481a-4959-9087-a8084dd3370a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Data Science — Sat 8 Aug 2026 at 17:00	You were recorded as TAUGHT for Data Science (SE), Sat 8 Aug 2026 at 17:00, in LR1.	2026-08-08 11:41:15.426125+00	\N	\N
a474b74c-39ed-4226-84dd-9f378fd225ce	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 23:53:22.770738+00	\N	\N
81d3a06a-5418-44b7-82e6-5b80aa157860	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.819307+00	\N	\N
9f4c4dc0-1102-4328-bcd2-f60f48403f85	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.909262+00	\N	\N
2d31be1c-3a53-47a5-a50a-ec923d178f6f	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.94541+00	\N	\N
d5042c93-5cd8-4e5e-a95e-25690e50ceac	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.977959+00	\N	\N
34fe7fdb-e66d-4bdf-9809-e002fed96aa9	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:44.014681+00	\N	\N
1e7d0c28-8aa0-4fb6-a44c-2f18ae45f087	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:44.044067+00	\N	\N
3439e300-8245-455b-a8cc-08f070bb3a6c	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.293576+00	\N	\N
2624a3cc-e3ad-4f38-91a7-0515d89067cb	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.315984+00	\N	\N
de17892f-0713-4477-8d38-5294d799e19c	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.368925+00	\N	\N
77de9983-e50b-4a79-b3da-4f204c7e4f42	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Mon 10 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Mon 10 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 20:26:43.78472+00	\N	\N
be36dc86-d074-4aef-8b34-c53ea8cdf32d	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-11 00:17:31.233192+00	\N	\N
f45708c0-d923-4663-ae21-312f7b38ccd6	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-11 00:24:49.375642+00	\N	\N
3a019d24-5377-4413-89a3-dcaae8f3e7b9	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.846713+00	\N	\N
fb90d8d5-927e-4655-bfdc-86dfaba8e258	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.944804+00	\N	\N
e316335e-ca1a-4c6d-8b3b-536b3451f872	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.999387+00	\N	\N
b7310bcd-8a5b-4d31-a82c-b3aaa4750a53	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:44.025491+00	\N	\N
870c49d5-1866-41a0-8d3e-3f2d310f3796	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:44.055544+00	\N	\N
3dfd3180-2399-4519-9498-f2ec938c4799	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.229232+00	\N	\N
73190a32-ae44-47f4-9069-572a01b10d43	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.256097+00	\N	\N
f0da856e-d3bd-4972-bcec-8854e5421aaa	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.272771+00	\N	\N
1f27b7a7-f04c-4732-8d6a-0c36a409c03e	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.30731+00	\N	\N
55067cb7-8d5c-458c-aac1-597699210b6a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.324347+00	\N	\N
4a46056a-c467-4b71-b9e4-451c1d925a70	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.337317+00	\N	\N
2ad986b3-8d5c-4d2b-8440-eb0e0367a0a2	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.368384+00	\N	\N
9e627a23-bd4e-4cc0-9636-8e550dba847a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.869522+00	\N	\N
d0a1f9d8-7305-4815-a15f-6d9e14e23bc4	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.949385+00	\N	\N
89d1fb8c-06fb-44da-9b78-63329f6cd01a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.96313+00	\N	\N
cc06b0e1-70f5-40e0-8faa-d4fdc42e685f	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:44.020838+00	\N	\N
e2adbbf7-3aa9-4a2f-9422-1435f7b9ab61	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:44.056368+00	\N	\N
02d29910-3971-41c1-80e0-92ceefa67b1f	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.250673+00	\N	\N
6b472e83-7bef-4a53-8996-389e6d2ae00c	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.312209+00	\N	\N
bb8d7798-dde2-43d1-8ef4-fc3187bacfcd	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.372692+00	\N	\N
009b62c4-6c13-4d9b-a664-5ef3aa03ef36	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Column Test Unit — Mon 10 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Mon 10 Aug 2026 at 10:00, in LR-COLT.	2026-08-10 20:41:16.973934+00	\N	\N
7037b250-bbde-436b-93de-c54cdbacda24	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.878919+00	\N	\N
eb8b4f0d-e085-47ef-ab52-651c093bae93	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.953899+00	\N	\N
eb59b3cf-a497-4ff9-979e-92ab650acf74	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:43.983004+00	\N	\N
d23a8d22-4360-4878-a1a7-31c53d5b29ac	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 11:59:44.036199+00	\N	\N
34cd3e14-8e03-478a-a513-9411692966a9	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.173772+00	\N	\N
fc25d0b3-a7b1-4823-88cb-7a1610aa03df	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.198467+00	\N	\N
25c110e5-f9f6-4085-8d1d-eb738c0f341a	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.220252+00	\N	\N
817ae822-f53b-477c-9d68-d4b59b34c399	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.243049+00	\N	\N
47374721-0e04-4fc6-bbcd-a9a7ed8cb3ac	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.29909+00	\N	\N
6439e751-6e2d-4fc2-80ad-1b18f66d5761	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.321477+00	\N	\N
ef2f682d-87ab-4d93-ace5-9ba1d29fedfb	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Structured Programming — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Structured Programming (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 12:00:07.362324+00	\N	\N
d1446d0c-e48e-41e2-9a0d-5a5cac34d042	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Monitor	QA_PATROLLER	DIRECT	\N	Monitor: Column Test Unit — Tue 11 Aug 2026 at 10:00	You were recorded as TAUGHT for Column Test Unit (COLT-COURSE), Tue 11 Aug 2026 at 10:00, in LR-COLT.	2026-08-11 00:44:57.763546+00	\N	\N
31b65a93-4495-4777-9145-b806bd9578b5	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.032518+00	\N	\N
2f912efb-4a20-42c9-abbc-f82a601535e8	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.074629+00	\N	\N
6272394f-ae49-49de-8636-f1609c40b622	13ab41a8-0a50-401c-a095-23203a8e41be	\N	QA Patrol	QA_PATROLLER	DIRECT	\N	Patrol: Load Test Unit 3 — Sat 8 Aug 2026 at 09:00	You were recorded as TAUGHT for Load Test Unit 3 (LOAD-COURSE), Sat 8 Aug 2026 at 09:00, in LR9.	2026-08-08 07:20:58.091877+00	\N	\N
\.


--
-- Data for Name: attendance_logs; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.attendance_logs (log_id, tenant_id, session_id, student_id, checkin_timestamp, device_fingerprint_hash, sequence_number, entry_method, override_officer_id, override_reason, audit_flags, coordinator_id) FROM stdin;
b76a1e86-a11b-483b-af58-3ebe779d00ce	13ab41a8-0a50-401c-a095-23203a8e41be	ef8080c5-d6c9-404b-84c5-e3c317cbce42	2025-08-40174	2026-08-08 11:41:13.096502+00	dev-A	1	AUTHENTICATED	\N	\N	{}	64ae5387-b705-45e5-b6ea-201cab10c1f8
8f413506-d8b1-4620-827b-ea670ec5d897	13ab41a8-0a50-401c-a095-23203a8e41be	b33cbe4a-8ce7-4729-a136-4c58d22ba84d	2025-08-40174	2026-08-08 06:51:48.666763+00	dev-A	1	AUTHENTICATED	\N	\N	{}	64ae5387-b705-45e5-b6ea-201cab10c1f8
b8356769-f4e1-4af0-b269-f29b4ff9b6e0	13ab41a8-0a50-401c-a095-23203a8e41be	1e0d76b4-50db-4251-95a0-c30125da017f	2025-08-40174	2026-08-08 07:05:21.399846+00	dev-A	1	AUTHENTICATED	\N	\N	{}	64ae5387-b705-45e5-b6ea-201cab10c1f8
18df75f7-c4e2-48af-9c96-d1fcc96a5ceb	13ab41a8-0a50-401c-a095-23203a8e41be	f7ef7c43-ef0d-465d-892e-789a4c1367f6	2025-08-40174	2026-08-08 07:06:03.878095+00	dev-A	1	AUTHENTICATED	\N	\N	{}	64ae5387-b705-45e5-b6ea-201cab10c1f8
5544346d-3d86-43ca-acdf-1559b34a7916	13ab41a8-0a50-401c-a095-23203a8e41be	729c9c46-3ef2-4428-a792-b73be7b0f16a	2025-08-40174	2026-08-08 07:51:37.750902+00	dev-A	1	AUTHENTICATED	\N	\N	{}	64ae5387-b705-45e5-b6ea-201cab10c1f8
\.


--
-- Data for Name: coordinator_delegations; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.coordinator_delegations (delegation_id, tenant_id, offering_id, coordinator_id, deputy_student_id, deputy_name, code, expires_at, revoked, created_at, last_used_at) FROM stdin;
35529a7c-c51a-4095-a260-490c7bfb29e6	13ab41a8-0a50-401c-a095-23203a8e41be	ab287ed5-4785-4ca7-b689-288b5e60d7c7	64ae5387-b705-45e5-b6ea-201cab10c1f8	2025-08-40174	NYAKWERA WINNIE	LS74NNQJ	2026-06-28 20:59:59+00	t	2026-06-28 04:39:54.401498+00	2026-06-28 04:40:21.290734+00
b27dddb8-e43b-43c6-aaf4-47c391e5d093	13ab41a8-0a50-401c-a095-23203a8e41be	ab287ed5-4785-4ca7-b689-288b5e60d7c7	64ae5387-b705-45e5-b6ea-201cab10c1f8	2025-08-40174	NYAKWERA WINNIE	H2Y5TWF9	2026-07-03 20:59:59+00	t	2026-07-03 19:07:58.584273+00	\N
\.


--
-- Data for Name: course_offerings; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.course_offerings (offering_id, tenant_id, course_id, session_type, coordinator_id, created_at, study_year, semester, level, intake, delivery_mode) FROM stdin;
ab287ed5-4785-4ca7-b689-288b5e60d7c7	13ab41a8-0a50-401c-a095-23203a8e41be	SE	Day	64ae5387-b705-45e5-b6ea-201cab10c1f8	2026-06-23 13:52:34.003114+00	2	2	Degree	August Intake	IN_PERSON
f270a5bf-76b3-4188-8a44-e9fa6c9463df	13ab41a8-0a50-401c-a095-23203a8e41be	SE	Weekend	13b6a47b-ebbb-4315-bb3e-6a5c8377cbab	2026-06-28 23:28:10.520814+00	2	2	Degree	August Intake	IN_PERSON
8c01906b-0aba-4ff2-9fdf-18a9708e1c33	13ab41a8-0a50-401c-a095-23203a8e41be	IT	Day	\N	2026-08-01 17:13:06.06404+00	1	1			IN_PERSON
\.


--
-- Data for Name: course_units; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.course_units (unit_id, tenant_id, course_id, name, year, semester, academic_year, default_venue_id, session_start, session_duration_minutes, schedule_locked, level) FROM stdin;
CSE 2110	13ab41a8-0a50-401c-a095-23203a8e41be	SE	Data Science	1	1	\N	\N	\N	\N	f	Degree
CSE 2120	13ab41a8-0a50-401c-a095-23203a8e41be	SE	Structured Programming	2	2	\N	\N	\N	\N	f	Degree
CSE 3394	13ab41a8-0a50-401c-a095-23203a8e41be	SE	Numerical Mathematics	1	1	\N	\N	\N	\N	f	Degree
CBE 2420	13ab41a8-0a50-401c-a095-23203a8e41be	IT	English Language skills	2	2	\N	\N	\N	\N	f	Degree
CBE 24520	13ab41a8-0a50-401c-a095-23203a8e41be	IT	English Language skills	2	2	\N	\N	\N	\N	f	Degree
CSE 21910	13ab41a8-0a50-401c-a095-23203a8e41be	IT	Data Science	1	1	\N	\N	\N	\N	f	Degree
CSE 36394	13ab41a8-0a50-401c-a095-23203a8e41be	IT	Numerical Mathematics	1	1	\N	\N	\N	\N	f	Degree
CSE 25420	13ab41a8-0a50-401c-a095-23203a8e41be	IT	English Language skills	2	2	\N	\N	\N	\N	f	Degree
DBE 32434	13ab41a8-0a50-401c-a095-23203a8e41be	IT	Operating Systems	2	2	\N	\N	\N	\N	f	Degree
CSE 21820	13ab41a8-0a50-401c-a095-23203a8e41be	IT	Structured Programming	2	2	\N	\N	\N	\N	f	Degree
CSE 2420	13ab41a8-0a50-401c-a095-23203a8e41be	SE	English Language skills	2	2	\N	B01	\N	\N	f	Degree
DBE 3234	13ab41a8-0a50-401c-a095-23203a8e41be	SE	Operating Systems	2	2	\N	C04	\N	\N	f	Degree
\.


--
-- Data for Name: courses; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.courses (course_id, tenant_id, name, coordinator_id, department, school, total_years, level, course_group, level_years, school_id, department_id) FROM stdin;
SE	13ab41a8-0a50-401c-a095-23203a8e41be	Software Engineering	\N	Computer Science	SOMAC	3	Degree	Software Engineering	{"PhD": 4, "Degree": 4, "Certificate": 2}	c78d76db-f936-4396-979f-483ac1202fb1	8c87ce07-fcab-4db8-9399-f28bd875f34b
IT	13ab41a8-0a50-401c-a095-23203a8e41be	Information Technology	\N	Computer Science	SOMAC	3	Degree	Information Technology	{}	c78d76db-f936-4396-979f-483ac1202fb1	8c87ce07-fcab-4db8-9399-f28bd875f34b
\.


--
-- Data for Name: departments; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.departments (department_id, tenant_id, school_id, name, kind, created_at) FROM stdin;
8c87ce07-fcab-4db8-9399-f28bd875f34b	13ab41a8-0a50-401c-a095-23203a8e41be	c78d76db-f936-4396-979f-483ac1202fb1	Computer Science	ACADEMIC	2026-08-01 17:13:06.806189+00
fd74358b-40cf-43fa-b20c-b522a38a3bc1	13ab41a8-0a50-401c-a095-23203a8e41be	\N	Finance	SUPPORT	2026-08-02 05:14:57.102935+00
fea5e8a0-ca05-4c2b-9dba-1b22cc7cd1e4	13ab41a8-0a50-401c-a095-23203a8e41be	\N	Admissions	SUPPORT	2026-08-02 05:14:57.102935+00
1c489f38-97ba-48e3-a75b-5442637c3d72	13ab41a8-0a50-401c-a095-23203a8e41be	\N	Bursary	SUPPORT	2026-08-02 05:14:57.102935+00
472d2374-c253-43e7-9239-ad765c047ade	13ab41a8-0a50-401c-a095-23203a8e41be	\N	Library	SUPPORT	2026-08-02 05:14:57.102935+00
8b982744-dcc9-4b67-90f9-fed09c56b1af	13ab41a8-0a50-401c-a095-23203a8e41be	\N	ICT	SUPPORT	2026-08-02 05:14:57.102935+00
\.


--
-- Data for Name: employee_attendance_days; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.employee_attendance_days (day_id, tenant_id, emp_no, ac_no, seq_no, full_name, auto_assign, department, work_date, timetable, on_duty, off_duty, clock_in, clock_out, normal, real_time, late, early, absent, ot_time, work_time, exception, must_cin, must_cout, ndays, weekend, holiday, att_time, ndays_ot, weekend_ot, holiday_ot, checked_in_late, checked_out_early, source_file, imported_at) FROM stdin;
\.


--
-- Data for Name: employee_attendance_logs; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.employee_attendance_logs (log_id, tenant_id, staff_id, event_time, event_type, source, device_id, comment, created_at) FROM stdin;
\.


--
-- Data for Name: employees; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.employees (employee_pk, tenant_id, staff_id, title, full_name, department, job_title, email, phone, is_active, created_at) FROM stdin;
f2bb97be-2107-4922-a970-072f89a745f0	13ab41a8-0a50-401c-a095-23203a8e41be	KIU-2232	\N	SEMUCYO JOSHUA	\N	\N	\N	\N	t	2026-07-23 11:47:30.354041+00
\.


--
-- Data for Name: hardware_vault; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.hardware_vault (student_id, tenant_id, fingerprint_hash, first_bound_at, last_verified_at, academic_year) FROM stdin;
\.


--
-- Data for Name: lecturer_assignments; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturer_assignments (assignment_id, tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session, created_at) FROM stdin;
1566e392-d41c-42af-9eea-59c58191072c	13ab41a8-0a50-401c-a095-23203a8e41be	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	CSE 2110	SE	2025/2026	1	1	Day	2026-06-23 13:57:00.40478+00
a980a98d-9b02-4a41-9966-25c81eb5d8a9	13ab41a8-0a50-401c-a095-23203a8e41be	ec27f1c8-5e10-4792-8bf1-dfd3c1cc4b4a	CBE 2420	IT	2025/2026	2	2	Weekend	2026-06-29 03:49:38.60922+00
2ff021ca-da00-4ed3-b697-156e098e65bf	13ab41a8-0a50-401c-a095-23203a8e41be	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	CSE 2420	SE	2025/2026	2	2	Day	2026-06-29 08:46:32.315485+00
9784cfa8-0c4e-42b1-bf46-7df5ddac8824	13ab41a8-0a50-401c-a095-23203a8e41be	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	CSE 2420	SE	2025/2026	2	2	Weekend	2026-07-05 04:30:22.31818+00
\.


--
-- Data for Name: lecturer_attendance_logs; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturer_attendance_logs (log_id, tenant_id, session_id, lecturer_id, gate_open_time, gate_close_time, contact_hours, unit_id, venue_id, session_date, lecturer_scanned_at, lecturer_fingerprint_hash, lecturer_ended_at, lecturer_end_fingerprint_hash) FROM stdin;
e3aaebb0-f128-430d-ba63-645910fa5a17	13ab41a8-0a50-401c-a095-23203a8e41be	f7ef7c43-ef0d-465d-892e-789a4c1367f6	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-08-08 07:06:03.599048+00	2026-08-08 07:06:03.995365+00	0.00	CSE 2110	\N	2026-08-08	2026-08-08 07:06:03.851796+00	fp-test-1	2026-08-08 07:06:03.981755+00	fp-test-1
8168b180-9442-4d5c-beb6-8809d1fad7f9	13ab41a8-0a50-401c-a095-23203a8e41be	e67b7bbb-ddb4-460a-b0a6-be3037c2af88	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-07-03 21:20:43.460672+00	2026-07-03 21:21:22.200036+00	0.01	CSE 2420	\N	2026-07-03	\N	\N	\N	\N
b7648939-bd5b-4690-80a8-b2be94cbe650	13ab41a8-0a50-401c-a095-23203a8e41be	2c915be0-54ea-4e42-9dbd-b27092c45c12	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-07-06 05:03:56.960594+00	2026-07-06 05:04:09.162978+00	0.00	CSE 2110	\N	2026-07-06	\N	\N	\N	\N
6b020772-5027-4815-8dac-ca4f897fc552	13ab41a8-0a50-401c-a095-23203a8e41be	ef8080c5-d6c9-404b-84c5-e3c317cbce42	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-08-08 11:41:12.700151+00	2026-08-08 11:41:13.195434+00	0.00	CSE 2110	\N	2026-08-08	2026-08-08 11:41:13.063653+00	fp-test-1	2026-08-08 11:41:13.180069+00	fp-test-1
21ced053-41ef-4916-9b36-2104586e9abb	13ab41a8-0a50-401c-a095-23203a8e41be	2b13c145-0929-4912-a818-03d74589d886	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-07-06 07:14:34.438272+00	2026-07-06 07:14:35.964395+00	0.00	CSE 2110	\N	2026-07-06	2026-07-06 07:14:34.977966+00	\N	\N	\N
1609aa6e-e682-4125-bb3b-ff0d7f2f85a6	13ab41a8-0a50-401c-a095-23203a8e41be	037bba66-8f7a-4df8-8561-c4d1e2de9374	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-08-08 06:50:05.403964+00	2026-08-08 06:50:05.681824+00	0.00	CSE 2110	\N	2026-08-08	\N	\N	\N	\N
81b6abea-fc69-4cba-9f83-cf57fc1d68e1	13ab41a8-0a50-401c-a095-23203a8e41be	880ef56b-ae99-4e36-83fa-c1757682f1a2	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-08-08 12:00:06.943087+00	2026-08-08 12:00:07.119278+00	0.00	CSE 2110	\N	2026-08-08	\N	\N	\N	\N
214df7f8-41ef-4dad-b504-50a88fd5eb6d	13ab41a8-0a50-401c-a095-23203a8e41be	b33cbe4a-8ce7-4729-a136-4c58d22ba84d	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-08-08 06:51:48.375604+00	2026-08-08 06:51:48.734184+00	0.00	CSE 2110	\N	2026-08-08	2026-08-08 06:51:48.635891+00	fp-test-1	2026-08-08 06:51:48.721374+00	fp-test-1
3ef5abf1-4c50-405c-a754-8502d2f340a2	13ab41a8-0a50-401c-a095-23203a8e41be	1e0d76b4-50db-4251-95a0-c30125da017f	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-08-08 07:05:21.10752+00	2026-08-08 07:05:21.466974+00	0.00	CSE 2110	\N	2026-08-08	2026-08-08 07:05:21.374348+00	fp-test-1	2026-08-08 07:05:21.456364+00	fp-test-1
424e6929-cade-40b0-8f2a-c5c8b715b723	13ab41a8-0a50-401c-a095-23203a8e41be	729c9c46-3ef2-4428-a792-b73be7b0f16a	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-08-08 07:51:37.26628+00	2026-08-08 07:51:37.954188+00	0.00	CSE 2110	\N	2026-08-08	2026-08-08 07:51:37.697338+00	fp-test-1	2026-08-08 07:51:37.929891+00	fp-test-1
\.


--
-- Data for Name: lecturer_biometric_templates; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturer_biometric_templates (template_id, tenant_id, lecturer_id, template, template_format, finger_position, reader_model, enrolled_by, created_at) FROM stdin;
\.


--
-- Data for Name: lecturer_daily_codes; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturer_daily_codes (tenant_id, lecturer_id, code, valid_date, created_at) FROM stdin;
13ab41a8-0a50-401c-a095-23203a8e41be	lec:KIU/STAFF/2332	0563	2026-08-05	2026-08-05 20:43:06.86014+00
\.


--
-- Data for Name: lecturer_patrol_logs; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturer_patrol_logs (patrol_id, tenant_id, unit_id, unit_name, course_code, lecturer_id, lecturer_name, room, session_date, scheduled_time, taught, patroller_id, patroller_name, patroller_staff_id, taken_at, entry_method, created_at, submission_id, remarks, patroller_device_hash, found_venue, found_start_time, found_date, venue_changed, offering_id, is_compensation, compensation_for, students_counted, class_group, school, department, compensation_for_at, end_time) FROM stdin;
9a25a726-cbe3-46f0-b82f-65d4536623d0	13ab41a8-0a50-401c-a095-23203a8e41be	CSE 2420	English Language skills	\N	KIU/STAFF/00001	Dr Jane Smith	LT-1	2026-08-02	09:00	t	d96df028-fa49-4cac-8794-5b1e191703c8	QA Patroller	KIU/QA/001	2026-08-02 05:22:04.466493+00	PATROL	2026-08-02 05:22:04.466493+00	\N	\N	\N	\N	\N	\N	f	\N	f	\N	\N	\N	\N	\N	\N	\N
c68a9fcb-3cbe-4e85-9e69-a28748d390e4	13ab41a8-0a50-401c-a095-23203a8e41be	CSE 2110	Data Science	\N	KIU/STAFF/00001	Dr Jane Smith	LT-2	2026-08-02	11:00	f	e80e16a0-14ef-4b3b-91f5-6a506b5d8660	QA Patroller	KIU/QA/001	2026-08-02 05:22:04.466493+00	PATROL	2026-08-02 05:22:04.466493+00	\N	\N	\N	\N	\N	\N	f	\N	f	\N	\N	\N	\N	\N	\N	\N
e1aa41af-8b19-4f98-adfd-e161e32baa23	13ab41a8-0a50-401c-a095-23203a8e41be	CSE 2420	English Language skills		L001			2026-08-02	08:00	f	05d7dc2c-daec-410d-974e-1d6aabea5421	Patrol Tester	QA-P-001	2026-08-02 10:55:58.053489+00	PATROL	2026-08-02 10:55:58.053489+00	\N	\N	PHONE-A	\N	\N	\N	f	\N	f	\N	\N	\N	\N	\N	\N	\N
4a9ef1d3-c3bb-43c0-a0a9-a9d1e7d4e651	13ab41a8-0a50-401c-a095-23203a8e41be	Not In The Curriculum	Not In The Curriculum	\N	Visiting	Visiting	\N	2026-08-11	07:05	t	16500a7a-3e87-4d96-9ff7-c5b82ebee5a5	MANT Monitor	MANT-MON	2026-08-11 09:35:00.660088+00	MANUAL	2026-08-10 21:34:46.900012+00	\N	\N		\N	\N	\N	f	\N	f	\N	5	\N	\N	\N	\N	\N
89c89143-4a63-4168-98c8-a901f25103e1	13ab41a8-0a50-401c-a095-23203a8e41be	Not In The Curriculum	Not In The Curriculum	\N	Visiting	Visiting	\N	2026-08-10	07:05	t	c7344398-19cb-4b33-bd66-0f8bb618fdd0	MANT Monitor	MANT-MON	2026-08-10 20:41:24.989869+00	MANUAL	2026-08-10 20:41:24.989869+00	\N	\N		\N	\N	\N	f	\N	f	\N	5	\N	\N	\N	\N	\N
\.


--
-- Data for Name: lecturer_presence_claims; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturer_presence_claims (claim_id, tenant_id, lecturer_user_id, lecturer_staff_id, lecturer_name, latitude, longitude, accuracy_metres, location_status, captured_at, unit_id, unit_name, room, day_of_week, scheduled_time, session_date, match_kind, minutes_from_start, note, device_hash, received_at, created_at) FROM stdin;
\.


--
-- Data for Name: lecturer_webauthn_credentials; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturer_webauthn_credentials (credential_id, tenant_id, lecturer_id, public_key, attestation_type, aaguid, sign_count, transports, created_at, last_used_at, credential) FROM stdin;
\.


--
-- Data for Name: lecturers; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.lecturers (lecturer_id, tenant_id, full_name, email, phone, department, created_at, staff_id, title, gender, user_id, school_id) FROM stdin;
ec27f1c8-5e10-4792-8bf1-dfd3c1cc4b4a	13ab41a8-0a50-401c-a095-23203a8e41be	SSERUNJOGI MARK	mark@studmc.kiu.ug	0783643736	Computer Science	2026-06-25 12:51:38.619279+00	458	Mr.	Male	f81491e0-c2c2-4abd-94ed-4d534f320c64	c78d76db-f936-4396-979f-483ac1202fb1
65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	13ab41a8-0a50-401c-a095-23203a8e41be	BYAMUKAMA PETER	peter@kiu.ac.ug	0788175631	Computer Science	2026-06-23 13:55:37.917341+00	KIU/STAFF/2332	Prof.	Male	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	c78d76db-f936-4396-979f-483ac1202fb1
\.


--
-- Data for Name: monitor_log_units; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.monitor_log_units (id, tenant_id, patrol_id, unit_id, unit_name, course_code, class_group, school, department, resolved, created_at) FROM stdin;
\.


--
-- Data for Name: notification_log; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.notification_log (log_id, tenant_id, kind, subject_key, subject_date, recipient_user_id, channels, sent_at) FROM stdin;
af757db2-9a06-43bc-8e7f-2d1333062307	13ab41a8-0a50-401c-a095-23203a8e41be	LECTURE_REMINDER	6ec9e599-437a-4412-bb9a-3cad7b3c6747	2026-08-08	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	APP	2026-08-08 04:52:29.978549+00
2beb2c17-faa8-4775-991f-911fac4b9946	13ab41a8-0a50-401c-a095-23203a8e41be	ATTENDANCE_MISSING	6ec9e599-437a-4412-bb9a-3cad7b3c6747	2026-08-08	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	APP	2026-08-08 05:11:30.036514+00
49461b65-a385-49c7-b8fb-179952e27908	13ab41a8-0a50-401c-a095-23203a8e41be	QA_ESCALATION	6ec9e599-437a-4412-bb9a-3cad7b3c6747	2026-08-08	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	APP	2026-08-08 05:21:29.987204+00
\.


--
-- Data for Name: notification_recipients; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.notification_recipients (notification_id, tenant_id, recipient_user_id, read_at, dismissed_at) FROM stdin;
f45708c0-d923-4663-ae21-312f7b38ccd6	13ab41a8-0a50-401c-a095-23203a8e41be	6b69af50-641f-4769-88d2-95c39e3a64e0	\N	\N
eb427ce4-c6a1-4d3d-955d-795448c04a8b	13ab41a8-0a50-401c-a095-23203a8e41be	9e4e6637-edfc-457f-b0ba-9da8131a5e3d	\N	\N
2900fd9e-793a-4cc1-89dc-c6c4ff99fa0c	13ab41a8-0a50-401c-a095-23203a8e41be	9e4e6637-edfc-457f-b0ba-9da8131a5e3d	\N	\N
a248e564-48f1-4223-bba9-b3ba9d91ffca	13ab41a8-0a50-401c-a095-23203a8e41be	9e4e6637-edfc-457f-b0ba-9da8131a5e3d	\N	\N
0f8fde10-5b12-43a7-a185-64f0da25bafc	13ab41a8-0a50-401c-a095-23203a8e41be	bee0244a-8a94-4fa2-b9a5-2187de888ea7	\N	\N
b7468c85-c0da-4202-b4f2-f19fb829304d	13ab41a8-0a50-401c-a095-23203a8e41be	bee0244a-8a94-4fa2-b9a5-2187de888ea7	\N	\N
de118a76-eb40-43cf-9ec1-7631ea41e4ef	13ab41a8-0a50-401c-a095-23203a8e41be	bee0244a-8a94-4fa2-b9a5-2187de888ea7	\N	\N
50af5396-34ce-4fff-9b26-a08fe6343814	13ab41a8-0a50-401c-a095-23203a8e41be	23b5e760-4e13-4dde-9fa6-f2aabeaac63f	\N	\N
400c57d4-8b58-4030-9c0a-e19b85d976b4	13ab41a8-0a50-401c-a095-23203a8e41be	23b5e760-4e13-4dde-9fa6-f2aabeaac63f	\N	\N
07f93359-e875-4ee2-ab4d-e706db71960e	13ab41a8-0a50-401c-a095-23203a8e41be	23b5e760-4e13-4dde-9fa6-f2aabeaac63f	\N	\N
0270a64a-1e16-4431-879d-d2f676803adc	13ab41a8-0a50-401c-a095-23203a8e41be	7d2d484a-d510-4ede-b1de-c2fcc6a96e0d	\N	\N
2b4a1792-ddbc-420b-8d62-ab4ac092aa60	13ab41a8-0a50-401c-a095-23203a8e41be	7d2d484a-d510-4ede-b1de-c2fcc6a96e0d	\N	\N
4057ed44-48e3-4f62-add6-b8b2428e2e96	13ab41a8-0a50-401c-a095-23203a8e41be	7d2d484a-d510-4ede-b1de-c2fcc6a96e0d	\N	\N
365422fd-fad0-4411-ae60-b91da763c4fa	13ab41a8-0a50-401c-a095-23203a8e41be	8ba316a0-2d4f-483d-b41f-04877681d6ad	\N	\N
a4d69b1e-921b-4407-9995-58549a046c05	13ab41a8-0a50-401c-a095-23203a8e41be	38a94711-1543-49bd-b2fe-dd6fc1e3e46a	\N	\N
a8654198-852a-4f09-8b3a-643947cbd75b	13ab41a8-0a50-401c-a095-23203a8e41be	38a94711-1543-49bd-b2fe-dd6fc1e3e46a	\N	\N
4af36035-e210-4ebf-9a5f-7018c70bc228	13ab41a8-0a50-401c-a095-23203a8e41be	38a94711-1543-49bd-b2fe-dd6fc1e3e46a	\N	\N
9c4214e3-1e95-4af3-aa77-9551fdb342cc	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
fc8b3eb1-44ff-4a6d-bfc4-2965c36f318f	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
b2b0da49-619f-457d-bb59-76783b162c0f	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
739bdd74-6ae1-40e9-89f8-6d7cf645f737	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
4ea86d73-3edf-4bdf-bbf8-692ca8465e0b	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
f3b4014d-9da9-4994-8505-5458c236cabd	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
e19fdb3b-642b-4217-8eff-b3e7ce89e981	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
8d137fe7-f99c-4808-ac90-e7d702353af7	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
ddff6144-d248-4672-bcc7-eaad60c68cce	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
f998e12b-5ab2-49ac-8a8d-228c039d332d	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
27b59a57-0b25-4a3e-8fde-bb0f0cf9c3df	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
bf0a1369-9ef7-4212-accf-1a5b918377b0	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
9338d4ce-9b0c-45bc-9cd4-873bb8661314	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
eabb559c-264e-43c9-a8a6-4e2650bdc214	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
c72edcbe-f491-45dc-a2b8-e74560ebb908	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
a97f02e6-9e7b-4631-b993-c1a44dc35d1b	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
ad36cee9-4e9c-40e7-9526-a03f34fb1526	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
5956c279-2bc5-4ba6-aeb6-be7de30a1eca	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
6aa79c82-3610-4d00-941e-a90300fdcb3a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
01176eac-940e-4dca-9cde-c8dc86c3c58e	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
dcda57cf-471a-4d92-b7b5-7b50415cc4e6	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
09e6323e-967a-4226-922c-9c57ee0fd822	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
6b6dd0f3-8d96-47bf-b9b8-c353d4f745e6	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
35f1601e-f604-45ef-b544-9b6e298a19cf	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
99c12bb2-38bc-4bf8-bc9c-5dd42840c786	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
53f89896-309a-4d2a-a97d-c9f0ca343280	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
7201f543-cbab-4672-baf2-2849a6d723b0	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
943bb7dd-0ba2-4330-90d2-28362dbfa5ed	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
019be5eb-ab9e-4f9e-ad1d-8acc59091408	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
569df5fe-a867-477d-b3c2-c56fed1162ba	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
6195df60-d391-4887-8629-8dc2a2beb0c5	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
0d75cfac-46bd-4d7c-9204-413957f8e063	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
224436fe-60d0-4c97-8f6e-55685b47eef8	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
d93d7c98-2b65-4c22-8d45-7f91abfed642	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
d82cab62-edde-4b74-b623-0de1f35470ce	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
1e6c9eb0-594a-4d75-a064-f6c8be621c9c	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
64dc3268-4d2d-4abc-a0f9-dd1cc2bbdfbc	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
190b3b65-410d-449f-821b-4e6da4a623ac	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
02ac1900-bfe1-4acf-9864-3e2a92664a25	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
31b65a93-4495-4777-9145-b806bd9578b5	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
5b16355f-566c-41b1-8ee7-744dfbbcba91	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
ca74ac76-c57b-48d4-a121-f915125889bb	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
2f912efb-4a20-42c9-abbc-f82a601535e8	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
969daf21-7498-44a4-8731-180ea22de4f3	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
c057b15b-651c-4dc9-bd5b-2e69bee50907	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
8f52e1ec-4b0e-47ae-917e-a25a2ba07dfe	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
6272394f-ae49-49de-8636-f1609c40b622	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
efa5f643-b133-45a0-b487-83e76fa2d507	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
398efd7f-020c-43bc-96a5-25b0e18f7586	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
32d9d2a8-a9a1-4e32-b2e0-a2f64974d188	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
ba68bfe1-5d4f-46b3-8590-86f4520361d2	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
8a0ad404-8076-4ff4-8fb8-9903764788db	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
7ef9346c-8ff5-4949-8285-c72722cbe5f7	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
a92ed287-97f4-4b3b-92c3-24a1939a079e	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
1be9c75b-b7be-4ac9-812e-4c1259ab4e81	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
e3ed2126-6e5c-4190-938a-f468f15f9f58	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
8d4d358f-35a7-4c3d-9554-28a65b13181e	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
bcbe4aa3-19bb-4bbc-a887-37b21d1981d6	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
41884603-d937-4162-82d6-6d3e40ee2310	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
0b3a19f8-e024-40e3-8aaa-fd17f2004090	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
8f150a95-c72c-4193-b045-bbefa9813508	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
b60a869a-4262-4a01-8833-cd1e58ed5700	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
8b0baf69-95d2-4e1c-a2d7-1a458b05419a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
b2fd6eca-9006-4918-85f6-025e6bba23e0	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
278b7454-121a-4a8c-9759-7103cfc13b6d	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
b1dcdc18-aaa6-4b2c-9946-4a6f69625d11	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
40c62b3a-9315-42df-81fc-1191e5235682	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
65c69d1d-ec0d-44c1-8a8c-0703a89e0a0a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
5b6916c4-8e09-4d03-a4a8-d24307406e30	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
bd980259-09eb-4b6c-81ab-e71d50651e9f	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
d9602da6-481a-4959-9087-a8084dd3370a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
81d3a06a-5418-44b7-82e6-5b80aa157860	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
3a019d24-5377-4413-89a3-dcaae8f3e7b9	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
9f4c4dc0-1102-4328-bcd2-f60f48403f85	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
9e627a23-bd4e-4cc0-9636-8e550dba847a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
7037b250-bbde-436b-93de-c54cdbacda24	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
fb90d8d5-927e-4655-bfdc-86dfaba8e258	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
2d31be1c-3a53-47a5-a50a-ec923d178f6f	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
d0a1f9d8-7305-4815-a15f-6d9e14e23bc4	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
eb8b4f0d-e085-47ef-ab52-651c093bae93	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
89d1fb8c-06fb-44da-9b78-63329f6cd01a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
cc06b0e1-70f5-40e0-8faa-d4fdc42e685f	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
e2adbbf7-3aa9-4a2f-9422-1435f7b9ab61	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
02d29910-3971-41c1-80e0-92ceefa67b1f	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
6b472e83-7bef-4a53-8996-389e6d2ae00c	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
bb8d7798-dde2-43d1-8ef4-fc3187bacfcd	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
009b62c4-6c13-4d9b-a664-5ef3aa03ef36	13ab41a8-0a50-401c-a095-23203a8e41be	b4fb88c1-208a-42cb-b34c-e3c0c953e074	\N	\N
3266f507-6c47-42a5-aaea-725bcd8f8e63	13ab41a8-0a50-401c-a095-23203a8e41be	29af2db8-ae9f-4921-b69e-d343504ff850	\N	\N
66e4d699-5764-4bdf-a4d9-29cf4e4ce4c7	13ab41a8-0a50-401c-a095-23203a8e41be	21f678c2-0b3b-4d70-a096-a90a5b654052	\N	\N
d5042c93-5cd8-4e5e-a95e-25690e50ceac	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
34fe7fdb-e66d-4bdf-9809-e002fed96aa9	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
1e7d0c28-8aa0-4fb6-a44c-2f18ae45f087	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
3439e300-8245-455b-a8cc-08f070bb3a6c	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
2624a3cc-e3ad-4f38-91a7-0515d89067cb	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
de17892f-0713-4477-8d38-5294d799e19c	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
15402f0d-f191-47c8-8658-e8cf4edd8283	13ab41a8-0a50-401c-a095-23203a8e41be	aa6c883e-fff5-4245-8d69-6b37f56831b5	\N	\N
fc2350a0-ca43-4e8c-9dc5-b0a2b32562a5	13ab41a8-0a50-401c-a095-23203a8e41be	0f767f54-af55-4801-8174-a20c64e6e681	\N	\N
b62f7608-1b0a-4a28-92b1-9bdbf5f01255	13ab41a8-0a50-401c-a095-23203a8e41be	1915abe9-26be-4cf7-a100-4f351d88a590	\N	\N
eb59b3cf-a497-4ff9-979e-92ab650acf74	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
d23a8d22-4360-4878-a1a7-31c53d5b29ac	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
34cd3e14-8e03-478a-a513-9411692966a9	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
fc25d0b3-a7b1-4823-88cb-7a1610aa03df	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
25c110e5-f9f6-4085-8d1d-eb738c0f341a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
817ae822-f53b-477c-9d68-d4b59b34c399	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
47374721-0e04-4fc6-bbcd-a9a7ed8cb3ac	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
6439e751-6e2d-4fc2-80ad-1b18f66d5761	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
ef2f682d-87ab-4d93-ace5-9ba1d29fedfb	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
73bde585-c8bb-4eed-9775-b9f75aabd859	13ab41a8-0a50-401c-a095-23203a8e41be	6d682bbf-a118-4592-b1fc-517d24e7f4ac	\N	\N
55940d61-9554-4e0d-b60d-1631d0a6f5fc	13ab41a8-0a50-401c-a095-23203a8e41be	064ff4b3-6ffa-41d3-9776-206f15b69a57	\N	\N
d1446d0c-e48e-41e2-9a0d-5a5cac34d042	13ab41a8-0a50-401c-a095-23203a8e41be	01f8d2e8-e9f3-472c-93e2-92c67ab2a556	\N	\N
ff2b1245-d479-4027-8cce-92100d67cf7c	13ab41a8-0a50-401c-a095-23203a8e41be	c70cb53e-aae9-480d-8fe5-1f6dc0dcb499	\N	\N
be36dc86-d074-4aef-8b34-c53ea8cdf32d	13ab41a8-0a50-401c-a095-23203a8e41be	225c7e1d-5635-4c9a-b29a-2cef4f74dd7f	\N	\N
e316335e-ca1a-4c6d-8b3b-536b3451f872	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
b7310bcd-8a5b-4d31-a82c-b3aaa4750a53	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
870c49d5-1866-41a0-8d3e-3f2d310f3796	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
3dfd3180-2399-4519-9498-f2ec938c4799	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
73190a32-ae44-47f4-9069-572a01b10d43	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
f0da856e-d3bd-4972-bcec-8854e5421aaa	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
1f27b7a7-f04c-4732-8d6a-0c36a409c03e	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
55067cb7-8d5c-458c-aac1-597699210b6a	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
4a46056a-c467-4b71-b9e4-451c1d925a70	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
2ad986b3-8d5c-4d2b-8440-eb0e0367a0a2	13ab41a8-0a50-401c-a095-23203a8e41be	a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	\N	\N
833a85c6-a959-4b1d-b626-bb23a2c4e025	13ab41a8-0a50-401c-a095-23203a8e41be	6a685e3c-75cd-43d8-b838-882240d32d44	\N	\N
900c4644-4adc-4c3a-a5a5-9fdccfe0060e	13ab41a8-0a50-401c-a095-23203a8e41be	c9e3a295-aa3f-4b06-bc93-68d3cd1c8c52	\N	\N
c400fe10-8bfc-4b6c-bf9c-27be9e49782e	13ab41a8-0a50-401c-a095-23203a8e41be	62be3f7e-24bf-445d-9a50-5f822b1a485a	\N	\N
49e4b7d3-d8df-4bad-86a8-2fbe81217378	13ab41a8-0a50-401c-a095-23203a8e41be	8ab939ff-811d-4dc2-8248-07c126dc04ba	\N	\N
4ac46d65-6e63-4c3e-be03-0fc5ea44c6a7	13ab41a8-0a50-401c-a095-23203a8e41be	f654fb29-5823-4634-8b5f-6df211168d34	\N	\N
a474b74c-39ed-4226-84dd-9f378fd225ce	13ab41a8-0a50-401c-a095-23203a8e41be	a1ef145b-99b3-4748-9f34-325f872ae044	\N	\N
77de9983-e50b-4a79-b3da-4f204c7e4f42	13ab41a8-0a50-401c-a095-23203a8e41be	287d5cab-0268-4ad7-97c7-7bd8ffdaeaed	\N	\N
d658c914-45c1-4fa8-be21-9369191cf82e	13ab41a8-0a50-401c-a095-23203a8e41be	c582bad2-b2fc-406f-ba13-64a77212228b	\N	\N
0169195a-2e59-4b53-b522-8d9d595e48ef	13ab41a8-0a50-401c-a095-23203a8e41be	d867c31e-101b-4823-aaed-dd4750555e76	\N	\N
\.


--
-- Data for Name: offering_unit_schedules; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.offering_unit_schedules (offering_id, unit_id, tenant_id, day_of_week, session_start, session_duration_minutes, schedule_locked) FROM stdin;
ab287ed5-4785-4ca7-b689-288b5e60d7c7	CSE 2110	13ab41a8-0a50-401c-a095-23203a8e41be	\N	17:00:00	180	t
ab287ed5-4785-4ca7-b689-288b5e60d7c7	CSE 2420	13ab41a8-0a50-401c-a095-23203a8e41be	1	16:00:00	180	t
ab287ed5-4785-4ca7-b689-288b5e60d7c7	DBE 3234	13ab41a8-0a50-401c-a095-23203a8e41be	2	14:00:00	180	f
f270a5bf-76b3-4188-8a44-e9fa6c9463df	CSE 2420	13ab41a8-0a50-401c-a095-23203a8e41be	6	08:00:00	60	f
ab287ed5-4785-4ca7-b689-288b5e60d7c7	CSE 2120	13ab41a8-0a50-401c-a095-23203a8e41be	\N	\N	0	t
\.


--
-- Data for Name: patroller_device_bindings; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.patroller_device_bindings (user_id, tenant_id, device_fingerprint_hash, bound_at, last_seen_at) FROM stdin;
05d7dc2c-daec-410d-974e-1d6aabea5421	13ab41a8-0a50-401c-a095-23203a8e41be	fp-verify	2026-08-05 21:11:18.597288+00	2026-08-06 00:22:49.45795+00
\.


--
-- Data for Name: patroller_pins; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.patroller_pins (user_id, tenant_id, pin_hash, set_at, last_verified_at, failed_attempts, locked_until) FROM stdin;
\.


--
-- Data for Name: qa_message_reads; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.qa_message_reads (message_id, tenant_id, user_id, read_at, dismissed_at) FROM stdin;
6a080516-7483-42eb-bac8-530d06eef81e	13ab41a8-0a50-401c-a095-23203a8e41be	bb6873f4-f907-4d40-95cb-b8572b192cc2	2026-07-29 00:24:55.648023+00	\N
\.


--
-- Data for Name: qa_messages; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.qa_messages (message_id, tenant_id, sender_id, sender_name, sender_role, audience, audience_value, subject, body, attachment_name, attachment_mime, attachment_data, created_at) FROM stdin;
37bea8b5-6f2a-4039-a72d-6e3d53c36680	13ab41a8-0a50-401c-a095-23203a8e41be	6ff915bd-a126-4a8e-90d1-7f7bad64389e	DAVID	DQA_DIRECTOR	ALL_QA	\N	Monthly QA report	Please review the attached figures.	\N	\N	\N	2026-07-29 00:23:44.422144+00
8d541ca1-c963-4fa3-80e5-166b3ef7f76e	13ab41a8-0a50-401c-a095-23203a8e41be	6ff915bd-a126-4a8e-90d1-7f7bad64389e	DAVID	DQA_DIRECTOR	DEPARTMENT	Computer Science	CS dept notice	CS-only.	\N	\N	\N	2026-07-29 00:23:44.466043+00
8be29ac8-c862-4473-8fdb-d86c090e013a	13ab41a8-0a50-401c-a095-23203a8e41be	bb6873f4-f907-4d40-95cb-b8572b192cc2	SEMUCYO JOSHUA	QA_OFFICER	DQA	\N	Re: report	Received, thanks.	\N	\N	\N	2026-07-29 00:23:44.880837+00
6a080516-7483-42eb-bac8-530d06eef81e	13ab41a8-0a50-401c-a095-23203a8e41be	6ff915bd-a126-4a8e-90d1-7f7bad64389e	DAVID	DQA_DIRECTOR	SCHOOL	SOMAC	SOMAC report	See attached.	report.txt	text/plain	\\x5141205245504f52543a20617474656e64616e6365203832250a43532064657074204f4b2e	2026-07-29 00:24:55.456534+00
d9bcc669-44e5-42e0-820e-0abe2e58dad5	13ab41a8-0a50-401c-a095-23203a8e41be	6ff915bd-a126-4a8e-90d1-7f7bad64389e	DAVID	DQA_DIRECTOR	DEPARTMENT	Computer Science||Nursing	Multi-dept notice	For CS and Nursing.	\N	\N	\N	2026-07-29 00:50:22.888119+00
\.


--
-- Data for Name: qa_rep_submissions; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.qa_rep_submissions (submission_id, tenant_id, submitted_by, submitter_name, submitter_role, scope_kind, department, school, period_label, period_from, period_to, notes, file_name, file_size, file_bytes, total_rows, parsed_rows, skipped_rows, parse_errors, created_at) FROM stdin;
\.


--
-- Data for Name: scheduled_job_runs; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.scheduled_job_runs (job_name, last_run_at, last_status, last_error, last_duration_ms, windows_caught_up, updated_at) FROM stdin;
lecture_reminder	2026-08-12 19:00:39.499238+00	OK	\N	3	1	2026-08-12 19:01:12.96011+00
attendance_missing	2026-08-12 19:00:39.561523+00	OK	\N	2	1	2026-08-12 19:01:12.967122+00
qa_escalation	2026-08-12 19:00:39.585106+00	OK	\N	2	1	2026-08-12 19:01:12.972289+00
employee_exceptions	2026-08-12 18:59:53.781331+00	OK	\N	0	0	2026-08-12 19:01:12.975747+00
employee_checkout_reminder	2026-08-12 19:00:39.647016+00	OK	\N	0	1	2026-08-12 19:01:12.978934+00
\.


--
-- Data for Name: schema_migrations; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.schema_migrations (version, name, checksum, adopted, statements, skipped, applied_at) FROM stdin;
001	init_schema	312226c6e046368d471415781ef1831ea38645677fb8b9878503339ecbe3d64d	t	21	18	2026-08-01 17:07:31.084892+00
002	indexes	81f9e04defaa0dd66d92ad900d702a903444c70142c4e863a3aaab6839bd1dfd	t	13	13	2026-08-01 17:07:31.187244+00
003	rls_policies	7a43acd4068eda61f4df084382a79a553138e4ff3d7d1e2e719bcc6a1e6e29e3	t	32	13	2026-08-01 17:07:31.259926+00
004	materialized_views	63322da8c3f54584e29f9bf79da1bba92bf31cf643bd5d1c329fa22c9abdbcaa	t	3	1	2026-08-01 17:07:31.323193+00
005	tenant_rsa_keys	739517ad9fcf9a447ff0c69cbacb0ded0ab49ed03df4ac3255119c8d5f24bc82	t	4	3	2026-08-01 17:07:31.352068+00
006	sync_package_hmac	dff0ae5780e5992b57326719a3a65e5a0d7dac3cc24e1e30d08f89e1ec0d01ce	f	1	0	2026-08-01 17:07:31.367781+00
007	integrity_and_scale	6ab33fb1ec1e44db59aa453807ccc3370cb55c153ba04cd94fc51f8be6076573	t	17	6	2026-08-01 17:07:31.372332+00
008	online_checkin	6df89c199786f07f81df6ca6c72668b3e5b3fc6470fca3cdbbc7dbb2b5af2aae	f	2	0	2026-08-01 17:07:31.416797+00
009	force_rls_and_app_role	a9b584b8ffeab56cd6812ba3e490ddc3a86d705fd2212d7daa79254a68fcf214	f	11	0	2026-08-01 17:07:31.421536+00
010	authenticated_checkin	be526b1b71ba5b079fbe91bd1b8a77793ba5b93e47eb702c42fde743e9399deb	f	1	0	2026-08-01 17:07:31.468822+00
011	lecturers_and_assignments	5e49f5a5f82352bf654978fc9757ff0f615428de53328a10af183bb9ddafc278	t	11	2	2026-08-01 17:07:31.474323+00
012	course_roadmap_and_semester	be36a96730e0d064faa068512346ce20ed12f90c6483ef366caca1cb3ffd8079	f	6	0	2026-08-01 17:07:31.494028+00
013	lecturer_attendance_indexes	421d47dd3ce9caa602c15b3bbe8eeb85e9c2b7b7e028debef8efd156abc2aa3c	f	6	0	2026-08-01 17:07:31.504155+00
014	lecturer_gate_qr	59d49aeaab0e99027843f285400eca7d90085e31aad190c963f2aa4868f8c0fa	f	1	0	2026-08-01 17:07:31.527092+00
015	super_admin_and_branding	326a10d2483e62f8b01f7e132b6d2b940afeeeaab6eb45d7fcb0800bf713c0b4	f	2	0	2026-08-01 17:07:31.53325+00
016	single_logo	0540227e35bfc58b6f622325889ba29f3e3f72a3aed7c69a22dd8302d2d71c22	f	1	0	2026-08-01 17:07:31.545505+00
017	brand_palette	d5d5eb5d01fa20d373c94dbe8488aa62dadb5935832e35ed901cfbc98746c79c	f	1	0	2026-08-01 17:07:31.556014+00
018	student_role	21b5c50144591afe85f1766313b7ded317511deaa478143b47017f4a2b4c03d8	f	1	0	2026-08-01 17:07:31.566465+00
019	institution_id	ee60be6aadca893c8b3f3435268016721692d9bbf6ef61422a329fbc5d10a321	f	2	0	2026-08-01 17:07:31.571576+00
020	session_schedule_and_staff_id	f68cc9ec41c2715df055c634b1a9209590ca99c77409d6c77dc25a993b3a773f	f	3	0	2026-08-01 17:07:31.582168+00
021	coordinator_code_one_course	692030d297c9493c705fe0e0768dbaba6680084a8e205b366fa9f676ba4a09a3	f	4	0	2026-08-01 17:07:31.595508+00
022	configurable_intakes	0e01309dd60b0c629c313fb3db211fba8f0216abf830761b8ec77a084b5db2d0	f	4	0	2026-08-01 17:07:31.627707+00
023	lecturer_start_end	3546d9b1d6e5d96102d628f0dd05ed2e84286917048cb373cd299d94cad5068d	f	1	0	2026-08-01 17:07:31.75336+00
024	levels_sessions	74cd1e2476c8202e5b946d2b33da2bd2feec5f60423a9cf6a4cb2374782f259b	t	23	2	2026-08-01 17:13:06.06404+00
025	contacts_and_staffid	36ee8a41c1527a5383e02fd31754c2b08c7e795cf73d277145a57505f0c443d4	f	3	0	2026-08-01 17:13:06.135322+00
026	titles_and_gender	7323815ae90e236a7186ee736a6d068e61c7c2fe80aeec6fec731fcf0c30379a	f	3	0	2026-08-01 17:13:06.142555+00
027	lecturer_quorum	b8047252554d7474345a1de0fa3ebb005887c10007449c88d22a6a05d5371d22	f	1	0	2026-08-01 17:13:06.146428+00
028	session_window_dvc_webauthn	7eff048e2e1d7a48e0c6ee28edf6e6d4a70845cf076e4a9d13df4a3703c06c40	t	8	1	2026-08-01 17:13:06.149847+00
029	lecturer_webauthn_cred	ffd697a90b3266c3a16030d46cce7b4f79428198e63b308edbb670dcc12a8e6d	f	1	0	2026-08-01 17:13:06.160877+00
030	lecturer_biometric_templates	1370caa711996adfe739241b64701ed2ada107e987591b65b285273bc1d95995	t	6	1	2026-08-01 17:13:06.163976+00
031	lecturer_lan_proximity	db36279918df48d43627a5f38031d26c4a9f2cfc97686b304b420863743cca83	f	2	0	2026-08-01 17:13:06.174049+00
032	level_on_student	208c55d8800014b614d7b8707a383a58066fbf7f66c191b8366c915d14d6b83b	f	3	0	2026-08-01 17:13:06.178501+00
033	offering_cohort	0a7a6bc33ffa1ff91b47611d38726921f4dc928f46f838ef8755a5f6e07f1b18	f	4	0	2026-08-01 17:13:06.189298+00
034	unit_level	b9b354b5920220391326d0f59e18b0b1f86b9df79ddb4d7e752d08719a6c0852	f	1	0	2026-08-01 17:13:06.195354+00
035	level_years	85e4d3e24ae6493ae988e942c3bc84f9759b979f7811745d5abc17c673bb9947	f	2	0	2026-08-01 17:13:06.198062+00
036	offering_cohort_deferrable	5c662cc329c6796d2b640ca78982190f5af0aa91f414bae1093f4e9e57948d79	t	2	2	2026-08-01 17:13:06.204407+00
037	course_level_years	7a9aec41f2136ec70c0928349197c9998b6fb0c64fd63d9f119ab514d9056e7a	f	1	0	2026-08-01 17:13:06.209437+00
038	platform_seed	edadde1b9c8d7e5c2c99d6ce026976d4252bb977bd408cb9dead3d3859f919ee	f	2	0	2026-08-01 17:13:06.211965+00
039	drop_beacons	d634fe499668dbefcb759e4db8d30e15b7829025d691159754ec0bd184f48c64	f	4	0	2026-08-01 17:13:06.223015+00
040	lecturer_login_role	e2f6f5c110bfde03f30a15df2e1bb693939d341c1b8cffe894557a65d4822e9b	f	2	0	2026-08-01 17:13:06.240415+00
041	timetable_slots	61ed54743f249e9f31412626fd00015d52cb0d7f71f1bd6d73c41c2811fde4e4	t	7	1	2026-08-01 17:13:06.243671+00
042	coordinator_delegations	a6cf92932bf102222bdc7ff9b07700d5dc2f549ec65768e4d9db7a2ddeee60b1	t	7	1	2026-08-01 17:13:06.258777+00
043	tenant_background_image	5dea476456ee8174c7983f7cce5b78997ac7b3ea67e718e20e298518004a091a	f	1	0	2026-08-01 17:13:06.267427+00
044	background_image_controls	fb876d790797ef9ea7f0b28fabda4386d285258e58353365331396237c147811	f	6	0	2026-08-01 17:13:06.270958+00
046	fix_attendance_summary	ad95a243ce599d0f5a46f9c5cdc7021a0a3f280c2953359eab8df8accb6cf1b9	f	1	0	2026-08-01 17:13:06.276697+00
047	employees	4d8990a6794d6d41d5a8af5bcc6e84dc9e9d9262715c7c9c43dc69a1e1dfe45d	t	12	2	2026-08-01 17:13:06.281763+00
048	semester_archives	fdb986fbf0281be44a810e2a80ea157098ebc0bcf883334c8a7e0e60507f5462	t	6	1	2026-08-01 17:13:06.300306+00
049	tenant_text_colors	911ba4329982ad8c9c4bb08db71854823f7a25cd8138cd1f44095ac8ef91be2f	f	1	0	2026-08-01 17:13:06.308674+00
050	student_device_bindings	7a2a768e8231efff8792c0e504369ab546bd4b6c3f31f1943debf8202d92367a	f	4	0	2026-08-01 17:13:06.311294+00
052	seed_default_passwords	e7bf590d2a53925896636ff35c1657bf022e8d958af3fa98797dc9ef28e97744	f	2	0	2026-08-01 17:13:06.342381+00
053	force_password_change	7d650e608fa8ea5a33a671c04b24dbafd34ae934b57c6318fb4c01881854b48d	f	2	0	2026-08-01 17:13:06.642032+00
054	user_department_school	80687ca994c37009ce07109ea0b9683bb531e9bbd9cba2dd14559597cac4159c	f	3	0	2026-08-01 17:13:06.646041+00
055	qa_messages	9ad7295b7ebe7df6a1c9dfd53e387574f4fee34eb3dca6f454561c4ec15eaaed	t	8	2	2026-08-01 17:13:06.652863+00
056	app_notifications	5f85f70e866b56794a5d364132f8d6ed071c43203128fc2b70d59a9681b0bb78	t	8	2	2026-08-01 17:13:06.670455+00
057	lecturer_daily_codes	0cc9372fd2265b18efe6b39b5f238947a6220e395b847b16f35aa385101de3f9	f	1	0	2026-08-01 17:13:06.678684+00
058	qa_patroller	68a497351fd017e0aa5a7151c2d4daf653e5b9062b9fde98cb220e309c10282f	f	9	0	2026-08-01 17:13:06.700584+00
059	schools_departments	e0d31cea1552bfaa0023a74d1eb533d3cb6870a5f0e02269af1f3ac84e458361	f	13	0	2026-08-01 17:13:06.742808+00
060	backfill_org_from_courses	1712e8e75a67ebf898a97057971e52f457d116b1e2779edcb61a10ed4a145ea4	f	6	0	2026-08-01 17:13:06.806189+00
061	hod_dean_qa_roles	bc702f8bf1375c58859921b1b8aa16c0b264e924f0105df14dd1c41228573b2e	f	4	0	2026-08-01 17:13:06.823145+00
062	qa_rep_reports_and_rooms	0441871ad5db10fad5d66d93ab8a5a7aae778caf7a42433481eba7d0ca53547c	f	21	0	2026-08-01 17:13:06.828032+00
063	drop_qr_subsystem	d7e2dbb1eb8618325bb23cb636294f0aeaa7bc586f9730a918e16de4e1d210ce	f	3	0	2026-08-01 20:06:32.027076+00
064	drop_super_admin_role	f876256311dd565a1e876b34b3bd02f1587cce69bb1d095ad0fb0caab1274d68	f	3	0	2026-08-01 20:06:32.133907+00
065	repair_session_offerings	3b3adbbcb13fa6db4a2c44bcc0a570c29bf1997ff3dfbacb9e60ef8c26a88998	f	2	0	2026-08-01 22:44:17.40225+00
066	standalone_support_departments	12303e28aa3fb1c34d1ceeaaaf4889f871c1fc68c9b1f15428a3d6768f3a7455	f	4	0	2026-08-02 05:14:43.764159+00
067	notification_dismiss	bdc886dad956b7ecd34a62b35988ec63ad3b5e1864e4ac20e2aab76a502536a6	f	2	0	2026-08-02 07:31:35.976852+00
068	qa_message_dismiss	943931fd3628256cc33598393f79d49ad4b809220cd04e90a2a5309f0921f790	f	2	0	2026-08-02 09:36:12.859623+00
069	patroller_device_binding	193b938953b714438aed12dfdd1c2b752dbdfff30bf1d598c9b8914873fdaf82	f	9	0	2026-08-02 20:48:15.843138+00
070	reseed_default_passwords	7412ea822d069be64c35ce9a2a81d9313aa9532d37dc448479b15494e625e30f	f	2	0	2026-08-02 20:48:15.889463+00
071	patroller_pin	6116d9998ed96da6661c636baae30dee6e74484768a1f755882324db4ed6f6c7	f	7	0	2026-08-02 20:48:16.219241+00
072	school_abbreviation	fe17ed763478e514c907e8277058277d0c411be92077a66d0c3175d91abbd850	f	3	0	2026-08-02 22:18:17.287869+00
073	scheduler_and_notification_log	cb64cc21d4abe19f7a2ab3cf0d83fbb983cfbb48d0eab5589fb30287ea93a0b8	f	8	0	2026-08-04 09:04:39.055514+00
074	timetable_consolidation	1e124b6eb03a1114fccd1e9c855e33b85cf47268c740607af3426d67d5bc64ac	f	6	0	2026-08-04 09:04:39.163924+00
075	org_roles_tlc_and_schools	5626441375ae6412646fd399264f8f784ed0791e2bb05483e4b7b23d45c7c427	f	12	0	2026-08-04 09:04:39.233522+00
076	patrol_search_and_venue_change	87dba1f4d5e0690d45fae05cfeebe126e8224663ed9126c089eab7a03ceb52f9	f	10	0	2026-08-04 09:04:39.301452+00
077	employee_attendance_days	c95c40bbb2def82f15d1617be7454fe7f6b41619e510bedd3ec6ebfb47d13e56	f	10	0	2026-08-04 09:04:39.359993+00
078	unlock_patroller_device	2ca296729566597e69d1a8917d374bba54ba1c8fad32af44e80928986ed9b112	f	2	0	2026-08-05 17:45:48.615719+00
079	rls_enabled_and_forced_everywhere	8669c7693639cb135a40a0fb1a9d20e13064bcbb50a22c1bb55bec7c0102c045	f	4	0	2026-08-06 00:21:15.385088+00
080	app_notifications_allow_system_sender	c940d5283943d6ea8a9069fb0b6de24015560a88da8a5fcdd08031c80f6cd6e1	f	2	0	2026-08-06 12:27:09.719479+00
081	lecturer_presence_claims	5dd80428feb16301502b8f484d1e44fbdf94366bbb9699594462bff5ab236388	f	9	0	2026-08-06 12:45:51.858818+00
082	unforce_rls_for_owner_services	8f0bb568a08c37ad7792b2939e361fe5b23ad9029dd7467b5043eb04bb517ab5	f	2	0	2026-08-07 23:29:53.062924+00
083	compensation_and_tlc_department	35b8398e2fc52af0fa3346533dce2ab92842ef11ca21d1dd2722ded2d63758c3	f	6	0	2026-08-08 13:08:16.477582+00
084	manual_monitor_attendance	1be17e787606dcef313832aaa8aac426d2429cdc08225380660f4523d59f4584	f	6	0	2026-08-08 14:35:15.521294+00
085	provision_rooms	9f2c64271b81b2c840fcb92aaeaea6dadcfdfcdde7900857f722bf8075bb6713	f	6	0	2026-08-10 19:02:10.084487+00
086	unscheduled_sessions	dc28d1517d1f580bd8dd40350dd4e13fbc61e44f91bd1652cc56567ba074b885	f	3	0	2026-08-10 20:33:47.515236+00
087	distance_learning_delivery	1aa333338fdcaf044d5ecb8b5a31b809179421c4d84a1f6ea6eb7a152ed8f45a	f	9	0	2026-08-10 21:08:52.310286+00
088	delivery_mode_from_session_type	5c16260646cf55931771fc65765d9a3b86503bec35d52def9c5f06bcc4307bf1	f	5	0	2026-08-10 21:33:20.919615+00
089	monitor_log_units_and_compensation_time	a6d6d764d9f8ad5435df3fa83988a18da662b5a190bf690ed68cea5efb801273	f	16	0	2026-08-10 22:50:23.595107+00
090	notification_actions	9bedfda30aeb0e596ee7d9cda19b0a96de32837781ebaed20f606687e0fe478a	f	4	0	2026-08-10 22:57:35.736035+00
091	no_room_double_booking	6e00e93eae3569b886734940c85b957aad64db00b9d8491850ee4dab12da4ac2	f	4	0	2026-08-12 16:53:55.686687+00
\.


--
-- Data for Name: schools; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.schools (school_id, tenant_id, name, created_at, abbreviation) FROM stdin;
c78d76db-f936-4396-979f-483ac1202fb1	13ab41a8-0a50-401c-a095-23203a8e41be	SOMAC	2026-08-01 17:13:06.806189+00	SOMAC
\.


--
-- Data for Name: semester_archives; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.semester_archives (archive_id, tenant_id, label, intakes, academic_year, semester, filename, content, size_bytes, attendance_rows, session_rows, lecturer_rows, created_by, created_at) FROM stdin;
\.


--
-- Data for Name: sessions; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.sessions (session_id, tenant_id, coordinator_id, unit_id, lecturer_id, venue_id, session_date, gate_open_time, gate_close_time, checkin_window_start, checkin_window_end, coordinator_end_time, auto_close_time, session_status, warden_id, sync_status, audit_flags, created_at, checkin_secret, planned_start, planned_duration_minutes, offering_id, coordinator_ip, room_is_provision, provision_note, unscheduled, delivery_mode) FROM stdin;
880ef56b-ae99-4e36-83fa-c1757682f1a2	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-08-08	2026-08-08 12:00:06.943087+00	2026-08-08 12:00:07.119278+00	2026-08-08 12:00:06.943087+00	2026-08-08 14:00:06.943087+00	2026-08-08 12:00:07.119278+00	\N	CLOSED	\N	PENDING	{}	2026-08-08 12:00:06.941064+00	\\x8b8dc4f8b3a4ad658d688bc1a139be12ce7569da563bd63ebcfc48a2671f471c	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
2b13c145-0929-4912-a818-03d74589d886	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-07-06	2026-07-06 07:14:34.438272+00	2026-07-06 07:14:35.964395+00	2026-07-06 07:14:34.438272+00	2026-07-06 09:14:34.438272+00	2026-07-06 07:14:35.964395+00	\N	CLOSED	\N	PENDING	{}	2026-07-06 07:14:34.453991+00	\\xdf0fc805010fb6ebd584a98b80e776c8f49a4ac035183ed4c265bc42abb51df5	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
729c9c46-3ef2-4428-a792-b73be7b0f16a	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-08-08	2026-08-08 07:51:37.26628+00	2026-08-08 07:51:37.954188+00	2026-08-08 07:51:37.26628+00	2026-08-08 09:51:37.26628+00	2026-08-08 07:51:37.954188+00	\N	CLOSED	\N	PENDING	{}	2026-08-08 07:51:37.265439+00	\\x06ba8e4fce6cf016b31f58d0421955abac9384db1ec34a31d4efa5716857dc7f	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
6d8cbda4-4eca-4ede-bc40-71d1460075bf	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2420	\N	\N	2026-07-05	\N	\N	\N	\N	\N	\N	CLOSED	\N	PENDING	{}	2026-07-05 07:24:56.047602+00	\N	\N	\N	ab287ed5-4785-4ca7-b689-288b5e60d7c7	\N	f	\N	f	IN_PERSON
3b379b4f-b4f8-451d-bc19-cc57dc1cf6b4	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2420	\N	\N	2026-07-05	\N	\N	\N	\N	\N	\N	CLOSED	\N	PENDING	{}	2026-07-05 07:24:56.269512+00	\N	\N	\N	ab287ed5-4785-4ca7-b689-288b5e60d7c7	\N	f	\N	f	IN_PERSON
3c2c371b-b6b8-4e32-b5ec-a0435d912bdf	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2420	\N	\N	2026-07-05	\N	\N	\N	\N	\N	\N	CLOSED	\N	PENDING	{}	2026-07-05 07:24:56.494253+00	\N	\N	\N	ab287ed5-4785-4ca7-b689-288b5e60d7c7	\N	f	\N	f	IN_PERSON
384aa8b4-79fb-49e9-acd8-84f039ff929d	13ab41a8-0a50-401c-a095-23203a8e41be	13b6a47b-ebbb-4315-bb3e-6a5c8377cbab	CSE 2420	\N	\N	2026-07-05	\N	\N	\N	\N	\N	\N	CLOSED	\N	PENDING	{}	2026-07-05 07:05:39.157622+00	\N	\N	\N	f270a5bf-76b3-4188-8a44-e9fa6c9463df	\N	f	\N	f	IN_PERSON
d94768c8-d099-46ae-b45d-37150e521c7c	13ab41a8-0a50-401c-a095-23203a8e41be	13b6a47b-ebbb-4315-bb3e-6a5c8377cbab	CSE 2420	\N	\N	2026-07-05	\N	\N	\N	\N	\N	\N	CLOSED	\N	PENDING	{}	2026-07-05 07:14:24.63571+00	\N	\N	\N	f270a5bf-76b3-4188-8a44-e9fa6c9463df	\N	f	\N	f	IN_PERSON
037bba66-8f7a-4df8-8561-c4d1e2de9374	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-08-08	2026-08-08 06:50:05.403964+00	2026-08-08 06:50:05.681824+00	2026-08-08 06:50:05.403964+00	2026-08-08 08:50:05.403964+00	2026-08-08 06:50:05.681824+00	\N	CLOSED	\N	PENDING	{}	2026-08-08 06:50:05.409547+00	\\xe59c4f4bbf18a8214daf44b6c882650cd6a8db899e26778480232e93694eb4e0	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
b33cbe4a-8ce7-4729-a136-4c58d22ba84d	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-08-08	2026-08-08 06:51:48.375604+00	2026-08-08 06:51:48.734184+00	2026-08-08 06:51:48.375604+00	2026-08-08 08:51:48.375604+00	2026-08-08 06:51:48.734184+00	\N	CLOSED	\N	PENDING	{}	2026-08-08 06:51:48.375854+00	\\x1308269344ae36fabc2a72ac3382d4a7817f0b027e0c7d3ac884ba1a7ed122dc	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
ef8080c5-d6c9-404b-84c5-e3c317cbce42	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-08-08	2026-08-08 11:41:12.700151+00	2026-08-08 11:41:13.195434+00	2026-08-08 11:41:12.700151+00	2026-08-08 13:41:12.700151+00	2026-08-08 11:41:13.195434+00	\N	CLOSED	\N	PENDING	{}	2026-08-08 11:41:12.693482+00	\\x1c81ffc5204402cca9f98a0058cc986a3f205f9cbc0fa647eb320bf12ec51acb	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
e67b7bbb-ddb4-460a-b0a6-be3037c2af88	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2420	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-07-03	2026-07-03 21:20:43.460672+00	2026-07-03 21:21:22.200036+00	2026-07-03 21:20:43.460672+00	2026-07-03 23:20:43.460672+00	2026-07-03 21:21:22.200036+00	\N	CLOSED	\N	PENDING	{}	2026-07-03 21:20:43.469873+00	\\x8f4c9e7d8e610d14c56f4ec0f4762a70fc3e5d69846559a4d429016cfd6c28f7	16:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
2c915be0-54ea-4e42-9dbd-b27092c45c12	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-07-06	2026-07-06 05:03:56.960594+00	2026-07-06 05:04:09.162978+00	2026-07-06 05:03:56.960594+00	2026-07-06 07:03:56.960594+00	2026-07-06 05:04:09.162978+00	\N	CLOSED	\N	PENDING	{}	2026-07-06 05:03:56.982693+00	\\x1793f1c28bb4ccac2332610b24798c85b34a50b255ffa1d168c8dd9e646d8c8c	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
1e0d76b4-50db-4251-95a0-c30125da017f	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-08-08	2026-08-08 07:05:21.10752+00	2026-08-08 07:05:21.466974+00	2026-08-08 07:05:21.10752+00	2026-08-08 09:05:21.10752+00	2026-08-08 07:05:21.466974+00	\N	CLOSED	\N	PENDING	{}	2026-08-08 07:05:21.108884+00	\\xd8643585f7bbdd2e8821ad93ac39fb61a8c29acb240b5f9898735fc6e4cfbeff	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
f7ef7c43-ef0d-465d-892e-789a4c1367f6	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	CSE 2110	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	\N	2026-08-08	2026-08-08 07:06:03.599048+00	2026-08-08 07:06:03.995365+00	2026-08-08 07:06:03.599048+00	2026-08-08 09:06:03.599048+00	2026-08-08 07:06:03.995365+00	\N	CLOSED	\N	PENDING	{}	2026-08-08 07:06:03.599429+00	\\x870e66d47c865d7b983306d86037c65c532910531455b297900395435037a93a	17:00:00	180	ab287ed5-4785-4ca7-b689-288b5e60d7c7	172.18.0.1	f	\N	f	IN_PERSON
\.


--
-- Data for Name: student_attendance_summary; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.student_attendance_summary (student_id, unit_id, unit_name, course_id, tenant_id, sessions_held, sessions_attended, attendance_percentage, refreshed_at) FROM stdin;
2025-08-40174	CSE 2210	Numerical Analysis	BSSE	280876a4-4005-42f4-8ab3-a0db75b8ec30	1	1	100.00	2026-06-19 15:26:52.165046+00
2025-08-41177	CSE 2002	Structured programming	BSSE	280876a4-4005-42f4-8ab3-a0db75b8ec30	1	1	100.00	2026-06-19 15:26:52.165046+00
2025-08-41177	CSE 2210	Numerical Analysis	BSSE	280876a4-4005-42f4-8ab3-a0db75b8ec30	2	2	100.00	2026-06-19 15:26:52.165046+00
2025-08-40174	CSE 2420	English Language skills	SE	13ab41a8-0a50-401c-a095-23203a8e41be	6	0	0.00	2026-07-05 07:24:56.503908+00
2025-08-40343	CSE 2420	English Language skills	SE	13ab41a8-0a50-401c-a095-23203a8e41be	6	0	0.00	2026-07-05 07:24:56.503908+00
s0000000-0000-0000-0000-000000000001	UNIT-E2E-001	Introduction to Programming	COURSE-E2E-001	a0000000-0000-0000-0000-000000000001	5	2	40.00	2026-07-25 20:04:26.327801+00
STU-ALICE	UNIT-E2E-001	Introduction to Programming	COURSE-E2E-001	a0000000-0000-0000-0000-000000000001	10	9	90.00	2026-08-04 09:58:28.802237+00
STU-BOB	UNIT-E2E-001	Introduction to Programming	COURSE-E2E-001	a0000000-0000-0000-0000-000000000001	10	4	40.00	2026-08-04 09:58:28.802237+00
\.


--
-- Data for Name: student_device_bindings; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.student_device_bindings (student_id, tenant_id, device_fingerprint_hash, attend_block_until, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: students_extended; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.students_extended (student_id, tenant_id, full_name, email, course_id, intake_session, current_year, semester, academic_year, enrollment_status, hardware_fingerprint, rebind_count, last_rebind_date, created_at, updated_at, offering_id, level, course_group) FROM stdin;
2025-08-40174	13ab41a8-0a50-401c-a095-23203a8e41be	NYAKWERA WINNIE	winnie@studmc.kiu.ac.ug	SE	August Intake	2	2	2025/2026	ACTIVE	\N	0	\N	2026-06-23 14:01:52.944533+00	2026-08-10 15:09:09.576545+00	ab287ed5-4785-4ca7-b689-288b5e60d7c7	Degree	Software Engineering
2025-08-40343	13ab41a8-0a50-401c-a095-23203a8e41be	SSERUNJOGI MARK	mark@studmc.kiu.ac.ug	SE	August Intake	2	1	2025/2026	ACTIVE	\N	0	\N	2026-06-25 12:47:34.895424+00	2026-08-10 15:09:09.576545+00	ab287ed5-4785-4ca7-b689-288b5e60d7c7	Degree	Software Engineering
\.


--
-- Data for Name: sync_uploads; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.sync_uploads (upload_id, tenant_id, coordinator_id, session_ids, total_chunks, received_chunks, package_checksum, status, chunk_size_bytes, created_at, completed_at, package_hmac) FROM stdin;
0f8b49c4-59f3-4e84-b879-2ed5428464a2	13ab41a8-0a50-401c-a095-23203a8e41be	13b6a47b-ebbb-4315-bb3e-6a5c8377cbab	{384aa8b4-79fb-49e9-acd8-84f039ff929d}	1	{0}	415418e9d8defc440fa0fd6b12562256e62ff86ec0c4e1897c95765151fa3628	SYNCED	65536	2026-07-05 07:05:39.011497+00	2026-07-05 07:05:39.210645+00	6d97bf41d67263725d755c55b43dad266d86eea92873c3d8a8b009b3156f719e
93ab1763-d879-428f-92cc-767a9d0955ab	13ab41a8-0a50-401c-a095-23203a8e41be	13b6a47b-ebbb-4315-bb3e-6a5c8377cbab	{d94768c8-d099-46ae-b45d-37150e521c7c}	1	{0}	22422393edb1eabbcf98ebe17b3842c370e7374dc76461b3b16ff9014dcf12c0	SYNCED	65536	2026-07-05 07:14:24.526373+00	2026-07-05 07:14:24.642776+00	49e48d767c19d014bb3871e6f8457cf6f5231bd7d16b88c18580115d11e5b520
d318323c-2154-4f7e-b3f6-32fac15aa23e	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	{6d8cbda4-4eca-4ede-bc40-71d1460075bf}	1	{0}	408bf3d9e8acff53c65ccf7813b6e17a90f57d9e6fda766ce01b8701f8753da3	SYNCED	65536	2026-07-05 07:24:55.954684+00	2026-07-05 07:24:56.054756+00	d1eebba26d7599c0ffb95967bcf8197b6d26cbd1b5510a1a23fd17246f205d21
55677a28-0664-496b-a329-b02e96b710c2	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	{3b379b4f-b4f8-451d-bc19-cc57dc1cf6b4}	1	{0}	b79688964f5414cf0100a0b99de886b1b0c64190daf53aac39f0ed582f38b47a	SYNCED	65536	2026-07-05 07:24:56.180283+00	2026-07-05 07:24:56.275955+00	0a1227738cbe7901d5842772fe553854c98f233595741fe3dbce0bc79ccd16e0
b157722f-f15b-4010-b0a7-bfa93888e5c3	13ab41a8-0a50-401c-a095-23203a8e41be	64ae5387-b705-45e5-b6ea-201cab10c1f8	{3c2c371b-b6b8-4e32-b5ec-a0435d912bdf}	1	{0}	5d4c1ba37a714d450094fa790f9163f0d82ec16a7f244f78d66799399f46943d	SYNCED	65536	2026-07-05 07:24:56.38729+00	2026-07-05 07:24:56.500821+00	bf26b6acafb2299a9563cdc74cb988dc09fcd7a9deae54844dfaa9b9c35713c3
\.


--
-- Data for Name: tenants; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.tenants (tenant_id, name, domain, rsa_key_id, attendance_threshold, checkin_window_minutes, auto_kill_minutes, logo_url, brand_color, is_active, created_at, student_hash_key, active_academic_year, active_semester, motto, slogan, address, sidebar_color, background_color, footer_color, institution_id, intakes, levels, study_sessions, staff_id_prefix, titles, lecturer_attendance_ratio, session_window_start, session_window_end, session_active_days, users_passcode_hash, require_lan_proximity, level_years, background_image, background_blur, background_brightness, background_contrast, background_overlay_color, background_overlay_opacity, text_color_light, text_color_dark) FROM stdin;
13ab41a8-0a50-401c-a095-23203a8e41be	KAMPALA INTERNATIOAL UNIVERSITY	kiu.ac.ug	kiu.ac.ug-rsa-key-v1	75	120	180	data:image/png;base64,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	#148f23	t	2026-06-23 13:43:10.177834+00	d838793e0e600f0ba1b190876553c32b50c8206f6ad2b66954ff0108aa7668ac	2025/2026	2	Exploring The Heights	\N	Kansanga - Kampala, Along Ggabba Road	#157528	#b3e5bd	#2aa22c	KIU-2102	{"January Intake","May Intake","August Intake"}	{Certificate,Diploma,Degree,Masters,PhD}	{Day,Evening,Distance,Weekend}	\N	{Prof.,Dr.,Eng.,Mr.,Mrs.,Ms.,Ass.}	0.5	07:00:00	18:00:00	{1,2,3,4,5,6,7}	$2a$12$kXEEh7N0DA1/15D.GOrVy.B3osAPI3arE1WL4FLI74bCh/S1C5DQe	t	{}	\N	0	30	30	\N	0	\N	\N
\.


--
-- Data for Name: timetable_slots; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.timetable_slots (slot_id, tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes, room, lecturer_id, created_at, venue_id) FROM stdin;
53ef0dcb-b8ac-4eb1-b010-3d830c4c22dd	13ab41a8-0a50-401c-a095-23203a8e41be	ab287ed5-4785-4ca7-b689-288b5e60d7c7	DBE 3234	4	08:00:00	60	\N	\N	2026-07-03 20:47:39.745802+00	\N
8550ab04-a9b1-4c72-9d9e-95c1d7d2933d	13ab41a8-0a50-401c-a095-23203a8e41be	ab287ed5-4785-4ca7-b689-288b5e60d7c7	DBE 3234	2	14:00:00	180	C04	\N	2026-07-03 19:12:29.404987+00	C04
6ed0a919-c504-4c3b-9783-f71944c75271	13ab41a8-0a50-401c-a095-23203a8e41be	ab287ed5-4785-4ca7-b689-288b5e60d7c7	CSE 2420	1	08:00:00	180	B01	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-07-03 19:11:59.767619+00	B01
6ec9e599-437a-4412-bb9a-3cad7b3c6747	13ab41a8-0a50-401c-a095-23203a8e41be	f270a5bf-76b3-4188-8a44-e9fa6c9463df	CSE 2420	6	08:00:00	60	B09	65f2b8d3-6fa2-4717-bc0b-c725c86e36cf	2026-07-05 04:32:09.404765+00	B09
\.


--
-- Data for Name: user_schools; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.user_schools (user_id, tenant_id, school_id, created_at) FROM stdin;
bb6873f4-f907-4d40-95cb-b8572b192cc2	13ab41a8-0a50-401c-a095-23203a8e41be	c78d76db-f936-4396-979f-483ac1202fb1	2026-08-04 09:04:39.233522+00
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.users (user_id, tenant_id, email, password_hash, role, full_name, is_active, totp_secret_enc, totp_enabled, totp_backup_codes_enc, device_binding_key_enc, last_login_at, failed_login_count, locked_until, created_at, updated_at, coordinator_code, phone, whatsapp, registration_number, title, gender, force_password_change, department, school, staff_id) FROM stdin;
5f3113e3-9097-4c80-9e1d-6e20a50f1b9b	13ab41a8-0a50-401c-a095-23203a8e41be	admin@kiu.ac.ug	$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52	ADMIN	SEMUCYO JOSHUA	t	\N	f	\N	\N	2026-08-12 18:20:34.680538+00	0	\N	2026-07-05 04:21:59.003968+00	2026-07-05 04:21:59.003968+00	\N	\N	\N	\N	\N	\N	f	\N	\N	KIU-ADMIN
ae74fe97-a9d8-4fff-a36b-a2180cdab181	13ab41a8-0a50-401c-a095-23203a8e41be	light@kiu.ac.ug	$2a$12$lYDbwyI8QpJR2i4rFNkuzuYSWXKjuopfBI8pzaVC.ciX61g69d4/y	VC	LIGHT	t	\N	f	\N	\N	2026-07-23 11:59:54.204987+00	0	\N	2026-06-23 14:27:09.66238+00	2026-06-23 14:27:09.66238+00	\N	\N	\N	\N	Prof.	Male	f	\N	\N	\N
6ff915bd-a126-4a8e-90d1-7f7bad64389e	13ab41a8-0a50-401c-a095-23203a8e41be	david@kiu.ac.ug	$2b$12$94aOId3Qt3gAuUdh8gYV1u4Yac01ot9k0JzbWD6479wIGp7qoGpX.	DQA_DIRECTOR	DAVID	t	\N	f	\N	\N	2026-07-29 00:50:21.574699+00	0	\N	2026-06-23 14:21:58.727773+00	2026-06-23 14:21:58.727773+00	\N	\N	\N	\N	Prof.	Male	f	\N	\N	\N
bf403b14-d64d-4b0f-8430-d81d6b6f91ca	13ab41a8-0a50-401c-a095-23203a8e41be	qa.nursing@kiu.ac.ug	$2b$12$94aOId3Qt3gAuUdh8gYV1u4Yac01ot9k0JzbWD6479wIGp7qoGpX.	QA_OFFICER	Nursing QA	t	\N	f	\N	\N	2026-07-29 00:50:22.825289+00	0	\N	2026-07-29 00:21:11.942679+00	2026-07-29 00:21:11.942679+00	\N	\N	\N	\N	\N	\N	f	Nursing	SOHS	\N
13b6a47b-ebbb-4315-bb3e-6a5c8377cbab	13ab41a8-0a50-401c-a095-23203a8e41be	mdavid@kiu.ac.ug	$2b$12$94aOId3Qt3gAuUdh8gYV1u4Yac01ot9k0JzbWD6479wIGp7qoGpX.	COORDINATOR	MUCHUNGUZI DAVID	t	\N	f	\N	Pl2UMFUnyX7e6qbHhEYDKrSL84aXS/erF3U88ZgEc3FFa3jVDjh10MsQv2sO4Qls+bEOQFUCszqOsIQrqC4m2zhOkUJZff4Uuc9QOSfaHsERA0zD6d2ZZU6A/64=	2026-07-29 01:46:28.529753+00	0	\N	2026-06-28 23:30:44.083943+00	2026-07-03 18:40:41.952028+00	CO-47D95A	0783743773	0783456652	2025-08-44433	Mr.	Male	f	\N	\N	\N
64ae5387-b705-45e5-b6ea-201cab10c1f8	13ab41a8-0a50-401c-a095-23203a8e41be	joshuasemucyo@kiu.ac.ug	$2a$10$EuNyUGf8PfigPFH0nZIvLeH391UaEQdyb4BYieEYsSIA56qhVQBaC	COORDINATOR	SEMUCYO JOSHUA	t	\N	f	\N	Rv9SFGcl6w85FJH2ztqYR+vFL1S4QYIgRGiZlEkc0DmRjUaf1EaH63E16pwTREjn7T/U7wAJ8fYBQKNV+C3GvdrW4aK9WfQd/vumTDeCkirD0z4cQpEj2CFDj3A=	2026-08-08 12:00:06.835092+00	0	\N	2026-06-23 13:53:39.488358+00	2026-08-08 06:50:05.226457+00	CO-61C15F	\N	0788175631	2025-08-41177	Mr.	Male	t	\N	\N	\N
f81491e0-c2c2-4abd-94ed-4d534f320c64	13ab41a8-0a50-401c-a095-23203a8e41be	lecturer.ec27f1c85e@kiu.ac.ug	$2a$10$hxY7JZImp3ZwgzPNoIap9OncNMocPcXA5D5RKV0MlVruPeeOE1AYi	LECTURER	SSERUNJOGI MARK	t	\N	f	\N	\N	\N	0	\N	2026-07-23 14:18:12.794945+00	2026-07-23 14:18:12.794945+00	\N	\N	\N	\N	\N	\N	t	\N	\N	\N
05d7dc2c-daec-410d-974e-1d6aabea5421	13ab41a8-0a50-401c-a095-23203a8e41be	patrol.test@kiu.ac.ug	$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52	QA_PATROLLER	Patrol Tester	t	\N	f	\N	\N	2026-08-02 10:56:20.617973+00	0	\N	2026-08-02 10:48:04.198621+00	2026-08-02 10:48:04.198621+00	\N	\N	\N	\N	\N	\N	f	\N	\N	QA-P-001
a626b8a5-f4e0-44d4-a6a2-bb0c7612815a	13ab41a8-0a50-401c-a095-23203a8e41be	peter@kiu.ac.ug	$2a$10$cHgNyDI0QGW9asyI9MHs0ud4LTg2d0ZqArmw2djspTrh3ccJhiRum	LECTURER	BYAMUKAMA PETER	t	\N	f	\N	\N	2026-08-02 21:03:05.944997+00	0	\N	2026-06-23 18:24:36.253936+00	2026-06-23 18:24:36.253936+00	\N	\N	\N	\N	\N	\N	t	\N	\N	\N
bb6873f4-f907-4d40-95cb-b8572b192cc2	13ab41a8-0a50-401c-a095-23203a8e41be	joshua@kiu.ac.ug	$2b$12$94aOId3Qt3gAuUdh8gYV1u4Yac01ot9k0JzbWD6479wIGp7qoGpX.	QA_OFFICER	SEMUCYO JOSHUA	t	\N	f	\N	\N	2026-07-29 00:50:22.225506+00	2	\N	2026-06-23 13:48:16.822976+00	2026-06-23 13:48:16.822976+00	\N	0788175631	\N	\N	Prof.	Male	f	Computer Science	SOMAC	\N
a75aaa5f-cd15-4edd-8c39-368919ae8d0b	13ab41a8-0a50-401c-a095-23203a8e41be	dqa.local@kiu.ac.ug	$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52	DQA_DIRECTOR	DQA Director (local)	t	\N	f	\N	\N	2026-08-12 18:18:32.088833+00	0	\N	2026-08-12 16:51:24.130117+00	2026-08-12 16:51:24.130117+00	\N	\N	\N	\N	\N	\N	f	\N	\N	\N
307104bb-6f31-412a-9826-d932e491aeb7	13ab41a8-0a50-401c-a095-23203a8e41be	winnie@studmc.kiu.ac.ug	$2a$10$EuNyUGf8PfigPFH0nZIvLeH391UaEQdyb4BYieEYsSIA56qhVQBaC	STUDENT	NYAKWERA WINNIE	t	\N	f	\N	\N	2026-08-08 11:41:12.916307+00	0	\N	2026-06-23 14:01:52.944533+00	2026-06-23 14:01:52.944533+00	\N	\N	\N	\N	\N	\N	t	\N	\N	\N
\.


--
-- Data for Name: venues; Type: TABLE DATA; Schema: public; Owner: qaat
--

COPY public.venues (venue_id, tenant_id, name, building, floor, capacity, gps_latitude, gps_longitude, geofence_radius_meters, school_id, department_id, room_type, is_active) FROM stdin;
A01	13ab41a8-0a50-401c-a095-23203a8e41be	A01	old building	1	100	\N	\N	50	\N	\N	LECTURE_HALL	t
B09	13ab41a8-0a50-401c-a095-23203a8e41be	B09	\N	\N	\N	\N	\N	50	c78d76db-f936-4396-979f-483ac1202fb1	\N	LECTURE_HALL	t
C04	13ab41a8-0a50-401c-a095-23203a8e41be	C04	\N	\N	\N	\N	\N	50	c78d76db-f936-4396-979f-483ac1202fb1	\N	LECTURE_HALL	t
B01	13ab41a8-0a50-401c-a095-23203a8e41be	B01	\N	\N	\N	\N	\N	50	c78d76db-f936-4396-979f-483ac1202fb1	\N	LECTURE_HALL	t
\.


--
-- Name: admin_audit_log admin_audit_log_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.admin_audit_log
    ADD CONSTRAINT admin_audit_log_pkey PRIMARY KEY (audit_id);


--
-- Name: app_notifications app_notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.app_notifications
    ADD CONSTRAINT app_notifications_pkey PRIMARY KEY (notification_id);


--
-- Name: attendance_logs attendance_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.attendance_logs
    ADD CONSTRAINT attendance_logs_pkey PRIMARY KEY (log_id);


--
-- Name: coordinator_delegations coordinator_delegations_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.coordinator_delegations
    ADD CONSTRAINT coordinator_delegations_pkey PRIMARY KEY (delegation_id);


--
-- Name: coordinator_delegations coordinator_delegations_tenant_id_code_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.coordinator_delegations
    ADD CONSTRAINT coordinator_delegations_tenant_id_code_key UNIQUE (tenant_id, code);


--
-- Name: course_offerings course_offerings_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_offerings
    ADD CONSTRAINT course_offerings_pkey PRIMARY KEY (offering_id);


--
-- Name: course_units course_units_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_units
    ADD CONSTRAINT course_units_pkey PRIMARY KEY (unit_id);


--
-- Name: courses courses_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.courses
    ADD CONSTRAINT courses_pkey PRIMARY KEY (course_id);


--
-- Name: departments departments_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.departments
    ADD CONSTRAINT departments_pkey PRIMARY KEY (department_id);


--
-- Name: departments departments_tenant_id_school_id_name_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.departments
    ADD CONSTRAINT departments_tenant_id_school_id_name_key UNIQUE (tenant_id, school_id, name);


--
-- Name: employee_attendance_days employee_attendance_days_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employee_attendance_days
    ADD CONSTRAINT employee_attendance_days_pkey PRIMARY KEY (day_id);


--
-- Name: employee_attendance_days employee_attendance_days_tenant_id_ac_no_work_date_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employee_attendance_days
    ADD CONSTRAINT employee_attendance_days_tenant_id_ac_no_work_date_key UNIQUE (tenant_id, ac_no, work_date);


--
-- Name: employee_attendance_logs employee_attendance_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employee_attendance_logs
    ADD CONSTRAINT employee_attendance_logs_pkey PRIMARY KEY (log_id);


--
-- Name: employee_attendance_logs employee_attendance_logs_tenant_id_staff_id_event_time_even_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employee_attendance_logs
    ADD CONSTRAINT employee_attendance_logs_tenant_id_staff_id_event_time_even_key UNIQUE (tenant_id, staff_id, event_time, event_type);


--
-- Name: employees employees_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_pkey PRIMARY KEY (employee_pk);


--
-- Name: employees employees_tenant_id_staff_id_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_tenant_id_staff_id_key UNIQUE (tenant_id, staff_id);


--
-- Name: hardware_vault hardware_vault_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.hardware_vault
    ADD CONSTRAINT hardware_vault_pkey PRIMARY KEY (student_id);


--
-- Name: lecturer_assignments lecturer_assignments_lecturer_id_unit_id_academic_year_inta_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_assignments
    ADD CONSTRAINT lecturer_assignments_lecturer_id_unit_id_academic_year_inta_key UNIQUE (lecturer_id, unit_id, academic_year, intake_session);


--
-- Name: lecturer_assignments lecturer_assignments_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_assignments
    ADD CONSTRAINT lecturer_assignments_pkey PRIMARY KEY (assignment_id);


--
-- Name: lecturer_attendance_logs lecturer_attendance_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_attendance_logs
    ADD CONSTRAINT lecturer_attendance_logs_pkey PRIMARY KEY (log_id);


--
-- Name: lecturer_biometric_templates lecturer_biometric_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_biometric_templates
    ADD CONSTRAINT lecturer_biometric_templates_pkey PRIMARY KEY (template_id);


--
-- Name: lecturer_daily_codes lecturer_daily_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_daily_codes
    ADD CONSTRAINT lecturer_daily_codes_pkey PRIMARY KEY (tenant_id, lecturer_id, valid_date);


--
-- Name: lecturer_daily_codes lecturer_daily_codes_tenant_id_valid_date_code_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_daily_codes
    ADD CONSTRAINT lecturer_daily_codes_tenant_id_valid_date_code_key UNIQUE (tenant_id, valid_date, code);


--
-- Name: lecturer_patrol_logs lecturer_patrol_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_patrol_logs
    ADD CONSTRAINT lecturer_patrol_logs_pkey PRIMARY KEY (patrol_id);


--
-- Name: lecturer_presence_claims lecturer_presence_claims_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_presence_claims
    ADD CONSTRAINT lecturer_presence_claims_pkey PRIMARY KEY (claim_id);


--
-- Name: lecturer_webauthn_credentials lecturer_webauthn_credentials_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_webauthn_credentials
    ADD CONSTRAINT lecturer_webauthn_credentials_pkey PRIMARY KEY (credential_id);


--
-- Name: lecturers lecturers_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturers
    ADD CONSTRAINT lecturers_pkey PRIMARY KEY (lecturer_id);


--
-- Name: monitor_log_units monitor_log_units_patrol_id_unit_id_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.monitor_log_units
    ADD CONSTRAINT monitor_log_units_patrol_id_unit_id_key UNIQUE (patrol_id, unit_id);


--
-- Name: monitor_log_units monitor_log_units_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.monitor_log_units
    ADD CONSTRAINT monitor_log_units_pkey PRIMARY KEY (id);


--
-- Name: notification_log notification_log_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.notification_log
    ADD CONSTRAINT notification_log_pkey PRIMARY KEY (log_id);


--
-- Name: notification_log notification_log_tenant_id_kind_subject_key_subject_date_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.notification_log
    ADD CONSTRAINT notification_log_tenant_id_kind_subject_key_subject_date_key UNIQUE (tenant_id, kind, subject_key, subject_date);


--
-- Name: notification_recipients notification_recipients_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.notification_recipients
    ADD CONSTRAINT notification_recipients_pkey PRIMARY KEY (notification_id, recipient_user_id);


--
-- Name: offering_unit_schedules offering_unit_schedules_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.offering_unit_schedules
    ADD CONSTRAINT offering_unit_schedules_pkey PRIMARY KEY (offering_id, unit_id);


--
-- Name: patroller_device_bindings patroller_device_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.patroller_device_bindings
    ADD CONSTRAINT patroller_device_bindings_pkey PRIMARY KEY (user_id);


--
-- Name: patroller_pins patroller_pins_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.patroller_pins
    ADD CONSTRAINT patroller_pins_pkey PRIMARY KEY (user_id);


--
-- Name: qa_message_reads qa_message_reads_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_message_reads
    ADD CONSTRAINT qa_message_reads_pkey PRIMARY KEY (message_id, user_id);


--
-- Name: qa_messages qa_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_messages
    ADD CONSTRAINT qa_messages_pkey PRIMARY KEY (message_id);


--
-- Name: qa_rep_submissions qa_rep_submissions_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_rep_submissions
    ADD CONSTRAINT qa_rep_submissions_pkey PRIMARY KEY (submission_id);


--
-- Name: scheduled_job_runs scheduled_job_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.scheduled_job_runs
    ADD CONSTRAINT scheduled_job_runs_pkey PRIMARY KEY (job_name);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: schools schools_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.schools
    ADD CONSTRAINT schools_pkey PRIMARY KEY (school_id);


--
-- Name: schools schools_tenant_id_name_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.schools
    ADD CONSTRAINT schools_tenant_id_name_key UNIQUE (tenant_id, name);


--
-- Name: semester_archives semester_archives_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.semester_archives
    ADD CONSTRAINT semester_archives_pkey PRIMARY KEY (archive_id);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (session_id);


--
-- Name: student_attendance_summary student_attendance_summary_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.student_attendance_summary
    ADD CONSTRAINT student_attendance_summary_pkey PRIMARY KEY (tenant_id, student_id, unit_id);


--
-- Name: student_device_bindings student_device_bindings_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.student_device_bindings
    ADD CONSTRAINT student_device_bindings_pkey PRIMARY KEY (student_id);


--
-- Name: students_extended students_extended_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.students_extended
    ADD CONSTRAINT students_extended_pkey PRIMARY KEY (student_id);


--
-- Name: sync_uploads sync_uploads_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.sync_uploads
    ADD CONSTRAINT sync_uploads_pkey PRIMARY KEY (upload_id);


--
-- Name: tenants tenants_domain_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_domain_key UNIQUE (domain);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (tenant_id);


--
-- Name: timetable_slots timetable_slots_no_room_double_booking; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_no_room_double_booking EXCLUDE USING gist (tenant_id WITH =, btrim(lower(room)) WITH =, day_of_week WITH =, int4range((((date_part('hour'::text, start_time) * (60)::double precision) + date_part('minute'::text, start_time)))::integer, ((((date_part('hour'::text, start_time) * (60)::double precision) + date_part('minute'::text, start_time)))::integer + GREATEST(COALESCE(duration_minutes, 60), 1))) WITH &&) WHERE (((room IS NOT NULL) AND (btrim(room) <> ''::text)));


--
-- Name: CONSTRAINT timetable_slots_no_room_double_booking ON timetable_slots; Type: COMMENT; Schema: public; Owner: qaat
--

COMMENT ON CONSTRAINT timetable_slots_no_room_double_booking ON public.timetable_slots IS 'One room cannot hold two lectures at overlapping times on the same weekday, across every department and college in the institution. Rooms are matched case- and whitespace-insensitively on the typed text; slots with no room named are exempt. The readable error comes from the handler''s own check — this is the backstop that survives two TLCs saving at the same instant.';


--
-- Name: timetable_slots timetable_slots_offering_id_unit_id_day_of_week_start_time_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_offering_id_unit_id_day_of_week_start_time_key UNIQUE (offering_id, unit_id, day_of_week, start_time);


--
-- Name: timetable_slots timetable_slots_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_pkey PRIMARY KEY (slot_id);


--
-- Name: user_schools user_schools_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.user_schools
    ADD CONSTRAINT user_schools_pkey PRIMARY KEY (user_id, school_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (user_id);


--
-- Name: users users_tenant_id_email_key; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_tenant_id_email_key UNIQUE (tenant_id, email);


--
-- Name: course_offerings ux_offerings_cohort; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_offerings
    ADD CONSTRAINT ux_offerings_cohort UNIQUE (tenant_id, course_id, session_type, study_year, semester, level, intake) DEFERRABLE INITIALLY DEFERRED;


--
-- Name: venues venues_pkey; Type: CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.venues
    ADD CONSTRAINT venues_pkey PRIMARY KEY (venue_id);


--
-- Name: idx_app_notifications_tenant_created; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_app_notifications_tenant_created ON public.app_notifications USING btree (tenant_id, created_at DESC);


--
-- Name: idx_attendance_session; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_attendance_session ON public.attendance_logs USING btree (session_id, tenant_id);


--
-- Name: idx_attendance_student; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_attendance_student ON public.attendance_logs USING btree (student_id, tenant_id);


--
-- Name: idx_attendance_student_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_attendance_student_unit ON public.attendance_logs USING btree (student_id, session_id) INCLUDE (checkin_timestamp);


--
-- Name: idx_audit_tenant_actor; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_audit_tenant_actor ON public.admin_audit_log USING btree (tenant_id, actor_id, occurred_at DESC);


--
-- Name: idx_course_units_id_lower; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_course_units_id_lower ON public.course_units USING btree (tenant_id, btrim(lower((unit_id)::text)));


--
-- Name: idx_course_units_roadmap; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_course_units_roadmap ON public.course_units USING btree (course_id, year, semester);


--
-- Name: idx_courses_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_courses_tenant ON public.courses USING btree (tenant_id);


--
-- Name: idx_departments_school; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_departments_school ON public.departments USING btree (tenant_id, school_id);


--
-- Name: idx_emp_days_department; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_emp_days_department ON public.employee_attendance_days USING btree (tenant_id, btrim(lower((department)::text)));


--
-- Name: idx_emp_days_flags; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_emp_days_flags ON public.employee_attendance_days USING btree (tenant_id, work_date) WHERE (absent OR checked_in_late OR checked_out_early);


--
-- Name: idx_emp_days_name; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_emp_days_name ON public.employee_attendance_days USING btree (tenant_id, btrim(lower(full_name)));


--
-- Name: idx_emp_days_tenant_date; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_emp_days_tenant_date ON public.employee_attendance_days USING btree (tenant_id, work_date DESC);


--
-- Name: idx_lal_lecturer; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_lal_lecturer ON public.lecturer_attendance_logs USING btree (lecturer_id, tenant_id, session_date DESC);


--
-- Name: idx_lal_session; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_lal_session ON public.lecturer_attendance_logs USING btree (session_id);


--
-- Name: idx_lal_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_lal_unit ON public.lecturer_attendance_logs USING btree (unit_id, tenant_id);


--
-- Name: idx_lecturer_assignments_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_lecturer_assignments_unit ON public.lecturer_assignments USING btree (unit_id, tenant_id);


--
-- Name: idx_lecturers_school; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_lecturers_school ON public.lecturers USING btree (tenant_id, school_id);


--
-- Name: idx_lecturers_staffid_lower; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_lecturers_staffid_lower ON public.lecturers USING btree (tenant_id, btrim(lower((staff_id)::text)));


--
-- Name: idx_notif_recipient; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_notif_recipient ON public.notification_recipients USING btree (tenant_id, recipient_user_id, read_at);


--
-- Name: idx_notif_recipients_visible; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_notif_recipients_visible ON public.notification_recipients USING btree (tenant_id, recipient_user_id) WHERE (dismissed_at IS NULL);


--
-- Name: idx_notification_log_tenant_date; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_notification_log_tenant_date ON public.notification_log USING btree (tenant_id, subject_date, kind);


--
-- Name: idx_patrol_bindings_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_patrol_bindings_tenant ON public.patroller_device_bindings USING btree (tenant_id);


--
-- Name: idx_patrol_device_fingerprint; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_patrol_device_fingerprint ON public.patroller_device_bindings USING btree (device_fingerprint_hash);


--
-- Name: idx_patrol_logs_changed; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_patrol_logs_changed ON public.lecturer_patrol_logs USING btree (tenant_id, session_date) WHERE venue_changed;


--
-- Name: idx_patrol_logs_date; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_patrol_logs_date ON public.lecturer_patrol_logs USING btree (tenant_id, session_date);


--
-- Name: idx_patrol_logs_lecturer; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_patrol_logs_lecturer ON public.lecturer_patrol_logs USING btree (tenant_id, lecturer_id, session_date);


--
-- Name: idx_patrol_logs_submission; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_patrol_logs_submission ON public.lecturer_patrol_logs USING btree (submission_id) WHERE (submission_id IS NOT NULL);


--
-- Name: idx_patroller_pins_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_patroller_pins_tenant ON public.patroller_pins USING btree (tenant_id);


--
-- Name: idx_presence_claims_lecturer; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_presence_claims_lecturer ON public.lecturer_presence_claims USING btree (tenant_id, lecturer_staff_id, session_date);


--
-- Name: idx_presence_claims_tenant_time; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_presence_claims_tenant_time ON public.lecturer_presence_claims USING btree (tenant_id, captured_at DESC);


--
-- Name: idx_qa_messages_audience; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_qa_messages_audience ON public.qa_messages USING btree (tenant_id, audience, audience_value);


--
-- Name: idx_qa_messages_tenant_created; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_qa_messages_tenant_created ON public.qa_messages USING btree (tenant_id, created_at DESC);


--
-- Name: idx_qa_rep_submissions_by; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_qa_rep_submissions_by ON public.qa_rep_submissions USING btree (tenant_id, submitted_by, created_at DESC);


--
-- Name: idx_qa_rep_submissions_scope; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_qa_rep_submissions_scope ON public.qa_rep_submissions USING btree (tenant_id, school, department, created_at DESC);


--
-- Name: idx_sdb_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_sdb_tenant ON public.student_device_bindings USING btree (tenant_id);


--
-- Name: idx_sessions_coordinator; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_sessions_coordinator ON public.sessions USING btree (coordinator_id, session_date, tenant_id);


--
-- Name: idx_sessions_status; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_sessions_status ON public.sessions USING btree (tenant_id, session_status);


--
-- Name: idx_sessions_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_sessions_unit ON public.sessions USING btree (unit_id, session_date, tenant_id);


--
-- Name: idx_students_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_students_tenant ON public.students_extended USING btree (tenant_id, enrollment_status);


--
-- Name: idx_summary_tenant_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_summary_tenant_unit ON public.student_attendance_summary USING btree (tenant_id, unit_id);


--
-- Name: idx_sync_uploads_coord; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_sync_uploads_coord ON public.sync_uploads USING btree (coordinator_id, tenant_id, status);


--
-- Name: idx_timetable_slots_day_lecturer; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_timetable_slots_day_lecturer ON public.timetable_slots USING btree (tenant_id, day_of_week, lecturer_id);


--
-- Name: idx_timetable_slots_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_timetable_slots_unit ON public.timetable_slots USING btree (tenant_id, unit_id);


--
-- Name: idx_timetable_slots_venue; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_timetable_slots_venue ON public.timetable_slots USING btree (tenant_id, venue_id);


--
-- Name: idx_units_course; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_units_course ON public.course_units USING btree (course_id, tenant_id);


--
-- Name: idx_user_schools_tenant_user; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_user_schools_tenant_user ON public.user_schools USING btree (tenant_id, user_id);


--
-- Name: idx_users_dept_school; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_users_dept_school ON public.users USING btree (tenant_id, role, department, school);


--
-- Name: idx_users_tenant_email; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_users_tenant_email ON public.users USING btree (tenant_id, email);


--
-- Name: idx_users_tenant_role; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_users_tenant_role ON public.users USING btree (tenant_id, role) WHERE (is_active = true);


--
-- Name: idx_venues_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX idx_venues_tenant ON public.venues USING btree (tenant_id, is_active);


--
-- Name: ix_app_notifications_action; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_app_notifications_action ON public.app_notifications USING btree (tenant_id, created_at DESC) WHERE (action IS NOT NULL);


--
-- Name: ix_bio_tmpl_lecturer; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_bio_tmpl_lecturer ON public.lecturer_biometric_templates USING btree (tenant_id, lecturer_id);


--
-- Name: ix_coord_deleg_code; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_coord_deleg_code ON public.coordinator_delegations USING btree (code);


--
-- Name: ix_coord_deleg_coordinator; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_coord_deleg_coordinator ON public.coordinator_delegations USING btree (coordinator_id);


--
-- Name: ix_emp_att_tenant_staff; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_emp_att_tenant_staff ON public.employee_attendance_logs USING btree (tenant_id, staff_id, event_time);


--
-- Name: ix_employees_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_employees_tenant ON public.employees USING btree (tenant_id);


--
-- Name: ix_lecturer_assignments_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_lecturer_assignments_unit ON public.lecturer_assignments USING btree (tenant_id, unit_id);


--
-- Name: ix_monitor_log_units_log; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_monitor_log_units_log ON public.monitor_log_units USING btree (patrol_id);


--
-- Name: ix_monitor_log_units_unit; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_monitor_log_units_unit ON public.monitor_log_units USING btree (tenant_id, unit_id);


--
-- Name: ix_patrol_logs_compensation; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_patrol_logs_compensation ON public.lecturer_patrol_logs USING btree (tenant_id, lecturer_id, session_date) WHERE is_compensation;


--
-- Name: ix_patrol_logs_manual; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_patrol_logs_manual ON public.lecturer_patrol_logs USING btree (tenant_id, session_date) WHERE (entry_method = 'MANUAL'::text);


--
-- Name: ix_semester_archives_tenant; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_semester_archives_tenant ON public.semester_archives USING btree (tenant_id, created_at DESC);


--
-- Name: ix_sessions_online; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_sessions_online ON public.sessions USING btree (tenant_id, session_date) WHERE (delivery_mode = 'ONLINE'::text);


--
-- Name: ix_sessions_provision; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_sessions_provision ON public.sessions USING btree (tenant_id, session_date) WHERE room_is_provision;


--
-- Name: ix_sessions_unscheduled; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_sessions_unscheduled ON public.sessions USING btree (tenant_id, session_date) WHERE unscheduled;


--
-- Name: ix_sessions_venue_date; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_sessions_venue_date ON public.sessions USING btree (tenant_id, venue_id, session_date) WHERE (venue_id IS NOT NULL);


--
-- Name: ix_timetable_slots_offering; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_timetable_slots_offering ON public.timetable_slots USING btree (offering_id);


--
-- Name: ix_timetable_slots_room_day; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_timetable_slots_room_day ON public.timetable_slots USING btree (tenant_id, venue_id, day_of_week, start_time) WHERE (venue_id IS NOT NULL);


--
-- Name: ix_users_tlc_department; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_users_tlc_department ON public.users USING btree (tenant_id, department) WHERE (role = 'TLC'::public.user_role_enum);


--
-- Name: ix_webauthn_lecturer; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX ix_webauthn_lecturer ON public.lecturer_webauthn_credentials USING btree (tenant_id, lecturer_id);


--
-- Name: student_attendance_summary_student_id_unit_id_tenant_id_idx; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX student_attendance_summary_student_id_unit_id_tenant_id_idx ON public.student_attendance_summary USING btree (student_id, unit_id, tenant_id);


--
-- Name: student_attendance_summary_tenant_id_unit_id_idx; Type: INDEX; Schema: public; Owner: qaat
--

CREATE INDEX student_attendance_summary_tenant_id_unit_id_idx ON public.student_attendance_summary USING btree (tenant_id, unit_id);


--
-- Name: uq_attendance_session_student; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX uq_attendance_session_student ON public.attendance_logs USING btree (tenant_id, session_id, student_id) WHERE (entry_method = 'QR_SCAN'::public.entry_method_enum);


--
-- Name: uq_attendance_session_student_auth; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX uq_attendance_session_student_auth ON public.attendance_logs USING btree (tenant_id, session_id, student_id) WHERE (entry_method = 'AUTHENTICATED'::public.entry_method_enum);


--
-- Name: uq_attendance_vector_clock; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX uq_attendance_vector_clock ON public.attendance_logs USING btree (tenant_id, session_id, coordinator_id, sequence_number) WHERE (entry_method = 'QR_SCAN'::public.entry_method_enum);


--
-- Name: uq_device_one_student; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX uq_device_one_student ON public.student_device_bindings USING btree (device_fingerprint_hash);


--
-- Name: uq_tenants_institution_id; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX uq_tenants_institution_id ON public.tenants USING btree (institution_id) WHERE (institution_id IS NOT NULL);


--
-- Name: ux_departments_standalone_name; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX ux_departments_standalone_name ON public.departments USING btree (tenant_id, name) WHERE (school_id IS NULL);


--
-- Name: ux_lecturers_tenant_staffid; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX ux_lecturers_tenant_staffid ON public.lecturers USING btree (tenant_id, staff_id) WHERE ((staff_id IS NOT NULL) AND ((staff_id)::text <> ''::text));


--
-- Name: ux_offerings_tenant_coordinator; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX ux_offerings_tenant_coordinator ON public.course_offerings USING btree (tenant_id, coordinator_id) WHERE (coordinator_id IS NOT NULL);


--
-- Name: ux_patrol_logs_slot; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX ux_patrol_logs_slot ON public.lecturer_patrol_logs USING btree (tenant_id, unit_id, session_date, scheduled_time, COALESCE(offering_id, '00000000-0000-0000-0000-000000000000'::uuid));


--
-- Name: ux_schools_abbreviation; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX ux_schools_abbreviation ON public.schools USING btree (tenant_id, lower(btrim((abbreviation)::text))) WHERE ((abbreviation IS NOT NULL) AND (btrim((abbreviation)::text) <> ''::text));


--
-- Name: ux_users_tenant_coordinator_code; Type: INDEX; Schema: public; Owner: qaat
--

CREATE UNIQUE INDEX ux_users_tenant_coordinator_code ON public.users USING btree (tenant_id, coordinator_code) WHERE (coordinator_code IS NOT NULL);


--
-- Name: course_offerings trg_offering_delivery_mode; Type: TRIGGER; Schema: public; Owner: qaat
--

CREATE TRIGGER trg_offering_delivery_mode BEFORE INSERT ON public.course_offerings FOR EACH ROW EXECUTE FUNCTION public.offering_delivery_mode_default();


--
-- Name: admin_audit_log admin_audit_log_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.admin_audit_log
    ADD CONSTRAINT admin_audit_log_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: app_notifications app_notifications_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.app_notifications
    ADD CONSTRAINT app_notifications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: attendance_logs attendance_logs_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.attendance_logs
    ADD CONSTRAINT attendance_logs_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.sessions(session_id);


--
-- Name: attendance_logs attendance_logs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.attendance_logs
    ADD CONSTRAINT attendance_logs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: coordinator_delegations coordinator_delegations_offering_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.coordinator_delegations
    ADD CONSTRAINT coordinator_delegations_offering_id_fkey FOREIGN KEY (offering_id) REFERENCES public.course_offerings(offering_id) ON DELETE CASCADE;


--
-- Name: coordinator_delegations coordinator_delegations_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.coordinator_delegations
    ADD CONSTRAINT coordinator_delegations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: course_offerings course_offerings_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_offerings
    ADD CONSTRAINT course_offerings_course_id_fkey FOREIGN KEY (course_id) REFERENCES public.courses(course_id) ON DELETE CASCADE;


--
-- Name: course_offerings course_offerings_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_offerings
    ADD CONSTRAINT course_offerings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: course_units course_units_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_units
    ADD CONSTRAINT course_units_course_id_fkey FOREIGN KEY (course_id) REFERENCES public.courses(course_id);


--
-- Name: course_units course_units_default_venue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_units
    ADD CONSTRAINT course_units_default_venue_id_fkey FOREIGN KEY (default_venue_id) REFERENCES public.venues(venue_id) ON DELETE SET NULL;


--
-- Name: course_units course_units_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.course_units
    ADD CONSTRAINT course_units_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: courses courses_department_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.courses
    ADD CONSTRAINT courses_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments(department_id) ON DELETE SET NULL;


--
-- Name: courses courses_school_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.courses
    ADD CONSTRAINT courses_school_id_fkey FOREIGN KEY (school_id) REFERENCES public.schools(school_id) ON DELETE SET NULL;


--
-- Name: courses courses_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.courses
    ADD CONSTRAINT courses_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: departments departments_school_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.departments
    ADD CONSTRAINT departments_school_id_fkey FOREIGN KEY (school_id) REFERENCES public.schools(school_id) ON DELETE CASCADE;


--
-- Name: departments departments_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.departments
    ADD CONSTRAINT departments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: employee_attendance_days employee_attendance_days_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employee_attendance_days
    ADD CONSTRAINT employee_attendance_days_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: employee_attendance_logs employee_attendance_logs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employee_attendance_logs
    ADD CONSTRAINT employee_attendance_logs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: employees employees_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: hardware_vault hardware_vault_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.hardware_vault
    ADD CONSTRAINT hardware_vault_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturer_assignments lecturer_assignments_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_assignments
    ADD CONSTRAINT lecturer_assignments_course_id_fkey FOREIGN KEY (course_id) REFERENCES public.courses(course_id);


--
-- Name: lecturer_assignments lecturer_assignments_lecturer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_assignments
    ADD CONSTRAINT lecturer_assignments_lecturer_id_fkey FOREIGN KEY (lecturer_id) REFERENCES public.lecturers(lecturer_id) ON DELETE CASCADE;


--
-- Name: lecturer_assignments lecturer_assignments_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_assignments
    ADD CONSTRAINT lecturer_assignments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturer_assignments lecturer_assignments_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_assignments
    ADD CONSTRAINT lecturer_assignments_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.course_units(unit_id);


--
-- Name: lecturer_attendance_logs lecturer_attendance_logs_session_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_attendance_logs
    ADD CONSTRAINT lecturer_attendance_logs_session_id_fkey FOREIGN KEY (session_id) REFERENCES public.sessions(session_id);


--
-- Name: lecturer_attendance_logs lecturer_attendance_logs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_attendance_logs
    ADD CONSTRAINT lecturer_attendance_logs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturer_biometric_templates lecturer_biometric_templates_lecturer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_biometric_templates
    ADD CONSTRAINT lecturer_biometric_templates_lecturer_id_fkey FOREIGN KEY (lecturer_id) REFERENCES public.lecturers(lecturer_id) ON DELETE CASCADE;


--
-- Name: lecturer_biometric_templates lecturer_biometric_templates_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_biometric_templates
    ADD CONSTRAINT lecturer_biometric_templates_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturer_patrol_logs lecturer_patrol_logs_submission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_patrol_logs
    ADD CONSTRAINT lecturer_patrol_logs_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES public.qa_rep_submissions(submission_id) ON DELETE CASCADE;


--
-- Name: lecturer_patrol_logs lecturer_patrol_logs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_patrol_logs
    ADD CONSTRAINT lecturer_patrol_logs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturer_presence_claims lecturer_presence_claims_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_presence_claims
    ADD CONSTRAINT lecturer_presence_claims_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturer_webauthn_credentials lecturer_webauthn_credentials_lecturer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_webauthn_credentials
    ADD CONSTRAINT lecturer_webauthn_credentials_lecturer_id_fkey FOREIGN KEY (lecturer_id) REFERENCES public.lecturers(lecturer_id) ON DELETE CASCADE;


--
-- Name: lecturer_webauthn_credentials lecturer_webauthn_credentials_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturer_webauthn_credentials
    ADD CONSTRAINT lecturer_webauthn_credentials_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturers lecturers_school_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturers
    ADD CONSTRAINT lecturers_school_id_fkey FOREIGN KEY (school_id) REFERENCES public.schools(school_id) ON DELETE SET NULL;


--
-- Name: lecturers lecturers_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturers
    ADD CONSTRAINT lecturers_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: lecturers lecturers_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.lecturers
    ADD CONSTRAINT lecturers_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE SET NULL;


--
-- Name: monitor_log_units monitor_log_units_patrol_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.monitor_log_units
    ADD CONSTRAINT monitor_log_units_patrol_id_fkey FOREIGN KEY (patrol_id) REFERENCES public.lecturer_patrol_logs(patrol_id) ON DELETE CASCADE;


--
-- Name: monitor_log_units monitor_log_units_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.monitor_log_units
    ADD CONSTRAINT monitor_log_units_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: notification_log notification_log_recipient_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.notification_log
    ADD CONSTRAINT notification_log_recipient_user_id_fkey FOREIGN KEY (recipient_user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: notification_log notification_log_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.notification_log
    ADD CONSTRAINT notification_log_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: notification_recipients notification_recipients_notification_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.notification_recipients
    ADD CONSTRAINT notification_recipients_notification_id_fkey FOREIGN KEY (notification_id) REFERENCES public.app_notifications(notification_id) ON DELETE CASCADE;


--
-- Name: notification_recipients notification_recipients_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.notification_recipients
    ADD CONSTRAINT notification_recipients_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: offering_unit_schedules offering_unit_schedules_offering_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.offering_unit_schedules
    ADD CONSTRAINT offering_unit_schedules_offering_id_fkey FOREIGN KEY (offering_id) REFERENCES public.course_offerings(offering_id) ON DELETE CASCADE;


--
-- Name: offering_unit_schedules offering_unit_schedules_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.offering_unit_schedules
    ADD CONSTRAINT offering_unit_schedules_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: offering_unit_schedules offering_unit_schedules_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.offering_unit_schedules
    ADD CONSTRAINT offering_unit_schedules_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.course_units(unit_id) ON DELETE CASCADE;


--
-- Name: patroller_device_bindings patroller_device_bindings_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.patroller_device_bindings
    ADD CONSTRAINT patroller_device_bindings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: patroller_device_bindings patroller_device_bindings_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.patroller_device_bindings
    ADD CONSTRAINT patroller_device_bindings_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: patroller_pins patroller_pins_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.patroller_pins
    ADD CONSTRAINT patroller_pins_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: patroller_pins patroller_pins_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.patroller_pins
    ADD CONSTRAINT patroller_pins_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: qa_message_reads qa_message_reads_message_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_message_reads
    ADD CONSTRAINT qa_message_reads_message_id_fkey FOREIGN KEY (message_id) REFERENCES public.qa_messages(message_id) ON DELETE CASCADE;


--
-- Name: qa_message_reads qa_message_reads_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_message_reads
    ADD CONSTRAINT qa_message_reads_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: qa_messages qa_messages_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_messages
    ADD CONSTRAINT qa_messages_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: qa_rep_submissions qa_rep_submissions_submitted_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_rep_submissions
    ADD CONSTRAINT qa_rep_submissions_submitted_by_fkey FOREIGN KEY (submitted_by) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: qa_rep_submissions qa_rep_submissions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.qa_rep_submissions
    ADD CONSTRAINT qa_rep_submissions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: schools schools_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.schools
    ADD CONSTRAINT schools_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: semester_archives semester_archives_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.semester_archives
    ADD CONSTRAINT semester_archives_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: sessions sessions_offering_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_offering_id_fkey FOREIGN KEY (offering_id) REFERENCES public.course_offerings(offering_id) ON DELETE SET NULL;


--
-- Name: sessions sessions_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: sessions sessions_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.course_units(unit_id);


--
-- Name: sessions sessions_venue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_venue_id_fkey FOREIGN KEY (venue_id) REFERENCES public.venues(venue_id);


--
-- Name: student_device_bindings student_device_bindings_student_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.student_device_bindings
    ADD CONSTRAINT student_device_bindings_student_id_fkey FOREIGN KEY (student_id) REFERENCES public.students_extended(student_id) ON DELETE CASCADE;


--
-- Name: student_device_bindings student_device_bindings_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.student_device_bindings
    ADD CONSTRAINT student_device_bindings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: students_extended students_extended_course_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.students_extended
    ADD CONSTRAINT students_extended_course_id_fkey FOREIGN KEY (course_id) REFERENCES public.courses(course_id);


--
-- Name: students_extended students_extended_offering_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.students_extended
    ADD CONSTRAINT students_extended_offering_id_fkey FOREIGN KEY (offering_id) REFERENCES public.course_offerings(offering_id) ON DELETE SET NULL;


--
-- Name: students_extended students_extended_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.students_extended
    ADD CONSTRAINT students_extended_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: sync_uploads sync_uploads_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.sync_uploads
    ADD CONSTRAINT sync_uploads_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: timetable_slots timetable_slots_lecturer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_lecturer_id_fkey FOREIGN KEY (lecturer_id) REFERENCES public.lecturers(lecturer_id) ON DELETE SET NULL;


--
-- Name: timetable_slots timetable_slots_offering_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_offering_id_fkey FOREIGN KEY (offering_id) REFERENCES public.course_offerings(offering_id) ON DELETE CASCADE;


--
-- Name: timetable_slots timetable_slots_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: timetable_slots timetable_slots_unit_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_unit_id_fkey FOREIGN KEY (unit_id) REFERENCES public.course_units(unit_id) ON DELETE CASCADE;


--
-- Name: timetable_slots timetable_slots_venue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.timetable_slots
    ADD CONSTRAINT timetable_slots_venue_id_fkey FOREIGN KEY (venue_id) REFERENCES public.venues(venue_id) ON DELETE SET NULL;


--
-- Name: user_schools user_schools_school_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.user_schools
    ADD CONSTRAINT user_schools_school_id_fkey FOREIGN KEY (school_id) REFERENCES public.schools(school_id) ON DELETE CASCADE;


--
-- Name: user_schools user_schools_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.user_schools
    ADD CONSTRAINT user_schools_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: user_schools user_schools_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.user_schools
    ADD CONSTRAINT user_schools_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id) ON DELETE CASCADE;


--
-- Name: users users_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: venues venues_department_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.venues
    ADD CONSTRAINT venues_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments(department_id) ON DELETE SET NULL;


--
-- Name: venues venues_school_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.venues
    ADD CONSTRAINT venues_school_id_fkey FOREIGN KEY (school_id) REFERENCES public.schools(school_id) ON DELETE SET NULL;


--
-- Name: venues venues_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: qaat
--

ALTER TABLE ONLY public.venues
    ADD CONSTRAINT venues_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(tenant_id) ON DELETE CASCADE;


--
-- Name: admin_audit_log; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.admin_audit_log ENABLE ROW LEVEL SECURITY;

--
-- Name: app_notifications; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.app_notifications ENABLE ROW LEVEL SECURITY;

--
-- Name: attendance_logs; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.attendance_logs ENABLE ROW LEVEL SECURITY;

--
-- Name: coordinator_delegations; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.coordinator_delegations ENABLE ROW LEVEL SECURITY;

--
-- Name: course_offerings; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.course_offerings ENABLE ROW LEVEL SECURITY;

--
-- Name: course_units; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.course_units ENABLE ROW LEVEL SECURITY;

--
-- Name: courses; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.courses ENABLE ROW LEVEL SECURITY;

--
-- Name: departments; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.departments ENABLE ROW LEVEL SECURITY;

--
-- Name: employee_attendance_days; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.employee_attendance_days ENABLE ROW LEVEL SECURITY;

--
-- Name: employee_attendance_logs; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.employee_attendance_logs ENABLE ROW LEVEL SECURITY;

--
-- Name: employees; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.employees ENABLE ROW LEVEL SECURITY;

--
-- Name: hardware_vault; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.hardware_vault ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturer_assignments; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturer_assignments ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturer_attendance_logs; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturer_attendance_logs ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturer_biometric_templates; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturer_biometric_templates ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturer_daily_codes; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturer_daily_codes ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturer_patrol_logs; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturer_patrol_logs ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturer_presence_claims; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturer_presence_claims ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturer_webauthn_credentials; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturer_webauthn_credentials ENABLE ROW LEVEL SECURITY;

--
-- Name: lecturers; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.lecturers ENABLE ROW LEVEL SECURITY;

--
-- Name: monitor_log_units; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.monitor_log_units ENABLE ROW LEVEL SECURITY;

--
-- Name: attendance_logs no_delete_attendance; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY no_delete_attendance ON public.attendance_logs AS RESTRICTIVE FOR DELETE USING (false);


--
-- Name: admin_audit_log no_delete_audit; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY no_delete_audit ON public.admin_audit_log AS RESTRICTIVE FOR DELETE USING (false);


--
-- Name: attendance_logs no_update_attendance; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY no_update_attendance ON public.attendance_logs AS RESTRICTIVE FOR UPDATE USING (false);


--
-- Name: admin_audit_log no_update_audit; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY no_update_audit ON public.admin_audit_log AS RESTRICTIVE FOR UPDATE USING (false);


--
-- Name: notification_log; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.notification_log ENABLE ROW LEVEL SECURITY;

--
-- Name: notification_recipients; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.notification_recipients ENABLE ROW LEVEL SECURITY;

--
-- Name: offering_unit_schedules; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.offering_unit_schedules ENABLE ROW LEVEL SECURITY;

--
-- Name: patroller_device_bindings; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.patroller_device_bindings ENABLE ROW LEVEL SECURITY;

--
-- Name: patroller_pins; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.patroller_pins ENABLE ROW LEVEL SECURITY;

--
-- Name: qa_message_reads; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.qa_message_reads ENABLE ROW LEVEL SECURITY;

--
-- Name: qa_messages; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.qa_messages ENABLE ROW LEVEL SECURITY;

--
-- Name: qa_rep_submissions; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.qa_rep_submissions ENABLE ROW LEVEL SECURITY;

--
-- Name: scheduled_job_runs; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.scheduled_job_runs ENABLE ROW LEVEL SECURITY;

--
-- Name: schools; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.schools ENABLE ROW LEVEL SECURITY;

--
-- Name: semester_archives; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.semester_archives ENABLE ROW LEVEL SECURITY;

--
-- Name: sessions; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.sessions ENABLE ROW LEVEL SECURITY;

--
-- Name: student_attendance_summary; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.student_attendance_summary ENABLE ROW LEVEL SECURITY;

--
-- Name: student_device_bindings; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.student_device_bindings ENABLE ROW LEVEL SECURITY;

--
-- Name: students_extended; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.students_extended ENABLE ROW LEVEL SECURITY;

--
-- Name: sync_uploads; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.sync_uploads ENABLE ROW LEVEL SECURITY;

--
-- Name: admin_audit_log tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.admin_audit_log USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: app_notifications tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.app_notifications USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: attendance_logs tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.attendance_logs USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: coordinator_delegations tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.coordinator_delegations USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: course_offerings tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.course_offerings USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: course_units tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.course_units USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: courses tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.courses USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: departments tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.departments USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: employee_attendance_days tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.employee_attendance_days USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: employee_attendance_logs tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.employee_attendance_logs USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: employees tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.employees USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: hardware_vault tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.hardware_vault USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturer_assignments tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturer_assignments USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturer_attendance_logs tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturer_attendance_logs USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturer_biometric_templates tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturer_biometric_templates USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturer_daily_codes tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturer_daily_codes USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid)) WITH CHECK ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturer_patrol_logs tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturer_patrol_logs USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturer_presence_claims tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturer_presence_claims USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid)) WITH CHECK ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturer_webauthn_credentials tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturer_webauthn_credentials USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: lecturers tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.lecturers USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: monitor_log_units tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.monitor_log_units USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: notification_log tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.notification_log USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: notification_recipients tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.notification_recipients USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: offering_unit_schedules tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.offering_unit_schedules USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: patroller_device_bindings tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.patroller_device_bindings USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: patroller_pins tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.patroller_pins USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: qa_message_reads tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.qa_message_reads USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: qa_messages tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.qa_messages USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: qa_rep_submissions tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.qa_rep_submissions USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: schools tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.schools USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: semester_archives tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.semester_archives USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: sessions tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.sessions USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: student_attendance_summary tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.student_attendance_summary USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: student_device_bindings tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.student_device_bindings USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid)) WITH CHECK ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: students_extended tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.students_extended USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: sync_uploads tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.sync_uploads USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: timetable_slots tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.timetable_slots USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: user_schools tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.user_schools USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: users tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.users USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: venues tenant_isolation; Type: POLICY; Schema: public; Owner: qaat
--

CREATE POLICY tenant_isolation ON public.venues USING ((tenant_id = (current_setting('app.current_tenant'::text, true))::uuid));


--
-- Name: timetable_slots; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.timetable_slots ENABLE ROW LEVEL SECURITY;

--
-- Name: user_schools; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.user_schools ENABLE ROW LEVEL SECURITY;

--
-- Name: users; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;

--
-- Name: venues; Type: ROW SECURITY; Schema: public; Owner: qaat
--

ALTER TABLE public.venues ENABLE ROW LEVEL SECURITY;

--
-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: pg_database_owner
--

GRANT USAGE ON SCHEMA public TO qaat_app;


--
-- Name: FUNCTION armor(bytea); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.armor(bytea) TO qaat_app;


--
-- Name: FUNCTION armor(bytea, text[], text[]); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.armor(bytea, text[], text[]) TO qaat_app;


--
-- Name: FUNCTION crypt(text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.crypt(text, text) TO qaat_app;


--
-- Name: FUNCTION dearmor(text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.dearmor(text) TO qaat_app;


--
-- Name: FUNCTION decrypt(bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.decrypt(bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION decrypt_iv(bytea, bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.decrypt_iv(bytea, bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION digest(bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.digest(bytea, text) TO qaat_app;


--
-- Name: FUNCTION digest(text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.digest(text, text) TO qaat_app;


--
-- Name: FUNCTION encrypt(bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.encrypt(bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION encrypt_iv(bytea, bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.encrypt_iv(bytea, bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION gen_random_bytes(integer); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.gen_random_bytes(integer) TO qaat_app;


--
-- Name: FUNCTION gen_random_uuid(); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.gen_random_uuid() TO qaat_app;


--
-- Name: FUNCTION gen_salt(text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.gen_salt(text) TO qaat_app;


--
-- Name: FUNCTION gen_salt(text, integer); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.gen_salt(text, integer) TO qaat_app;


--
-- Name: FUNCTION hmac(bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.hmac(bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION hmac(text, text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.hmac(text, text, text) TO qaat_app;


--
-- Name: FUNCTION pgp_armor_headers(text, OUT key text, OUT value text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_armor_headers(text, OUT key text, OUT value text) TO qaat_app;


--
-- Name: FUNCTION pgp_key_id(bytea); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_key_id(bytea) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_decrypt(bytea, bytea); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_decrypt(bytea, bytea) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_decrypt(bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_decrypt(bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_decrypt(bytea, bytea, text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_decrypt(bytea, bytea, text, text) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_decrypt_bytea(bytea, bytea); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_decrypt_bytea(bytea, bytea) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_decrypt_bytea(bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_decrypt_bytea(bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_decrypt_bytea(bytea, bytea, text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_decrypt_bytea(bytea, bytea, text, text) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_encrypt(text, bytea); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_encrypt(text, bytea) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_encrypt(text, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_encrypt(text, bytea, text) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_encrypt_bytea(bytea, bytea); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_encrypt_bytea(bytea, bytea) TO qaat_app;


--
-- Name: FUNCTION pgp_pub_encrypt_bytea(bytea, bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_pub_encrypt_bytea(bytea, bytea, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_decrypt(bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_decrypt(bytea, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_decrypt(bytea, text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_decrypt(bytea, text, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_decrypt_bytea(bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_decrypt_bytea(bytea, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_decrypt_bytea(bytea, text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_decrypt_bytea(bytea, text, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_encrypt(text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_encrypt(text, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_encrypt(text, text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_encrypt(text, text, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_encrypt_bytea(bytea, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_encrypt_bytea(bytea, text) TO qaat_app;


--
-- Name: FUNCTION pgp_sym_encrypt_bytea(bytea, text, text); Type: ACL; Schema: public; Owner: qaat
--

GRANT ALL ON FUNCTION public.pgp_sym_encrypt_bytea(bytea, text, text) TO qaat_app;


--
-- Name: FUNCTION refresh_attendance_summary(p_tenant uuid); Type: ACL; Schema: public; Owner: qaat
--

REVOKE ALL ON FUNCTION public.refresh_attendance_summary(p_tenant uuid) FROM PUBLIC;
GRANT ALL ON FUNCTION public.refresh_attendance_summary(p_tenant uuid) TO qaat_app;


--
-- Name: TABLE admin_audit_log; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.admin_audit_log TO qaat_app;


--
-- Name: TABLE app_notifications; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.app_notifications TO qaat_app;


--
-- Name: TABLE attendance_logs; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.attendance_logs TO qaat_app;


--
-- Name: TABLE coordinator_delegations; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.coordinator_delegations TO qaat_app;


--
-- Name: TABLE course_offerings; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.course_offerings TO qaat_app;


--
-- Name: TABLE course_units; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.course_units TO qaat_app;


--
-- Name: TABLE courses; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.courses TO qaat_app;


--
-- Name: TABLE departments; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.departments TO qaat_app;


--
-- Name: TABLE employee_attendance_days; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.employee_attendance_days TO qaat_app;


--
-- Name: TABLE employee_attendance_logs; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.employee_attendance_logs TO qaat_app;


--
-- Name: TABLE employees; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.employees TO qaat_app;


--
-- Name: TABLE hardware_vault; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.hardware_vault TO qaat_app;


--
-- Name: TABLE lecturer_assignments; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.lecturer_assignments TO qaat_app;


--
-- Name: TABLE lecturer_attendance_logs; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.lecturer_attendance_logs TO qaat_app;


--
-- Name: TABLE lecturer_biometric_templates; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.lecturer_biometric_templates TO qaat_app;


--
-- Name: TABLE lecturer_daily_codes; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.lecturer_daily_codes TO qaat_app;


--
-- Name: TABLE lecturer_patrol_logs; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.lecturer_patrol_logs TO qaat_app;


--
-- Name: TABLE lecturer_presence_claims; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT ON TABLE public.lecturer_presence_claims TO qaat_app;


--
-- Name: TABLE lecturer_webauthn_credentials; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.lecturer_webauthn_credentials TO qaat_app;


--
-- Name: TABLE lecturers; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.lecturers TO qaat_app;


--
-- Name: TABLE monitor_log_units; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.monitor_log_units TO qaat_app;


--
-- Name: TABLE notification_log; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.notification_log TO qaat_app;


--
-- Name: TABLE notification_recipients; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.notification_recipients TO qaat_app;


--
-- Name: TABLE offering_unit_schedules; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.offering_unit_schedules TO qaat_app;


--
-- Name: TABLE patroller_device_bindings; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.patroller_device_bindings TO qaat_app;


--
-- Name: TABLE patroller_pins; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.patroller_pins TO qaat_app;


--
-- Name: TABLE qa_message_reads; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.qa_message_reads TO qaat_app;


--
-- Name: TABLE qa_messages; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.qa_messages TO qaat_app;


--
-- Name: TABLE qa_rep_submissions; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.qa_rep_submissions TO qaat_app;


--
-- Name: TABLE scheduled_job_runs; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.scheduled_job_runs TO qaat_app;


--
-- Name: TABLE schema_migrations; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.schema_migrations TO qaat_app;


--
-- Name: TABLE schools; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.schools TO qaat_app;


--
-- Name: TABLE semester_archives; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.semester_archives TO qaat_app;


--
-- Name: TABLE sessions; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.sessions TO qaat_app;


--
-- Name: TABLE student_attendance_summary; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.student_attendance_summary TO qaat_app;


--
-- Name: TABLE student_device_bindings; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.student_device_bindings TO qaat_app;


--
-- Name: TABLE students_extended; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.students_extended TO qaat_app;


--
-- Name: TABLE sync_uploads; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.sync_uploads TO qaat_app;


--
-- Name: TABLE tenants; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.tenants TO qaat_app;


--
-- Name: TABLE timetable_slots; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.timetable_slots TO qaat_app;


--
-- Name: TABLE user_schools; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE public.user_schools TO qaat_app;


--
-- Name: TABLE users; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.users TO qaat_app;


--
-- Name: TABLE venues; Type: ACL; Schema: public; Owner: qaat
--

GRANT SELECT,INSERT,UPDATE ON TABLE public.venues TO qaat_app;


--
-- Name: DEFAULT PRIVILEGES FOR SEQUENCES; Type: DEFAULT ACL; Schema: public; Owner: qaat
--

ALTER DEFAULT PRIVILEGES FOR ROLE qaat IN SCHEMA public GRANT SELECT,USAGE ON SEQUENCES  TO qaat_app;


--
-- Name: DEFAULT PRIVILEGES FOR TABLES; Type: DEFAULT ACL; Schema: public; Owner: qaat
--

ALTER DEFAULT PRIVILEGES FOR ROLE qaat IN SCHEMA public GRANT SELECT,INSERT,UPDATE ON TABLES  TO qaat_app;


--
-- PostgreSQL database dump complete
--

\unrestrict hAxV8ecjA3LpYF7W35XEL3Co3ztrguG5fwPwHHYjUvAvggieUeNtuE9Mcstf8Ap

