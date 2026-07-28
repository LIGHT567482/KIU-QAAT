package ug.qaat.student

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import ug.qaat.student.store.StudentStore
import ug.qaat.student.ui.StudentApp

/** Single-activity Compose host. Initializes encrypted storage, then shows the app. */
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        StudentStore.init(applicationContext)
        setContent { StudentApp() }
    }
}
