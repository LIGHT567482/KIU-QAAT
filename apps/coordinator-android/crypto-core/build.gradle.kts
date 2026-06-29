// Pure Kotlin/JVM — no Android deps. Verified off-device (see verify.sh).
plugins { kotlin("jvm") }
kotlin { jvmToolchain(17) }
dependencies { testImplementation(kotlin("test")) }
