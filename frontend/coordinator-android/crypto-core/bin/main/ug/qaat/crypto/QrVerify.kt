package ug.qaat.crypto

import java.security.KeyFactory
import java.security.Signature
import java.security.spec.X509EncodedKeySpec
import java.util.Base64

/**
 * QrVerify — the on-device port of the student-QR signature check
 * (apps/coordinator-pwa/src/qr/validator.ts + services/qr-generator/src/crypto/rsa-keys.ts).
 *
 * The QR is signed `RSA-SHA256` (RSASSA-PKCS1-v1_5 + SHA-256, base64) over
 * `body = JSON.stringify(payload)` where payload is the 8 fields below IN THIS
 * ORDER (Node's JSON.stringify preserves insertion order; we must match it exactly).
 */
object QrVerify {
    /** The signed QR payload fields, in the canonical order Node serialises them. */
    data class QrPayload(
        val student_id: String,
        val tenant_id: String,
        val course_id: String,
        val full_name: String,
        val academic_year: String,
        val serial_number: String,
        val expiry_date: String,
        val issued_at: String,
    )

    /** Rebuild the exact `JSON.stringify(payload)` string Node signed. */
    fun canonicalBody(p: QrPayload): String {
        fun esc(s: String) = s
            .replace("\\", "\\\\").replace("\"", "\\\"")
            .replace("\n", "\\n").replace("\r", "\\r").replace("\t", "\\t")
        return "{" +
            "\"student_id\":\"${esc(p.student_id)}\"," +
            "\"tenant_id\":\"${esc(p.tenant_id)}\"," +
            "\"course_id\":\"${esc(p.course_id)}\"," +
            "\"full_name\":\"${esc(p.full_name)}\"," +
            "\"academic_year\":\"${esc(p.academic_year)}\"," +
            "\"serial_number\":\"${esc(p.serial_number)}\"," +
            "\"expiry_date\":\"${esc(p.expiry_date)}\"," +
            "\"issued_at\":\"${esc(p.issued_at)}\"" +
            "}"
    }

    /** Verify the RSA-SHA256 signature (base64) over `body` against a PEM public key. */
    fun verify(publicKeyPem: String, body: String, signatureB64: String): Boolean {
        val pub = parsePublicKey(publicKeyPem)
        val sig = Signature.getInstance("SHA256withRSA")
        sig.initVerify(pub)
        sig.update(body.toByteArray(Charsets.UTF_8))
        return sig.verify(Base64.getDecoder().decode(signatureB64))
    }

    fun verifyPayload(publicKeyPem: String, p: QrPayload, signatureB64: String): Boolean =
        verify(publicKeyPem, canonicalBody(p), signatureB64)

    private fun parsePublicKey(pem: String): java.security.PublicKey {
        val b64 = pem.replace("-----BEGIN PUBLIC KEY-----", "")
            .replace("-----END PUBLIC KEY-----", "")
            .replace("\\s".toRegex(), "")
        val der = Base64.getDecoder().decode(b64)
        return KeyFactory.getInstance("RSA").generatePublic(X509EncodedKeySpec(der))
    }
}
