package upanel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch_export(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token secret-admin" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/attendance/export/":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"lists": []map[string]any{{
					"id": "L1", "whoTaught": "Dr Ada", "lecturerUid": "12", "room": "LH1",
					"courseUnitName": "Algorithms", "year": "2", "sem": "1", "program": "day",
					"courses": []string{"CSC2101"},
				}},
				"sessions": []map[string]any{{
					"id": "S1", "listId": "L1", "startTime": "2026-08-21T09:00:00Z", "endTime": "2026-08-21T11:00:00Z",
				}},
				"records": []map[string]any{{
					"id": "S1_uid-ann", "sessionId": "S1", "studentId": "uid-ann",
					"listId": "L1", "course": "CSC2101", "present": true, "verified": true,
					"timestamp": "2026-08-21T10:00:00Z",
				}},
			})
		case "/api/attendance/students/":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "uid-ann", "name": "Ann", "registrationNumber": "2020/001"},
				{"id": "uid-ben", "name": "Ben", "registrationNumber": "2020/002"},
			})
		case "/api/attendance/sign-ins/":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "si1", "listId": "L1", "studentId": "uid-ann", "studentName": "Ann", "registrationNumber": "2020/001", "course": "CSC2101", "signedInAt": "2026-08-01T08:00:00Z"},
				{"id": "si2", "listId": "L1", "studentId": "uid-ben", "studentName": "Ben", "registrationNumber": "2020/002", "course": "CSC2101", "signedInAt": "2026-08-01T08:00:00Z"},
			})
		case "/api/campus/presence/":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": "p1", "adminUid": "83", "kind": "arrival",
				"capturedAt": "2026-08-21T08:00:00Z", "displayName": "QA Admin",
				"staffNumber": "ADM-1",
			}})
		case "/api/accounts/lecturers/":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": "12", "fullName": "Dr Ada", "staffNumber": "KIU-0001", "email": "ada@kiu.ac.ug",
			}})
		case "/api/accounts/staff-numbers/":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": "KIU-0001", "uid": "12", "staffNumber": "KIU-0001", "fullName": "Dr Ada",
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("UPANEL_API_URL", srv.URL)
	t.Setenv("UPANEL_API_TOKEN", "secret-admin")

	got, err := Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Configured || got.FetchedVia != "export" {
		t.Fatalf("%+v", got)
	}
	if got.StudentCount != 2 || got.LecturerCount != 1 || got.AdminCount != 1 {
		t.Fatalf("counts student=%d lecturer=%d admin=%d payload=%+v",
			got.StudentCount, got.LecturerCount, got.AdminCount, got)
	}
	var present, absent Record
	var lecturer, admin Record
	for _, r := range got.Records {
		switch r.Kind {
		case KindStudent:
			if r.Present {
				present = r
			} else {
				absent = r
			}
		case KindLecturer:
			lecturer = r
		case KindAdmin:
			admin = r
		}
	}
	if present.StudentID != "2020/001" || present.FullName != "Ann" || present.Lecturer != "Dr Ada" || present.Room != "LH1" {
		t.Fatalf("present %+v", present)
	}
	if present.Course != "CSC2101" || present.UnitName != "Algorithms" || present.Session != "Day" || present.Semester != "1" || present.Year != "2" {
		t.Fatalf("qaat fields %+v", present)
	}
	if absent.StudentID != "2020/002" || absent.FullName != "Ben" || absent.EventType != "ABSENT" || absent.Present {
		t.Fatalf("absent %+v", absent)
	}
	if lecturer.PersonName != "Dr Ada" || lecturer.LecturerName != "Dr Ada" || lecturer.Room != "LH1" || lecturer.EventType != "LECTURE" || lecturer.Session != "Day" {
		t.Fatalf("lecturer %+v", lecturer)
	}
	if lecturer.StaffID != "KIU-0001" || lecturer.PersonID != "KIU-0001" {
		t.Fatalf("lecturer staff id %+v", lecturer)
	}
	if admin.StaffID != "ADM-1" || admin.EventType != "IN" || admin.FullName != "QA Admin" {
		t.Fatalf("admin %+v", admin)
	}
}

func TestFetch_collectionsFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/attendance/export/", func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})
	mux.HandleFunc("/api/attendance/lists/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": "L1", "whoTaught": "Dr Ada", "room": "LH1", "year": "1", "sem": "2",
			"lecturerSignedAt": "2026-08-20T07:00:00Z",
		}})
	})
	mux.HandleFunc("/api/attendance/sessions/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{})
	})
	mux.HandleFunc("/api/attendance/records/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": "S1_x", "sessionId": "S1", "studentId": "x", "present": false,
		}})
	})
	mux.HandleFunc("/api/campus/presence/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": "p2", "adminUid": "9", "kind": "departure",
			"capturedAt": "2026-08-21T17:00:00Z", "displayName": "Bursar",
		}})
	})
	mux.HandleFunc("/api/accounts/lecturers/", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": "99", "fullName": "Dr Ada", "staffNumber": "KIU-0001",
		}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("UPANEL_API_URL", srv.URL)
	t.Setenv("UPANEL_API_TOKEN", "secret-admin")

	got, err := Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.FetchedVia != "collections" || got.StudentCount != 1 || got.LecturerCount != 1 || got.AdminCount != 1 {
		t.Fatalf("%+v", got)
	}
	if got.Records[0].Kind == KindStudent && got.Records[0].Present {
		t.Fatal("expected absent student")
	}
	var lecturer, admin Record
	for _, r := range got.Records {
		switch r.Kind {
		case KindLecturer:
			lecturer = r
		case KindAdmin:
			admin = r
		}
	}
	if lecturer.EventType != "LECTURER_SIGN" || lecturer.PersonName != "Dr Ada" {
		t.Fatalf("lecturer %+v", lecturer)
	}
	if lecturer.StaffID != "KIU-0001" || lecturer.PersonID != "KIU-0001" {
		t.Fatalf("lecturer staff from name %+v", lecturer)
	}
	if admin.EventType != "OUT" || admin.PersonID != "9" {
		t.Fatalf("admin %+v", admin)
	}
}

func TestFetch_missingToken(t *testing.T) {
	t.Setenv("UPANEL_API_TOKEN", "")
	t.Setenv("UPANEL_API_URL", "http://127.0.0.1:1")
	got, err := Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Configured {
		t.Fatal("should not be configured")
	}
}

func TestLecturerIdentities_uidAndName(t *testing.T) {
	ids := lecturerIdentities(
		[]map[string]any{{"id": "12", "fullName": "Dr Ada", "staffNumber": "kiu-0001"}},
		[]map[string]any{{"id": "KIU-0001", "uid": "12", "staffNumber": "KIU-0001", "fullName": "Dr Ada"}},
	)
	staff, name := resolveLecturer("12", "Dr Ada", ids)
	if staff != "KIU-0001" || name != "Dr Ada" {
		t.Fatalf("uid lookup staff=%s name=%s", staff, name)
	}
	staff, name = resolveLecturer("", "Dr Ada", ids)
	if staff != "KIU-0001" || name != "Dr Ada" {
		t.Fatalf("name lookup staff=%s name=%s", staff, name)
	}
	staff, name = resolveLecturer("KIU-0001", "", ids)
	if staff != "KIU-0001" {
		t.Fatalf("staff-number lookup staff=%s name=%s", staff, name)
	}
}
