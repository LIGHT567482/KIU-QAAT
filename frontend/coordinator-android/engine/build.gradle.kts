// Pure Kotlin/JVM in-room engine (validation chain, session manager, room code).
// Verified off-device against real vectors + the live backend.
plugins { kotlin("jvm") }
kotlin { jvmToolchain(17) }
dependencies {
    implementation(project(":crypto-core"))
    testImplementation(kotlin("test"))
}

// The files under src/test here are NOT JUnit tests — they are `main()` programs run by hand
// against real vectors (a node-signed QR, a live backend), each taking arguments. Gradle 9 fails
// the build when a test source set discovers nothing, which made `./gradlew test` unusable for the
// whole project and hid the app module's real JUnit E2E test behind an unrelated failure.
//
// Kept as-is rather than converted: they need external inputs a unit test cannot supply. This just
// stops their presence failing the run. `:app:test` is where the automated coverage lives.
tasks.withType<Test>().configureEach {
    failOnNoDiscoveredTests = false
}
