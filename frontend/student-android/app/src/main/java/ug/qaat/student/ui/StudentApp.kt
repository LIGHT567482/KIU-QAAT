package ug.qaat.student.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import ug.qaat.student.store.StudentStore

/** Root: onboard once, then the one-tap attend screen for every lecture after. */
@Composable
fun StudentApp() = StudentTheme {
    var onboarded by remember { mutableStateOf(StudentStore.onboarded) }
    if (!onboarded) OnboardScreen(onDone = { onboarded = true })
    else AttendScreen(onReonboard = { StudentStore.clear(); onboarded = false })
}
