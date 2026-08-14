package handlers

// WHAT THE HEAD OF A DEPARTMENT IS CALLED.
//
// KIU has two kinds of department and they are not led by the same office. An academic department
// — Computer Science, and its peers — is run by a HEAD OF DEPARTMENT. A support department —
// Library, ICT, Bursary, Finance, Admissions — is run by a DIRECTOR, in exactly the way quality
// assurance is run by the Director of Quality Assurance rather than by a "head of QA".
//
// The system had one word for both, so the org chart introduced the Director of the Library as an
// HOD. That is not a cosmetic slip: these screens are how the institution's own management layer
// is presented back to it, and naming somebody's office wrongly is the kind of error a director
// notices immediately and an engineer never does.
//
// `departments.kind` already records which is which (ACADEMIC | SUPPORT), populated for every row
// and defaulted for older ones, so nothing new has to be captured — the title is derived from a
// fact the org chart already holds. Derived HERE, once, so every screen agrees: the alternative is
// each page deciding for itself and drifting, which is how the label came to be wrong in the first
// place.
//
// The RBAC role is untouched and stays HOD. A director's authority over their department is the
// same authority a head of department has over theirs — one department, its staff and its
// records — so a parallel role would be an identical permission set under a second name, and two
// roles that mean the same thing eventually diverge by accident. What changes is what we CALL the
// person holding it.

// headTitleFor returns the office title for a department of the given kind. Unknown or empty kinds
// fall back to the academic title, matching the column's own default: a department whose kind was
// never set is far more likely to be academic than to be the library.
func headTitleFor(kind string) string {
	if kind == "SUPPORT" {
		return "Director"
	}
	return "Head of Department"
}
