package ug.qaat.coordinator.di

import android.content.Context
import androidx.room.Room
import ug.qaat.coordinator.data.Repository
import ug.qaat.coordinator.db.AppDatabase
import ug.qaat.coordinator.db.MIGRATION_1_2
import ug.qaat.coordinator.db.MIGRATION_2_3

/** Tiny manual service-locator (no Hilt) — one DB instance shared by the UI + the service. */
object Graph {
    lateinit var db: AppDatabase
        private set
    lateinit var repo: Repository
        private set
    lateinit var appContext: Context
        private set

    fun init(context: Context) {
        appContext = context.applicationContext
        if (::db.isInitialized) return
        // Production: wrap with SQLCipher SupportFactory keyed by an Android-Keystore secret.
        db = Room.databaseBuilder(context.applicationContext, AppDatabase::class.java, "qaat.db")
            .addMigrations(MIGRATION_1_2, MIGRATION_2_3)
            .fallbackToDestructiveMigration()   // safety net for any unforeseen schema drift
            .build()
        repo = Repository(db.dao())
    }
}
