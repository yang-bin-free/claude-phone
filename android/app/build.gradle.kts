plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val syncWebAssets by tasks.registering(Sync::class) {
    from("../../web/chat/index.html") { into("chat") }
    from(listOf("../../web/chat/chat.js", "../../web/chat/core.css", "../../web/chat/desktop.css", "../../web/chat/mobile.css")) {
        into("assets")
    }
    from(listOf("../../web/admin/admin.js", "../../web/admin/admin.css")) { into("assets") }
    into(layout.buildDirectory.dir("generated/sharedWebAssets"))
}

android {
    namespace = "com.claudephone"
    compileSdk = 35
    buildToolsVersion = "35.0.0"

    defaultConfig {
        applicationId = "com.claudephone"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    sourceSets["main"].assets.srcDir(layout.buildDirectory.dir("generated/sharedWebAssets"))
}

tasks.named("preBuild") { dependsOn(syncWebAssets) }

dependencies {
    // gomobile 生成的 .aar
    implementation(files("../../build/claudelib.aar"))
}
