package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import org.json.JSONObject
import ug.qaat.coordinator.db.AppDao
import ug.qaat.coordinator.db.RosterEntity

/**
 * Pulls GET /api/v1/manifest/daily once while online and stores it locally — the app
 * INHERITS everything the super-admin/tenant-admin set (roster hashes + QR serials, the
 * institution RSA public key, the student-hash key, the policy, the day's units).
 * Parsed with org.json (built into Android — no extra dependency). Shape mirrors
 * services/api-gateway/internal/handlers/manifest.go.
 */
class ManifestClient(private val dao: AppDao) {
    private val http = Net.client()
    private val base = Net.baseUrl

    data class UnitInfo(val unitId: String, val unitName: String)
    data class Parsed(
        val academicYear: String,
        val institutionPublicKeyPem: String,
        val studentHashKey: String,
        val units: List<UnitInfo>,
        val rosterByUnit: Map<String, List<RosterEntity>>,
    )

    suspend fun fetchAndStore(token: String): Parsed {
        val r = http.get("$base/api/v1/manifest/daily") { header("Authorization", "Bearer $token") }
        require(r.status.value in 200..299) { "manifest fetch failed: ${r.status.value}" }
        val j = JSONObject(r.bodyAsText())

        val publicKey = j.optString("institution_public_key", "")
        val hashKey = j.optString("student_hash_key", "")
        val academicYear = j.optString("academic_year", "")

        val units = mutableListOf<UnitInfo>()
        val sessions = j.optJSONArray("sessions")
        if (sessions != null) for (i in 0 until sessions.length()) {
            val s = sessions.getJSONObject(i)
            units.add(UnitInfo(s.getString("unit_id"), s.optString("unit_name", s.getString("unit_id"))))
        }

        val rosterByUnit = LinkedHashMap<String, List<RosterEntity>>()
        val roster = j.optJSONObject("roster")
        if (roster != null) for (unitId in roster.keys()) {
            val arr = roster.getJSONArray(unitId)
            val rows = ArrayList<RosterEntity>(arr.length())
            for (i in 0 until arr.length()) {
                val e = arr.getJSONObject(i)
                rows.add(RosterEntity(unitId, e.getString("student_id_hash"), e.optString("qr_serial_number", "")))
            }
            rosterByUnit[unitId] = rows
            dao.upsertRoster(rows)   // cache for offline validation
        }
        return Parsed(academicYear, publicKey, hashKey, units, rosterByUnit)
    }
}
