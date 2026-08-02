package ug.qaat.coordinator.notify

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import ug.qaat.coordinator.MainActivity
import ug.qaat.coordinator.net.NotificationClient

/**
 * Turns in-app alerts into ANDROID notifications — the pop-up on the lock screen / status bar that
 * arrives whether or not the app is on screen.
 *
 * There is no Firebase in this build (the phones are frequently offline and the institution runs its
 * own backend), so there is no server push. Delivery is POLLED — see [AlertPoller] — and this object
 * is the part that decides what the reader actually sees:
 *
 *  • Each alert pops up ONCE, ever. Seen ids are persisted, so a re-poll, an app restart or a
 *    background sync never re-rings something already shown.
 *  • The FIRST poll on a device rings nothing. A reader who installs the app with forty alerts
 *    already in their inbox gets a silent catch-up, not forty pop-ups.
 *  • Dismissing an alert in the app ([suppress]) both cancels its pop-up and blocks it for good —
 *    the ✕ has to mean gone everywhere, not just in the list.
 */
object AlertNotifier {
    const val CHANNEL = "qaat_alerts"

    /** Application context, set once at process start. Application-scoped, so this leaks nothing. */
    @Volatile private var appContext: Context? = null

    private const val PREFS = "qaat_alert_notify"
    private const val KEY_SEEN = "seen_ids"
    private const val KEY_PRIMED = "primed"

    /** Cap on remembered ids. Well past any real inbox, and bounded so prefs cannot grow forever. */
    private const val MAX_SEEN = 400

    fun init(context: Context) {
        appContext = context.applicationContext
        ensureChannel(context.applicationContext)
    }

    private fun prefs(ctx: Context) = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    /**
     * IMPORTANCE_HIGH is what makes this a pop-up ("heads-up") rather than a silent status-bar row.
     * The channel's importance is fixed at creation and cannot be raised later, so a build that
     * shipped with LOW would stay silent forever on every phone that already installed it — hence
     * the distinct channel id rather than reusing the foreground-service channel.
     */
    private fun ensureChannel(ctx: Context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val mgr = ctx.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (mgr.getNotificationChannel(CHANNEL) != null) return
        mgr.createNotificationChannel(
            NotificationChannel(CHANNEL, "Alerts & messages", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "Notices from your lecturer, coordinator or QA office"
                enableVibration(true)
            }
        )
    }

    /**
     * Pop up anything in [inbox] this device has not shown before.
     *
     * Called with the inbox as the server returned it, which already excludes dismissed alerts — so
     * clearing something on the web dashboard also stops it ringing here.
     */
    fun notifyNew(inbox: List<NotificationClient.Notif>) {
        val ctx = appContext ?: return
        if (inbox.isEmpty()) return
        ensureChannel(ctx)

        val p = prefs(ctx)
        val seen = p.getStringSet(KEY_SEEN, emptySet())!!.toMutableSet()

        // First run on this device: adopt the whole inbox silently. Everything already there is
        // history the reader has had a chance to see in the app.
        if (!p.getBoolean(KEY_PRIMED, false)) {
            seen += inbox.map { it.id }
            p.edit().putStringSet(KEY_SEEN, trim(seen, inbox.map { it.id }))
                .putBoolean(KEY_PRIMED, true).apply()
            return
        }

        val fresh = inbox.filter { it.id !in seen && !it.read }
        if (fresh.isEmpty()) {
            // Still record read-elsewhere alerts, so opening one on the web never rings it here.
            val all = seen + inbox.map { it.id }
            p.edit().putStringSet(KEY_SEEN, trim(all.toMutableSet(), inbox.map { it.id })).apply()
            return
        }

        val manager = NotificationManagerCompat.from(ctx)
        for (n in fresh) {
            val tap = PendingIntent.getActivity(
                ctx, n.id.hashCode(),
                Intent(ctx, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP),
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
            )
            val who = n.senderName.ifBlank { n.senderRole.replace('_', ' ') }
            val note = NotificationCompat.Builder(ctx, CHANNEL)
                .setSmallIcon(android.R.drawable.ic_dialog_email)
                .setContentTitle(n.subject.ifBlank { "New notice" })
                .setContentText(if (n.body.isBlank()) "from $who" else n.body)
                .setStyle(NotificationCompat.BigTextStyle().bigText(n.body).setSummaryText("from $who"))
                .setPriority(NotificationCompat.PRIORITY_HIGH)     // pre-O equivalent of the channel
                .setCategory(NotificationCompat.CATEGORY_MESSAGE)
                .setAutoCancel(true)
                .setContentIntent(tap)
                .build()
            // POST_NOTIFICATIONS may be denied (Android 13+). notify() then throws; the alert is
            // still in the app's inbox, so swallow it rather than crash a background poll.
            runCatching { manager.notify(idOf(n.id), note) }
        }

        val all = (seen + inbox.map { it.id }).toMutableSet()
        p.edit().putStringSet(KEY_SEEN, trim(all, inbox.map { it.id })).apply()
    }

    /**
     * The reader cleared this alert in the app: cancel its pop-up and never raise it again.
     * Called BEFORE the server round-trip, because the intent to be rid of it is the reader's,
     * not the network's.
     */
    fun suppress(id: String) {
        val ctx = appContext ?: return
        runCatching { NotificationManagerCompat.from(ctx).cancel(idOf(id)) }
        val p = prefs(ctx)
        val seen = p.getStringSet(KEY_SEEN, emptySet())!!.toMutableSet()
        seen += id
        p.edit().putStringSet(KEY_SEEN, seen).apply()
    }

    /** Signing out clears the pop-ups and the memory of them — the next account starts clean. */
    fun reset() {
        val ctx = appContext ?: return
        runCatching { NotificationManagerCompat.from(ctx).cancelAll() }
        prefs(ctx).edit().clear().apply()
    }

    /** Stable positive notification id from an alert's uuid. */
    private fun idOf(alertId: String): Int = (alertId.hashCode() and 0x7fffffff)

    /** Keep the set bounded, preferring ids still present in the inbox over old ones. */
    private fun trim(seen: MutableSet<String>, current: List<String>): Set<String> {
        if (seen.size <= MAX_SEEN) return seen
        val keep = LinkedHashSet<String>(current)
        for (id in seen) {
            if (keep.size >= MAX_SEEN) break
            keep += id
        }
        return keep
    }
}
