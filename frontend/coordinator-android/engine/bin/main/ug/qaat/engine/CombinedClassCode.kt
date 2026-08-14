package ug.qaat.engine

import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

/**
 * THE COMBINED-CLASS CODE — how one lecturer starts a lecture in several registers at once.
 *
 * A lecture is often taught to several cohorts together: one room, one hour, one lecturer, and two
 * or three coordinators each running their own hotspot for their own cohort. The lecturer's phone
 * can only join ONE of those hotspots — a phone associates with one access point at a time — so
 * they can only gate in through one coordinator. The other coordinators are left holding sessions
 * that no lecturer has started, and their students cannot check in.
 *
 * So the lecturer, having gated in on whichever hotspot they joined, reads out a THREE-DIGIT CODE.
 * The other coordinators type it into their own dashboards; their phone checks it and opens their
 * register. That is the whole handshake, and it happens across the room by voice.
 *
 * IT MUST WORK WITH NO NETWORK AT ALL. Not "degrades gracefully offline" — there is no uplink in a
 * lecture hall, and the coordinator phones are not even on the same Wi-Fi as each other (each is
 * running its own hotspot). A code the phones had to fetch, register, or compare with anyone would
 * simply not work. So the code is DERIVED, not issued: every coordinator's phone independently
 * computes what today's code for this lecturer and this class must be, from the session package it
 * was already given, and compares. Nothing is transmitted between phones except the lecturer's
 * voice saying three digits.
 *
 * A NEW SET OF CODES EVERY DAY. The local date is part of what is hashed, so the code for a class
 * changes at midnight and every day's codes are independent of every other day's. A code overheard,
 * written down or screenshotted today is refused tomorrow, because tomorrow derives a different
 * number and `validate` only ever compares against the day it is called on.
 *
 * Stated exactly, because the tests measure it: with a thousand possible codes, about one day in a
 * thousand happens to derive the SAME digits as any given other day, and on such a day the old
 * number genuinely is that day's code and would be accepted. Three digits cannot avoid that, and
 * this is not a reused session — it is two independent draws landing on the same number. What the
 * day binding guarantees is independence, not distinctness: 999 times in 1000 yesterday's code is
 * dead, and a repeat carries no meaning from the day before it.
 *
 * WHAT "UNIQUE" HONESTLY MEANS HERE. Three digits is a thousand values, so across a campus running
 * dozens of simultaneous lectures two classes will sometimes show the same digits — that is
 * arithmetic, not a defect, and no offline scheme can avoid it without more digits. It does not
 * matter, because a code is never checked in the abstract: each coordinator validates against the
 * code THEIR class must have today. A code belonging to another lecture, or to the right lecture on
 * the wrong day, derives differently and is refused. Uniqueness is scoped to where it is used, and
 * that is the only place it has to hold.
 *
 * The code is bound to the LECTURER and the CLASS, not to the coordinator: every cohort of the same
 * combined lecture derives the same digits, which is exactly why one spoken number opens all of
 * them, while a different lecturer in the next room derives different digits from the same key.
 */
object CombinedClassCode {
    const val DIGITS = 3
    private const val MOD = 1_000L // 10^DIGITS

    /**
     * The code for this lecturer + class on this local date.
     *
     * @param secret     the shared session-package key both phones already hold
     * @param lecturerId who is teaching — binds the code to the person who started the lecture
     * @param classKey   what is being taught. For a combined class this must be the SAME string on
     *                   every cohort's phone, so pass the shared identity (the combined-class id, or
     *                   the timetabled slot group) rather than a per-cohort unit code — the cohorts
     *                   deliberately carry different unit codes and hashing those would give each
     *                   coordinator a different number.
     * @param localDate  ISO yyyy-MM-dd in the institution's own timezone, NOT UTC. A lecture at
     *                   22:00 Kampala is still 01:00 UTC the next day, and deriving from UTC would
     *                   roll the code over in the middle of an evening class.
     */
    fun derive(secret: ByteArray, lecturerId: String, classKey: String, localDate: String): String {
        // The unit separator between fields is load-bearing, not decoration: plain concatenation
        // hashes ("ab","c") and ("a","bc") to the same message, so two different classes could derive
        // one code by accident of where their ids happen to split. A control character is used
        // because no id can contain one and forge a different split.
        val msg = listOf("combined-class-v1", lecturerId, classKey, localDate)
            .joinToString("\u001F")
        val mac = Mac.getInstance("HmacSHA256")
        mac.init(SecretKeySpec(secret, "HmacSHA256"))
        val h = mac.doFinal(msg.toByteArray(Charsets.UTF_8))
        // Dynamic truncation, as RFC 4226 — take an offset from the low nibble so the digits are
        // drawn from a varying window of the hash rather than always its first bytes.
        val off = (h[h.size - 1].toInt() and 0x0F)
        val bin = ((h[off].toInt() and 0x7F) shl 24) or
            ((h[off + 1].toInt() and 0xFF) shl 16) or
            ((h[off + 2].toInt() and 0xFF) shl 8) or
            (h[off + 3].toInt() and 0xFF)
        return (bin % MOD).toString().padStart(DIGITS, '0')
    }

    /**
     * Whether a typed code is the one for this lecturer, class and day.
     *
     * There is deliberately NO grace for yesterday's code. The whole reason the date is in the
     * derivation is that a day's codes must not carry into the next day's registers, and accepting
     * yesterday's for a few hours after midnight would reintroduce exactly what it prevents.
     *
     * Constant-time comparison: a coordinator typing digits is not a plausible timing attacker, but
     * this is a credential check and the cost of doing it properly is nil.
     */
    fun validate(
        secret: ByteArray,
        typed: String,
        lecturerId: String,
        classKey: String,
        localDate: String,
    ): Boolean {
        val clean = typed.trim()
        if (clean.length != DIGITS) return false
        return constantTimeEquals(derive(secret, lecturerId, classKey, localDate), clean)
    }

    private fun constantTimeEquals(a: String, b: String): Boolean {
        if (a.length != b.length) return false
        var diff = 0
        for (i in a.indices) diff = diff or (a[i].code xor b[i].code)
        return diff == 0
    }
}
