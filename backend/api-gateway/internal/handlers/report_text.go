package handlers

// Text encoding for generated reports.
//
// THE BUG THIS EXISTS FOR. Every PDF this system produced rendered an em dash as `â€"`, a middle
// dot as `Â·` and an accented name as gibberish. It looked like a font problem. It was an encoding
// one, and it was in our code, not the library's.
//
// fpdf's built-in fonts (Helvetica, Times, Courier — the ones you get without shipping a font
// file) are single-byte Latin-1/cp1252 fonts. Handed a Go string, fpdf writes its BYTES and the
// font draws one glyph per byte. Go strings are UTF-8, so an em dash — U+2014, bytes E2 80 94 —
// arrives as three bytes and is drawn as three glyphs: cp1252 E2 is 'â', 80 is '€', 94 is '"'.
//
//	"KIU QAAT — Session Audit"  →  "KIU QAAT â€" Session Audit"
//
// Which is precisely what people were reading. The dash is not corrupt and the file is not
// damaged; the bytes were simply never converted to the encoding the font speaks.
//
// TWO MORE FAULTS FALL OUT OF THE SAME PLACE, and both are fixed by converting EARLY — before the
// text is measured or trimmed — which is why [pdfEncoder] exists rather than a translator called
// at each CellFormat:
//
//  1. pdf.GetStringWidth counted UTF-8 bytes, so any accented text measured wider than it draws
//     and columns were clipped that had room to spare.
//  2. The column clipper trimmed with cell[:len(cell)-1], one BYTE at a time. Landing mid-rune
//     left a half-character at the end of the cell — a fresh piece of mojibake produced by the
//     code meant to make things fit. After conversion the text is single-byte, so a byte is a
//     character and the same loop is exactly right.
//
// CSV IS A SEPARATE PROBLEM WITH THE SAME SYMPTOM. A UTF-8 .csv is correct on the wire, but Excel
// opens a downloaded file as the system codepage unless the bytes start with a UTF-8 BOM — so the
// same em dash reappeared as `â€"` in the one program most people open these in. See [writeCSVBOM].

import (
	"io"
	"strings"

	"github.com/go-pdf/fpdf"
)

// pdfASCIIFallback rewrites the characters cp1252 has no room for, BEFORE translation.
//
// fpdf's translator turns anything it cannot map into '.', so without this an arrow becomes a full
// stop — which reads as a typo rather than as missing information. A patrol remark saying
// "LR3 → LR7" would arrive as "LR3 . LR7", quietly reversing nothing and explaining nothing.
//
// Only characters this system actually emits are listed. Anything else still falls through to the
// translator's '.', which is a visible marker rather than a silent deletion.
var pdfASCIIFallback = strings.NewReplacer(
	"→", "->", "←", "<-", "⟶", "->", "⇒", "=>",
	"≥", ">=", "≤", "<=", "≠", "!=",
	"✓", "Yes", "✔", "Yes", "✗", "No", "✘", "No",
	"⟳", "", "🎓", "", "📍", "", "⎋", "", "✕", "x", "✎", "",
	// Non-breaking and thin spaces read as a normal space; cp1252 has A0, but a plain space
	// behaves better for a column that is about to be width-measured and clipped.
	" ", " ", " ", " ", " ", " ",
)

// pdfEncoder returns the function that prepares text for [pdf]'s core fonts.
//
// Call it on EVERY string that reaches the PDF — headings, cells, headers, footers — and call it
// before measuring or trimming. The returned function is cheap; the translator is built once.
//
// If the report ever needs a script cp1252 cannot express, the fix is not here: it is
// pdf.AddUTF8Font with an embedded TTF, after which the translator should be dropped entirely.
func pdfEncoder(pdf *fpdf.Fpdf) func(string) string {
	tr := pdf.UnicodeTranslatorFromDescriptor("") // "" = cp1252, fpdf's default core encoding
	return func(s string) string { return tr(pdfASCIIFallback.Replace(s)) }
}

// utf8BOM is what tells Excel the bytes that follow are UTF-8.
//
// THE TRADE-OFF, STATED PLAINLY, because it is not free. Excel opens a downloaded .csv as the
// system codepage unless the file starts with this, which is how a correctly-encoded UTF-8 export
// still showed "â€"" on a marker's screen. But a BOM is not universally transparent: Go's own
// encoding/csv does NOT strip it, nor does pandas without encoding="utf-8-sig", nor psql \copy —
// they all see it glued to the first header, as "<BOM>student_id".
//
// It goes on anyway, on every CSV, because of who opens these. They are dashboard downloads that a
// QA officer, a dean or a marker opens in Excel; that is the whole reason the export button exists.
// A first header cell that a script must trim is a small, visible, fixable problem. Text that
// silently renders as gibberish in the one program everybody uses is neither small nor visible.
//
// And it goes on EVERY CSV rather than only those containing non-ASCII. Conditioning it on the
// content would leave an endpoint that emits a BOM on Tuesday and not on Monday, depending on
// whether a student with an accent in their name happened to be in the result — a parser built and
// tested against one would break later, on data, for no reason its author could see.
//
// Consumers should skip it: strings.TrimPrefix(s, "\uFEFF") in Go, encoding="utf-8-sig" in Python.
const utf8BOM = "\xEF\xBB\xBF"

// writeCSVBOM emits the BOM. Call it once, immediately before the first csv.Writer write, on every
// CSV response the system serves.
func writeCSVBOM(w io.Writer) { _, _ = io.WriteString(w, utf8BOM) }

// trimBOM removes a leading UTF-8 BOM. What a reader of these files should do.
func trimBOM(s string) string { return strings.TrimPrefix(s, "\uFEFF") }
