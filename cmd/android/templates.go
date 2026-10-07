package main

// Ebitengine's Android bridge requires an Activity written against Android APIs.
// These host files are generated locally; rendering, input and audio stay in Go.
const settingsGradle = `pluginManagement {
    repositories { google(); mavenCentral(); gradlePluginPortal() }
}
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories { google(); mavenCentral() }
}
rootProject.name = 'BattleSquadron'
include(':app')
`

const appGradle = `plugins { id 'com.android.application' }
android {
    namespace 'com.olivierh.battlesquadron'
    compileSdk 36
    buildToolsVersion '36.0.0'
    defaultConfig {
        applicationId 'com.olivierh.battlesquadron'
        minSdk 23
        targetSdk 36
        versionCode 1
        versionName '0.1.0'
        testInstrumentationRunner 'com.olivierh.battlesquadron.TouchRunner'
    }
    signingConfigs {
        debug {
            storeFile file(DEBUG_KEY)
            storeType 'PKCS12'
            storePassword 'android'
            keyAlias 'androiddebugkey'
            keyPassword 'android'
        }
    }
    buildTypes {
        debug { signingConfig signingConfigs.debug }
        release { minifyEnabled false }
    }
    compileOptions {
        sourceCompatibility JavaVersion.VERSION_17
        targetCompatibility JavaVersion.VERSION_17
    }
    packaging {
        jniLibs { useLegacyPackaging false }
    }
}
dependencies { implementation files('libs/battlesquadron.aar') }
`

const androidManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-feature android:glEsVersion="0x00030000" android:required="true" />
    <application android:label="Battle Squadron" android:allowBackup="false"
        android:theme="@android:style/Theme.Material.NoActionBar.Fullscreen"
        android:enableOnBackInvokedCallback="true">
        <activity android:name=".MainActivity" android:exported="true"
            android:screenOrientation="landscape" android:launchMode="singleTop"
            android:configChanges="orientation|screenSize|keyboardHidden|uiMode">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`

const nativeActivity = `package com.olivierh.battlesquadron;

import android.app.Activity;
import android.os.Build;
import android.os.Bundle;
import android.view.InputDevice;
import android.view.KeyEvent;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.window.OnBackInvokedDispatcher;
import com.olivierh.battlesquadron.mobile.EbitenView;
import com.olivierh.battlesquadron.mobile.Mobile;

/** Generated Android lifecycle bridge; all game behavior is implemented in Go. */
public final class MainActivity extends Activity {
    private EbitenView view;

    @Override protected void onCreate(Bundle state) {
        super.onCreate(state);
        Mobile.configure(getFilesDir().getAbsolutePath());
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        if (Build.VERSION.SDK_INT >= 28) {
            WindowManager.LayoutParams attributes = getWindow().getAttributes();
            attributes.layoutInDisplayCutoutMode =
                WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            getWindow().setAttributes(attributes);
        }
        if (Build.VERSION.SDK_INT >= 33) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::handleBack);
        }
        view = new EbitenView(this);
        view.setFocusableInTouchMode(true);
        view.requestFocus();
        setContentView(view);
        hideSystemUi();
    }

    @Override protected void onPause() {
        if (view != null) {
            Mobile.requestPause();
            view.suspendGame();
        }
        super.onPause();
    }

    @Override protected void onResume() {
        super.onResume();
        hideSystemUi();
        if (view != null) view.resumeGame();
    }

    @Override public void onBackPressed() { handleBack(); }

    @Override public boolean dispatchKeyEvent(KeyEvent event) {
        // Route legacy system Back before EbitenView consumes it.
        if (Build.VERSION.SDK_INT < 33 && event.getKeyCode() == KeyEvent.KEYCODE_BACK
            && !event.isFromSource(InputDevice.SOURCE_GAMEPAD)
            && !event.isFromSource(InputDevice.SOURCE_JOYSTICK)) {
            if (event.getAction() == KeyEvent.ACTION_UP && !event.isCanceled()) handleBack();
            return true;
        }
        return super.dispatchKeyEvent(event);
    }

    private void handleBack() {
        if (Mobile.isAtTitle()) finish();
        else Mobile.requestBack();
    }

    @Override public void onWindowFocusChanged(boolean focused) {
        super.onWindowFocusChanged(focused);
        if (focused) hideSystemUi();
        else Mobile.cancelInput();
    }

    private void hideSystemUi() {
        if (Build.VERSION.SDK_INT >= 30) {
            getWindow().setDecorFitsSystemWindows(false);
            WindowInsetsController controller = getWindow().getDecorView().getWindowInsetsController();
            if (controller != null) {
                controller.hide(WindowInsets.Type.statusBars() | WindowInsets.Type.navigationBars());
                controller.setSystemBarsBehavior(WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            }
        } else {
            getWindow().getDecorView().setSystemUiVisibility(
                View.SYSTEM_UI_FLAG_FULLSCREEN | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                | View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
        }
    }
}
`
