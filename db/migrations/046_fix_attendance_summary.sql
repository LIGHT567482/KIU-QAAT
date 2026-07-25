-- 046: Fix refresh_attendance_summary — sessions_held was undercounted.
--
-- BUG: the old function derived the student from attendance_logs and GROUPed BY
-- al.student_id, so sessions_held (COUNT(DISTINCT s.session_id) within that group)
-- only ever counted the sessions the student ATTENDED. Result: attendance_percentage
-- was always ~100% for anyone present ≥1 time, absences never lowered it, and students
-- who never attended did not appear at all — making eligibility (≥ threshold) and the
-- chronic-absentee signal meaningless.
--
-- FIX: enumerate every ACTIVE enrolled student × their course's units (the same
-- mapping the manifest roster uses: students_extended.course_id = course_units.course_id),
-- count sessions_held = ALL CLOSED/AUTO_CLOSED sessions of that unit, and
-- sessions_attended = the subset the student attended (LEFT JOIN → 0 if none). Absentees
-- now appear with a real, sub-100% percentage.

CREATE OR REPLACE FUNCTION public.refresh_attendance_summary(p_tenant uuid)
 RETURNS void
 LANGUAGE plpgsql
 SECURITY DEFINER
AS $function$
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
$function$;
