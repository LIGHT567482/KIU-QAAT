package ug.qaat.coordinator.net

import io.ktor.client.request.*
import io.ktor.client.statement.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * Which rooms are free, right now — for the coordinator standing outside a room they cannot use.
 *
 * The timetabled room is locked, double-booked, being repainted, or an exam has it, and a class is
 * waiting. Before this the coordinator walked the corridor looking through doors, and the empty
 * room they found was often one another college was about to fill.
 *
 * ONLINE ONLY, and that is honest rather than a limitation to hide. "Is this room free?" is a
 * question about what every OTHER coordinator in the institution is doing at this second; a cached
 * answer would be confidently wrong in exactly the situation it is consulted, and sending two
 * classes to one room is worse than telling someone to look for themselves.
 */
data class FreeRoom(
    val venueId: String,
    val name: String,
    val building: String,
    val capacity: Int,
    val school: String,
    val free: Boolean,
    val occupiedBy: String,
    val occupiedUntil: String,
    val occupiedNote: String,
)

class FreeRoomsClient {
    private val http = Net.client()
    private val base = Net.baseUrl

    /** GET /api/v1/rooms/free — every room in the institution, free or busy, for the next window. */
    suspend fun list(token: String, minutes: Int = 60): List<FreeRoom> = withContext(Dispatchers.IO) {
        val r = http.get("$base/api/v1/rooms/free?minutes=$minutes") {
            header("Authorization", "Bearer $token")
        }
        if (!r.status.value.let { it in 200..299 }) return@withContext emptyList()
        val o = JSONObject(r.bodyAsText())
        val arr = o.optJSONArray("rooms") ?: return@withContext emptyList()
        (0 until arr.length()).map {
            val x = arr.getJSONObject(it)
            FreeRoom(
                venueId = x.optString("venue_id"),
                name = x.optString("name").ifBlank { x.optString("venue_id") },
                building = x.optString("building"),
                capacity = x.optInt("capacity"),
                school = x.optString("school"),
                free = x.optBoolean("free"),
                occupiedBy = x.optString("occupied_by"),
                occupiedUntil = x.optString("occupied_until"),
                occupiedNote = x.optString("occupied_note"),
            )
        }
    }
}
