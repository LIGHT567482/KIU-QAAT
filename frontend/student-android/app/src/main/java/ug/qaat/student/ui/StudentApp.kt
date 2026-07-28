package ug.qaat.student.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import ug.qaat.student.store.StudentStore

/** Root: onboard once, then a two-tab app — one-tap Attend + online My-attendance. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun StudentApp() = StudentTheme {
    var onboarded by remember { mutableStateOf(StudentStore.onboarded) }
    if (!onboarded) {
        OnboardScreen(onDone = { onboarded = true })
        return@StudentTheme
    }
    var tab by remember { mutableStateOf(0) }
    Scaffold(
        bottomBar = {
            NavigationBar {
                NavigationBarItem(selected = tab == 0, onClick = { tab = 0 },
                    icon = { Text("✓") }, label = { Text("Attend") })
                NavigationBarItem(selected = tab == 1, onClick = { tab = 1 },
                    icon = { Text("📊") }, label = { Text("My attendance") })
            }
        },
    ) { pad ->
        Box(Modifier.padding(pad)) {
            if (tab == 0) AttendScreen(onReonboard = { StudentStore.clear(); onboarded = false })
            else ProgressScreen()
        }
    }
}
