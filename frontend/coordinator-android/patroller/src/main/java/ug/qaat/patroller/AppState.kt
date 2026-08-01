package ug.qaat.patroller

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue

/** Tiny observable app state for the patroller app. */
object AppState {
    var lastCrash by mutableStateOf<String?>(null)
    var token by mutableStateOf<String?>(null)
    var name by mutableStateOf("")
    var staffId by mutableStateOf("")
    val loggedIn: Boolean get() = token != null
}
