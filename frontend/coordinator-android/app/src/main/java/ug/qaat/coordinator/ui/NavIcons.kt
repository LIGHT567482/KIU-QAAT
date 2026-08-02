package ug.qaat.coordinator.ui

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AccountCircle
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.DateRange
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Menu
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Warning
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.path
import androidx.compose.ui.unit.dp

/**
 * The single icon set for every nav bar in the app — top bars and bottom bars alike.
 *
 * The bars used to draw emoji (🏠 📈 🔔 📊 👥). Emoji render in the font's own colours, at the
 * font's own weight, and each OEM ships a different picture for the same code point — so the bar
 * looked like a row of small colourful stickers that varied per phone. These are plain vector
 * shapes instead: one flat silhouette each, drawn in whatever colour the bar asks for (solid
 * white on the branded bars) and crisp at any density.
 *
 * Most come straight from `material-icons-core`, which the app already depends on. The four the
 * core set doesn't carry are declared below as vector paths rather than by pulling in the whole
 * `material-icons-extended` artifact, which would add megabytes to an APK that ships unminified.
 */
object NavIcons {
    val Home get() = Icons.Filled.Home
    val Session get() = Icons.Filled.PlayArrow
    val Absentees get() = Icons.Filled.Warning
    val Alerts get() = Icons.Filled.Notifications
    val Sync get() = Icons.Filled.Refresh
    val Attend get() = Icons.Filled.Check
    val Profile get() = Icons.Filled.Menu
    val Account get() = Icons.Filled.AccountCircle
    val Person get() = Icons.Filled.Person

    /** The ✕ that dismisses an alert. */
    val Close get() = Icons.Filled.Close

    /** The lecturer's teaching calendar. */
    val Calendar get() = Icons.Filled.DateRange

    /** Shield with a tick — the QA patroller's round. */
    val Patrol: ImageVector by lazy {
        materialPath("qaat_patrol") {
            moveTo(12f, 1f); lineTo(3f, 5f); verticalLineToRelative(6f)
            curveToRelative(0f, 5.55f, 3.84f, 10.74f, 9f, 12f)
            curveToRelative(5.16f, -1.26f, 9f, -6.45f, 9f, -12f)
            verticalLineTo(5f); close()
            moveTo(10.6f, 16.2f); lineTo(6.4f, 12f); lineToRelative(1.4f, -1.4f)
            lineToRelative(2.8f, 2.8f); lineToRelative(5.6f, -5.6f); lineTo(17.6f, 9.2f); close()
        }
    }

    /** Rising line chart — "Trends". */
    val Trends: ImageVector by lazy {
        materialPath("qaat_trends") {
            moveTo(16f, 6f); lineToRelative(2.29f, 2.29f); lineToRelative(-4.88f, 4.88f)
            lineToRelative(-4f, -4f); lineTo(2f, 16.59f); lineTo(3.41f, 18f); lineToRelative(6f, -6f)
            lineToRelative(4f, 4f); lineToRelative(6.3f, -6.29f); lineTo(22f, 12f); lineTo(22f, 6f)
            close()
        }
    }

    /** Column chart — "Attendance". */
    val Attendance: ImageVector by lazy {
        materialPath("qaat_attendance") {
            moveTo(5f, 9.2f); horizontalLineToRelative(3f); verticalLineTo(19f); horizontalLineTo(5f); close()
            moveTo(10.6f, 5f); horizontalLineToRelative(2.8f); verticalLineTo(19f); horizontalLineTo(10.6f); close()
            moveTo(16.2f, 13f); horizontalLineTo(19f); verticalLineTo(19f); horizontalLineTo(16.2f); close()
        }
    }

