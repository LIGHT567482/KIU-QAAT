package handlers

// Minimal XLSX (Excel) read/write using only the Go standard library (archive/zip
// + encoding/xml) — no third-party dependency. Enough for the student roster
// import/export: a single sheet of string cells. Lets the admin import/export in
// either CSV or Excel, their choice (next.txt #6).

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// looksXLSX reports whether the bytes are a ZIP (XLSX) rather than CSV/text.
func looksXLSX(b []byte) bool {
	return len(b) > 4 && b[0] == 'P' && b[1] == 'K' && b[2] == 0x03 && b[3] == 0x04
}

// parseXLSX returns the first worksheet as rows of string cells.
func parseXLSX(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid xlsx: %w", err)
	}

	// shared strings (optional).
	var shared []string
	if f := findFile(zr, "xl/sharedStrings.xml"); f != nil {
		shared = parseSharedStrings(f)
	}

	// first worksheet (sheet1.xml is the conventional first sheet).
	sheet := findFile(zr, "xl/worksheets/sheet1.xml")
	if sheet == nil {
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "xl/worksheets/") && strings.HasSuffix(f.Name, ".xml") {
				sheet = f
				break
			}
		}
	}
	if sheet == nil {
		return nil, fmt.Errorf("no worksheet found")
	}
	return parseSheet(sheet, shared)
}

func findFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func parseSharedStrings(f *zip.File) []string {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	var out []string
	dec := xml.NewDecoder(rc)
	var cur strings.Builder
	inSI, inT := false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				inSI = true
				cur.Reset()
			case "t":
				inT = true
			}
		case xml.CharData:
			if inSI && inT {
				cur.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inT = false
			case "si":
				out = append(out, cur.String())
				inSI = false
			}
		}
	}
	return out
}

func parseSheet(f *zip.File, shared []string) ([][]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var rows [][]string
	var row []string
	var cellType, cellRef string
	var val strings.Builder
	inV, inT := false, false

	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				row = []string{}
			case "c":
				cellType, cellRef = "", ""
				for _, a := range t.Attr {
					if a.Name.Local == "t" {
						cellType = a.Value
					}
					if a.Name.Local == "r" {
						cellRef = a.Value
					}
				}
				val.Reset()
			case "v":
				inV = true
			case "t":
				inT = true
			}
		case xml.CharData:
			if inV || inT {
				val.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inV = false
			case "t":
				inT = false
			case "c":
				v := val.String()
				if cellType == "s" { // shared string index
					if idx := atoiSafe(v); idx >= 0 && idx < len(shared) {
						v = shared[idx]
					}
				}
				// place the cell at its column index so gaps don't shift columns.
				ci := colIndex(cellRef)
				if ci < 0 {
					ci = len(row)
				}
				for len(row) <= ci {
					row = append(row, "")
				}
				row[ci] = v
			case "row":
				rows = append(rows, row)
			}
		}
	}
	return rows, nil
}

// colIndex turns a cell ref like "C12" into the 0-based column index (2).
func colIndex(ref string) int {
	n := 0
	has := false
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			n = n*26 + int(ch-'A'+1)
			has = true
		} else if ch >= 'a' && ch <= 'z' {
			n = n*26 + int(ch-'a'+1)
			has = true
		} else {
			break
		}
	}
	if !has {
		return -1
	}
	return n - 1
}

// buildXLSX writes a single-sheet workbook from rows of strings (row 0 = header).
func buildXLSX(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	write := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, content)
		return err
	}

	if err := write("[Content_Types].xml",
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
			`<Default Extension="xml" ContentType="application/xml"/>`+
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`+
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`+
			`</Types>`); err != nil {
		return nil, err
	}
	if err := write("_rels/.rels",
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>`+
			`</Relationships>`); err != nil {
		return nil, err
	}
	if err := write("xl/workbook.xml",
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`+
			`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`); err != nil {
		return nil, err
	}
	if err := write("xl/_rels/workbook.xml.rels",
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>`+
			`</Relationships>`); err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for ri, row := range rows {
		fmt.Fprintf(&sb, `<row r="%d">`, ri+1)
		for ci, cell := range row {
			ref := fmt.Sprintf("%s%d", colName(ci), ri+1)
			sb.WriteString(`<c r="` + ref + `" t="inlineStr"><is><t xml:space="preserve">`)
			xml.EscapeText(&sb, []byte(cell)) //nolint:errcheck
			sb.WriteString(`</t></is></c>`)
		}
		sb.WriteString(`</row>`)
	}
	sb.WriteString(`</sheetData></worksheet>`)
	if err := write("xl/worksheets/sheet1.xml", sb.String()); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportStudentsXLSX streams the tenant's students as an .xlsx (next.txt #6).
// GET /api/v1/admin/tenants/{tenant_id}/students/export.xlsx
func ExportStudentsXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		// Optional filters (any combination): the admin can export e.g. only
		// Computer Science, or only Year-3 Sem-1 of a level, or one course unit.
		where := "se.tenant_id = $1"
		args := []interface{}{tenantID}
		add := func(clause string, val string) {
			if val == "" {
				return
			}
			args = append(args, val)
			where += fmt.Sprintf(" AND %s = $%d", clause, len(args))
		}
		q := r.URL.Query()
		add("se.course_id", q.Get("course_id"))
		add("se.level", q.Get("level"))
		add("se.current_year::text", q.Get("study_year"))
		add("se.semester::text", q.Get("semester"))
		add("se.intake_session", q.Get("intake"))
		add("se.offering_id::text", q.Get("offering_id"))
		// unit_id → resolve the unit's course/year/semester/level and match students.
		if unitID := q.Get("unit_id"); unitID != "" {
			args = append(args, unitID)
			where += fmt.Sprintf(`
			  AND se.course_id    = (SELECT course_id FROM course_units WHERE unit_id = $%d AND tenant_id = $1)
			  AND se.current_year = (SELECT year      FROM course_units WHERE unit_id = $%d AND tenant_id = $1)
			  AND se.semester     = (SELECT semester  FROM course_units WHERE unit_id = $%d AND tenant_id = $1)`,
				len(args), len(args), len(args))
		}
		rows, err := adminPool.Query(r.Context(), `
			SELECT se.student_id, se.full_name, se.email, se.course_id,
			       COALESCE(c.name,''), COALESCE(se.academic_year,''),
			       COALESCE(se.current_year,0), COALESCE(se.semester,0),
			       COALESCE(se.intake_session,''), se.enrollment_status::text
			FROM students_extended se
			LEFT JOIN courses c ON c.course_id = se.course_id AND c.tenant_id = se.tenant_id
			WHERE `+where+`
			ORDER BY se.full_name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := [][]string{{"student_id", "full_name", "email", "course_id", "course_name", "academic_year", "current_year", "semester", "intake_session", "enrollment_status"}}
		for rows.Next() {
			var sid, name, email, cid, cname, ay, intake, statusS string
			var yr, sem int
			rows.Scan(&sid, &name, &email, &cid, &cname, &ay, &yr, &sem, &intake, &statusS) //nolint:errcheck
			out = append(out, []string{sid, name, email, cid, cname, ay, itoa(yr), itoa(sem), intake, statusS})
		}

		xlsx, err := buildXLSX(out)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="students.xlsx"`)
		_, _ = w.Write(xlsx)
	}
}

func itoa(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

// colName turns a 0-based column index into spreadsheet letters (0 -> A, 26 -> AA).
func colName(i int) string {
	name := ""
	i++
	for i > 0 {
		i--
		name = string(rune('A'+i%26)) + name
		i /= 26
	}
	return name
}
