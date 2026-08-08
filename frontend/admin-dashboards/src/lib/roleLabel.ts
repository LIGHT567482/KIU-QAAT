/**
 * What each role is CALLED, as distinct from what it is stored as.
 *
 * The stored value of the monitor role is still `QA_PATROLLER`. Renaming a database enum means
 * rewriting every row that references it, invalidating every signed token that carries it, and
 * shipping a new build to every handset already in the field — three ways to lock people out of a
 * live system in exchange for a word. The word is what people actually see, so the word is what
 * changes here, in one place, and every screen reads it from here.
 *
 * Anything not listed falls back to the underscore-free form, which is right for ADMIN, VC, DEAN
 * and the rest.
 */
const LABELS: Record<string, string> = {
  QA_PATROLLER:      'QA Monitor',
  QA_OFFICER:        'QA Officer',
  QA_DEPT_REP:       'QA Dept Rep',
  QA_SCHOOL_HANDLER: 'QA School Handler',
  DQA_DIRECTOR:      'DQA Director',
  HOD:               'Head of Department',
  TLC:               'Teaching & Learning Centre',
}

/** Title-case display name for a role, e.g. "QA Monitor". */
export function roleLabel(role: string): string {
  return LABELS[role] ?? role.replace(/_/g, ' ')
}

/** Lower-case, for the middle of a sentence: "a department is required for a qa monitor". */
export function roleLabelLower(role: string): string {
  return roleLabel(role).toLowerCase()
}
