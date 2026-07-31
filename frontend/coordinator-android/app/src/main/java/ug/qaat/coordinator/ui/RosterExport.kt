package ug.qaat.coordinator.ui

import android.content.Context
import android.graphics.Paint
import android.graphics.pdf.PdfDocument
import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.platform.LocalContext
import java.io.ByteArrayOutputStream

/** One roster row for export: registration number, full name, present/absent. */
data class RosterLine(val regNo: String, val name: String, val present: Boolean)

/** CSV with exactly the three agreed columns. Opens directly in Excel/Google Sheets. */
fun rosterCsv(rows: List<RosterLine>): ByteArray {
    fun cell(s: String): String =
        if (s.contains(',') || s.contains('"') || s.contains('\n')) "\"${s.replace("\"", "\"\"")}\"" else s
    val sb = StringBuilder("Reg no,Student name,Status\n")
    rows.forEach { sb.append(cell(it.regNo)).append(',').append(cell(it.name)).append(',')
        .append(if (it.present) "Present" else "Absent").append('\n') }
    return sb.toString().toByteArray(Charsets.UTF_8)
}

/** A simple A4 PDF table with the same three columns, paginated. Uses only android.graphics. */
fun rosterPdf(title: String, rows: List<RosterLine>): ByteArray {
    val doc = PdfDocument()
    val pageW = 595; val pageH = 842; val margin = 36f
    val head = Paint().apply { textSize = 13f; isFakeBoldText = true }
    val cell = Paint().apply { textSize = 11f }
    val present = Paint().apply { textSize = 11f; isFakeBoldText = true; color = 0xFF15803D.toInt() }
    val absent = Paint().apply { textSize = 11f; isFakeBoldText = true; color = 0xFFB91C1C.toInt() }
    val titlePaint = Paint().apply { textSize = 16f; isFakeBoldText = true }
    val xReg = margin; val xName = margin + 130f; val xStatus = pageW - margin - 70f
    val rowH = 20f
    var page = 1; var y = 0f
    var pg = doc.startPage(PdfDocument.PageInfo.Builder(pageW, pageH, page).create())
    fun header(c: android.graphics.Canvas) {
        c.drawText(title, margin, margin + 6f, titlePaint)
        y = margin + 34f
        c.drawText("Reg no", xReg, y, head); c.drawText("Student name", xName, y, head); c.drawText("Status", xStatus, y, head)
        y += 6f; c.drawLine(margin, y, pageW - margin, y, cell); y += rowH
    }
    header(pg.canvas)
    for (r in rows) {
        if (y > pageH - margin) {
            doc.finishPage(pg); page++
            pg = doc.startPage(PdfDocument.PageInfo.Builder(pageW, pageH, page).create()); header(pg.canvas)
        }
        val c = pg.canvas
        c.drawText(r.regNo.take(20), xReg, y, cell)
        c.drawText(r.name.take(28), xName, y, cell)
        if (r.present) c.drawText("Present", xStatus, y, present) else c.drawText("Absent", xStatus, y, absent)
        y += rowH
    }
    doc.finishPage(pg)
    val out = ByteArrayOutputStream(); doc.writeTo(out); doc.close(); return out.toByteArray()
}

private fun write(ctx: Context, uri: Uri?, bytes: ByteArray?) {
    if (uri == null || bytes == null) return
    runCatching { ctx.contentResolver.openOutputStream(uri)?.use { it.write(bytes) } }
}

/**
 * "Export" button that lets the lecturer save the current roster as **PDF or CSV** (their choice),
 * both carrying the same three columns (Reg no · Student name · Status). Uses the system
 * "create document" picker — no storage permission and no FileProvider config needed.
 */
@Composable
fun ExportRosterButton(baseName: String, title: String, rows: () -> List<RosterLine>) {
    val ctx = LocalContext.current
    var choose by remember { mutableStateOf(false) }
    var pending by remember { mutableStateOf<ByteArray?>(null) }
    val saveCsv = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("text/csv")) { uri -> write(ctx, uri, pending) }
    val savePdf = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/pdf")) { uri -> write(ctx, uri, pending) }

    TextButton(onClick = { choose = true }) { Text("⤓ Export") }
    if (choose) AlertDialog(
        onDismissRequest = { choose = false },
        title = { Text("Save roster as") },
        text = { Text("Both formats have the same three columns: Reg no, Student name, Status.") },
        confirmButton = {
            TextButton(onClick = { pending = rosterPdf(title, rows()); choose = false; savePdf.launch("$baseName.pdf") }) { Text("PDF") }
        },
        dismissButton = {
            TextButton(onClick = { pending = rosterCsv(rows()); choose = false; saveCsv.launch("$baseName.csv") }) { Text("CSV (Excel)") }
        },
    )
}
