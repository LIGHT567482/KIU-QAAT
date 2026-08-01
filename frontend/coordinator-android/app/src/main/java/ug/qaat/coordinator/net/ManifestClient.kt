package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
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

    data class UnitInfo(
        val unitId: String, val unitName: String,
        val lecturerStaffId: String = "", val lecturerName: String = "", val lecturerPhone: String = "",
        val durationMinutes: Int = 0,   // scheduled length; drives the auto-close deadline
        // Timetable schedule — cached so the coordinator's Timetable grid still renders the units,
        // days and times with NO internet (offline these came back as unscheduled before).
        val dayOfWeek: Int = 0,         // 1=Mon…7=Sun, 0 = unscheduled / any day
        val startTime: String = "",     // "HH:MM" scheduled start
        val venueId: String = "",       // room / venue label
        // ONE code covering both lecturer + session (only set when the lecturer is shared across
        // coordinators today). A coordinator whose lecturer is in another room enters this single
        // code to mark the lecturer present and start attendance here.
        val sessionCode: String = "",
    )
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
        val bodyText = r.bodyAsText()
        // Parse + cache to Room OFF the main thread. This is called from the login coroutine
        // (Compose Main dispatcher) as well as the IO refresh path; Room throws on a
        // main-thread write, so pin the whole parse-and-store step to IO.
        return withContext(Dispatchers.IO) {
        val j = JSONObject(bodyText)

        val publicKey = j.optString("institution_public_key", "")
        val hashKey = j.optString("student_hash_key", "")
        val academicYear = j.optString("academic_year", "")

        val units = mutableListOf<UnitInfo>()
        val sessions = j.optJSONArray("sessions")
        if (sessions != null) for (i in 0 until sessions.length()) {
            val s = sessions.optJSONObject(i) ?: continue
            val unitId = s.optString("unit_id", "")
            if (unitId.isBlank()) continue
            units.add(UnitInfo(
                unitId, s.optString("unit_name", unitId),
                s.optString("lecturer_staff_id", ""), s.optString("lecturer_name", ""), s.optString("lecturer_phone", ""),
                s.optInt("session_duration_minutes", 0),
                s.optInt("day_of_week", 0), s.optString("scheduled_start", ""), s.optString("venue_id", ""),
                s.optString("session_code", ""),
            ))
        }

        val rosterByUnit = LinkedHashMap<String, List<RosterEntity>>()
        val roster = j.optJSONObject("roster")
        if (roster != null) for (unitId in roster.keys()) {
            // A unit with no enrolled students comes back as null (or is simply absent) —
            // optJSONArray returns null instead of throwing, so ONE empty unit no longer
            // aborts the whole manifest parse. Treat it as an empty roster and move on.
            val arr = roster.optJSONArray(unitId) ?: continue
            val rows = ArrayList<RosterEntity>(arr.length())
            for (i in 0 until arr.length()) {
                val e = arr.optJSONObject(i) ?: continue
                val hash = e.optString("student_id_hash", "")
                if (hash.isBlank()) continue
                rows.add(RosterEntity(unitId, hash, e.optString("qr_serial_number", ""),
                    e.optString("student_id", ""), e.optString("full_name", "")))
            }
            rosterByUnit[unitId] = rows
            if (rows.isNotEmpty()) dao.upsertRoster(rows)   // cache for offline validation
        }
        Parsed(academicYear, publicKey, hashKey, units, rosterByUnit)
        }
    }
}
