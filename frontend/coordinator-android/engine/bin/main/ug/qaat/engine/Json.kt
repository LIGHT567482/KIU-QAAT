package ug.qaat.engine

/**
 * Minimal, dependency-free parser for a FLAT JSON object of string values — exactly
 * the shape of a student QR payload (`{"student_id":"…", … ,"signature":"…"}`).
 * Avoids pulling a JSON library onto the hot check-in path. The Android app may use
 * kotlinx.serialization elsewhere; the QR boundary uses this.
 */
object FlatJson {
    fun parse(raw: String): Map<String, String> {
        val out = LinkedHashMap<String, String>()
        var i = 0
        val n = raw.length
        fun skipWs() { while (i < n && raw[i].isWhitespace()) i++ }
        fun readString(): String {
            // assumes raw[i] == '"'
            i++ // opening quote
            val sb = StringBuilder()
            while (i < n) {
                val c = raw[i]
                if (c == '\\' && i + 1 < n) {
                    val e = raw[i + 1]
                    sb.append(
                        when (e) {
                            '"' -> '"'; '\\' -> '\\'; '/' -> '/'
                            'n' -> '\n'; 'r' -> '\r'; 't' -> '\t'; 'b' -> '\b'
                            'u' -> { val hex = raw.substring(i + 2, i + 6); i += 4; hex.toInt(16).toChar() }
                            else -> e
                        }
                    )
                    i += 2
                } else if (c == '"') { i++; return sb.toString() } else { sb.append(c); i++ }
            }
            throw IllegalArgumentException("unterminated string")
        }
        skipWs()
        require(i < n && raw[i] == '{') { "expected object" }
        i++
        while (true) {
            skipWs()
            if (i < n && raw[i] == '}') { i++; break }
            require(i < n && raw[i] == '"') { "expected key" }
            val key = readString()
            skipWs(); require(i < n && raw[i] == ':') { "expected ':'" }; i++; skipWs()
            require(i < n && raw[i] == '"') { "only string values supported" }
            out[key] = readString()
            skipWs()
            if (i < n && raw[i] == ',') { i++; continue }
        }
        return out
    }

    /** Parse a raw QR string into the signed fields + signature, or null if malformed. */
    fun parseQr(raw: String): SubmittedQr? = try {
        val m = parse(raw)
        SubmittedQr(
            fields = QrFields(
                studentId = m.getValue("student_id"),
                tenantId = m.getValue("tenant_id"),
                courseId = m.getValue("course_id"),
                fullName = m.getValue("full_name"),
                academicYear = m.getValue("academic_year"),
                serialNumber = m.getValue("serial_number"),
                expiryDate = m.getValue("expiry_date"),
                issuedAt = m.getValue("issued_at"),
            ),
            signatureB64 = m.getValue("signature"),
        )
    } catch (_: Exception) { null }
}