    /** Two figures — "Roster". */
    val Roster: ImageVector by lazy {
        materialPath("qaat_roster") {
            moveTo(16f, 11f)
            curveToRelative(1.66f, 0f, 2.99f, -1.34f, 2.99f, -3f)
            reflectiveCurveTo(17.66f, 5f, 16f, 5f)
            curveToRelative(-1.66f, 0f, -3f, 1.34f, -3f, 3f)
            reflectiveCurveToRelative(1.34f, 3f, 3f, 3f)
            close()
            moveTo(8f, 11f)
            curveToRelative(1.66f, 0f, 2.99f, -1.34f, 2.99f, -3f)
            reflectiveCurveTo(9.66f, 5f, 8f, 5f)
            curveTo(6.34f, 5f, 5f, 6.34f, 5f, 8f)
            reflectiveCurveToRelative(1.34f, 3f, 3f, 3f)
            close()
            moveTo(8f, 13f)
            curveToRelative(-2.33f, 0f, -7f, 1.17f, -7f, 3.5f)
            verticalLineTo(19f)
            horizontalLineToRelative(14f)
            verticalLineToRelative(-2.5f)
            curveToRelative(0f, -2.33f, -4.67f, -3.5f, -7f, -3.5f)
            close()
            moveTo(16f, 13f)
            curveToRelative(-0.29f, 0f, -0.62f, 0.02f, -0.97f, 0.05f)
            curveToRelative(1.16f, 0.84f, 1.97f, 1.97f, 1.97f, 3.45f)
            verticalLineTo(19f)
            horizontalLineToRelative(6f)
            verticalLineToRelative(-2.5f)
            curveToRelative(0f, -2.33f, -4.67f, -3.5f, -7f, -3.5f)
            close()
        }
    }

    /** Filled sun — light-theme half of the theme toggle. */
    val LightMode: ImageVector by lazy {
        materialPath("qaat_light_mode") {
            moveTo(12f, 7f)
            curveToRelative(-2.76f, 0f, -5f, 2.24f, -5f, 5f)
            reflectiveCurveToRelative(2.24f, 5f, 5f, 5f)
            reflectiveCurveToRelative(5f, -2.24f, 5f, -5f)
            reflectiveCurveToRelative(-2.24f, -5f, -5f, -5f)
            close()
            moveTo(11f, 1f); horizontalLineToRelative(2f); verticalLineToRelative(3f); horizontalLineToRelative(-2f); close()
            moveTo(11f, 20f); horizontalLineToRelative(2f); verticalLineToRelative(3f); horizontalLineToRelative(-2f); close()
            moveTo(1f, 11f); horizontalLineToRelative(3f); verticalLineToRelative(2f); horizontalLineToRelative(-3f); close()
            moveTo(20f, 11f); horizontalLineToRelative(3f); verticalLineToRelative(2f); horizontalLineToRelative(-3f); close()
            moveTo(4.22f, 5.64f); lineToRelative(1.42f, -1.42f); lineToRelative(2.12f, 2.12f)
            lineToRelative(-1.42f, 1.42f); close()
            moveTo(16.24f, 17.66f); lineToRelative(1.42f, -1.42f); lineToRelative(2.12f, 2.12f)
            lineToRelative(-1.42f, 1.42f); close()
            moveTo(16.24f, 6.34f); lineToRelative(2.12f, -2.12f); lineToRelative(1.42f, 1.42f)
            lineToRelative(-2.12f, 2.12f); close()
            moveTo(4.22f, 18.36f); lineToRelative(2.12f, -2.12f); lineToRelative(1.42f, 1.42f)
            lineToRelative(-2.12f, 2.12f); close()
        }
    }

    /** Filled crescent — dark-theme half of the theme toggle. */
    val DarkMode: ImageVector by lazy {
        materialPath("qaat_dark_mode") {
            moveTo(12f, 3f)
            curveToRelative(-4.97f, 0f, -9f, 4.03f, -9f, 9f)
            reflectiveCurveToRelative(4.03f, 9f, 9f, 9f)
            curveToRelative(3.72f, 0f, 6.91f, -2.25f, 8.28f, -5.47f)
            curveToRelative(-0.9f, 0.31f, -1.86f, 0.47f, -2.86f, 0.47f)
            curveToRelative(-4.83f, 0f, -8.75f, -3.92f, -8.75f, -8.75f)
            curveToRelative(0f, -1.45f, 0.35f, -2.81f, 0.98f, -4.01f)
            curveTo(11.42f, 3.08f, 11.71f, 3f, 12f, 3f)
            close()
        }
    }
}

/**
 * Builds a 24dp Material-geometry vector filled with the *current* content colour, so a single
 * declaration renders solid white on a branded bar and takes the theme colour anywhere else.
 */
private fun materialPath(
    name: String,
    pathBuilder: androidx.compose.ui.graphics.vector.PathBuilder.() -> Unit,
): ImageVector = ImageVector.Builder(
    name = name,
    defaultWidth = 24.dp, defaultHeight = 24.dp,
    viewportWidth = 24f, viewportHeight = 24f,
).apply {
    path(fill = SolidColor(Color.White), pathBuilder = pathBuilder)
}.build()
