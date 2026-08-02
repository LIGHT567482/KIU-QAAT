import { useMemo } from 'react'
import { api } from '../lib/api'
import { useQuery } from '../lib/useApi'

/**
 * The department + college/school pair, picked from what the ADMIN has actually created.
 *
 * WHY THIS EXISTS. Department and school used to be free-text boxes with a `datalist` of whatever
 * strings happened to already be in the data. Every screen that scopes by org unit — the HOD's
 * lecturers, the dean's school, a QA rep's reports, the DQA's DEPARTMENT broadcasts — matches those
 * strings against `courses.department` / `courses.school` by NAME. So one typo, one "Comp. Science"
 * against "Computer Science", and the account silently sees nothing at all: the queries return an
 * empty set rather than an error. Typing was the bug. Choosing cannot be.
 *
 * DEPARTMENT DRIVES SCHOOL. Departments sit under schools, so picking a department fills the school
 * in for you and locks it — the pair can never disagree. Picking a school first is still allowed and
 * simply narrows the department list.
 *
 * EXCEPT WHERE IT DOESN'T. Support departments — Finance, ICT, Library, admissions — sit under no
 * faculty at all (`departments.school_id IS NULL`, migration 066). Choosing one clears the school
 * and says so, rather than leaving a stale school attached to a department that has none.
 */

export interface OrgSchool { school_id: string; name: string }
export interface OrgDepartment { department_id: string; school_id: string; name: string; kind: string }

/** Load the tenant's org lists once; every picker on a page shares the result. */
export function useOrg(tenantId: string) {
  const schools = useQuery<OrgSchool[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/schools`), [tenantId])
  const departments = useQuery<OrgDepartment[]>(() => api.get(`/api/v1/admin/tenants/${tenantId}/departments`), [tenantId])
  return {
    schools: schools.data ?? [],
    departments: departments.data ?? [],
    loading: schools.status === 'loading' || departments.status === 'loading',
    error: schools.message || departments.message,
    refetch: () => { schools.refetch(); departments.refetch() },
  }
}

interface Props {
  schools: OrgSchool[]
  departments: OrgDepartment[]
  /** Current values, as NAMES — that is what users.department / lecturers.department store. */
  department: string
  school: string
  onChange: (next: { department: string; school: string }) => void
  requireDepartment?: boolean
  requireSchool?: boolean
  /** Hidden entirely when a role has no use for it (e.g. a lecturer has no school field). */
  showSchool?: boolean
  hint?: string
  disabled?: boolean
}

export function OrgPicker({
  schools, departments, department, school, onChange,
  requireDepartment = false, requireSchool = false, showSchool = true, hint, disabled = false,
}: Props) {
  // The chosen department's own record, which is what decides the school.
  const chosen = useMemo(
    () => departments.find(d => d.name === department),
    [departments, department],
  )
  const standalone = !!chosen && !chosen.school_id

  // A school chosen FIRST narrows the departments; otherwise every department is offered, each
  // labelled with its school so two same-named departments in different faculties stay distinct.
  const visibleDepartments = useMemo(() => {
    if (!school) return departments
    const sid = schools.find(s => s.name === school)?.school_id
    if (!sid) return departments
    // Support departments belong to no school, so a school filter must exclude them rather than
    // silently listing them under a faculty they are not part of.
    return departments.filter(d => d.school_id === sid)
  }, [departments, schools, school])

  function pickDepartment(name: string) {
    if (!name) { onChange({ department: '', school }); return }
    const d = departments.find(x => x.name === name)
    // The department's school wins, always — including when it has none, which CLEARS the field.
    // Leaving the previous school behind would attach Finance to a faculty it does not sit under.
    const s = d?.school_id ? (schools.find(x => x.school_id === d.school_id)?.name ?? '') : ''
    onChange({ department: name, school: s })
  }

  function pickSchool(name: string) {
    // Changing school invalidates a department that does not belong to it.
    const sid = schools.find(x => x.name === name)?.school_id
    const keep = chosen && sid && chosen.school_id === sid
    onChange({ department: keep ? department : '', school: name })
  }

  const noOrg = schools.length === 0 && departments.length === 0

  return (
    <>
      <label>
        <div style={labelStyle}>
          Department {requireDepartment ? <span style={{ color: '#b91c1c' }}>*</span> : '(optional)'}
        </div>
        <select
          value={department} disabled={disabled || noOrg}
          onChange={e => pickDepartment(e.target.value)}
          style={{ ...selectStyle, borderColor: requireDepartment && !department ? '#fca5a5' : '#e2e8f0' }}
        >
          <option value="">— none —</option>
          {visibleDepartments.map(d => (
            <option key={d.department_id} value={d.name}>
              {d.name}{d.school_id ? '' : ' (no school)'}
            </option>
          ))}
        </select>
        {hint && <div style={hintStyle}>{hint}</div>}
      </label>

      {showSchool && (
        <label>
          <div style={labelStyle}>
            College / School {requireSchool ? <span style={{ color: '#b91c1c' }}>*</span> : '(optional)'}
          </div>
          <select
            value={school}
            // Locked once a department has decided it — the two must not be able to disagree.
            disabled={disabled || noOrg || !!chosen}
            onChange={e => pickSchool(e.target.value)}
            style={{
              ...selectStyle,
              borderColor: requireSchool && !school ? '#fca5a5' : '#e2e8f0',
              background: chosen ? '#f1f5f9' : undefined,
            }}
          >
            <option value="">— none —</option>
            {schools.map(s => <option key={s.school_id} value={s.name}>{s.name}</option>)}
          </select>
          {chosen && (
            <div style={hintStyle}>
              {standalone
                ? `${chosen.name} is a support department — it sits under no college or school.`
                : `Set automatically from ${chosen.name}. Change the department to change it.`}
            </div>
          )}
        </label>
      )}

      {noOrg && (
        <div style={{ gridColumn: '1 / -1', fontSize: 12, color: '#b45309' }}>
          No colleges/schools or departments have been created yet. Add them under{' '}
          <strong>Schools &amp; Departments</strong> first — every org-scoped role is bounded by one,
          and an account with a unit that does not exist matches nothing.
        </div>
      )}
    </>
  )
}

const labelStyle: React.CSSProperties = { fontSize: 12, color: 'var(--muted)', marginBottom: 4 }
const hintStyle: React.CSSProperties = { fontSize: 11, color: 'var(--muted)', marginTop: 3 }
const selectStyle: React.CSSProperties = {
  width: '100%', padding: '8px 10px', borderRadius: 6, border: '1px solid #e2e8f0',
  fontSize: 14, boxSizing: 'border-box', background: '#fff', color: '#1e293b',
}
