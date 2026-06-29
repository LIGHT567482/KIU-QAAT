package ug.qaat.coordinator.di

import android.content.Context
import androidx.room.Room
import ug.qaat.coordinator.data.Repository
import ug.qaat.coordinator.db.AppDatabase

/** Tiny manual service-locator (no Hilt) — one DB instance shared by the UI + the service. */
object Graph {
    lateinit var db: AppDatabase
        private set
    lateinit var repo: Repository
        private set

    fun init(context: Context) {
        if (::db.isInitialized) return
        // Production: wrap with SQLCipher SupportFactory keyed by an Android-Keystore secret.
        db = Room.databaseBuilder(context.applicationContext, AppDatabase::class.java, "qaat.db")
            .fallbackToDestructiveMigration()
            .build()
        repo = Repository(db.dao())
    }
}
