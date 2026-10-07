package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// checkEmulator installs only on explicitly selected local emulator transports.
func (c *configuration) checkEmulator(apk string) error {
	args := []string{"--no-daemon", "--console=plain", "-p", c.project, ":app:assembleDebugAndroidTest"}
	if c.offline {
		args = append(args, "--offline")
	}
	if err := c.command(c.root, c.gradle, args...); err != nil {
		return err
	}
	adb := filepath.Join(c.sdk, "platform-tools", "adb")
	// A fresh emulator's one-time fullscreen hint intercepts the first gesture.
	if err := c.command(c.root, adb, "-s", c.serial, "shell", "settings", "put", "secure", "immersive_mode_confirmations", "confirmed"); err != nil {
		return err
	}
	testAPK := filepath.Join(c.project, "app", "build", "outputs", "apk", "androidTest", "debug", "app-debug-androidTest.apk")
	for _, artifact := range []string{apk, testAPK} {
		if err := c.command(c.root, adb, "-s", c.serial, "install", "-r", artifact); err != nil {
			return err
		}
	}
	command := exec.Command(adb, "-s", c.serial, "shell", "am", "instrument", "-w", applicationID+".test/"+applicationID+".TouchRunner")
	command.Env, command.Dir = c.environment, c.root
	result, err := command.CombinedOutput()
	fmt.Print(string(result))
	if err != nil {
		return fmt.Errorf("emulator instrumentation: %w", err)
	}
	if !strings.Contains(string(result), "INSTRUMENTATION_RESULT: success=true") {
		return fmt.Errorf("emulator instrumentation did not report success")
	}
	captures := filepath.Join(c.root, "captures")
	if err := os.MkdirAll(captures, 0755); err != nil {
		return err
	}
	for _, name := range []string{"android-title.png", "android-touch.png", "android-paused.png", "android-check.json"} {
		path := filepath.Join(captures, name)
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		pull := exec.Command(adb, "-s", c.serial, "exec-out", "run-as", applicationID, "cat", "files/"+name)
		pull.Env, pull.Stdout, pull.Stderr = c.environment, file, os.Stderr
		pullErr, closeErr := pull.Run(), file.Close()
		if pullErr != nil {
			return fmt.Errorf("retrieve %s: %w", name, pullErr)
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Println("Emulator evidence:", path)
	}
	return nil
}

// Android's input-injection and lifecycle APIs require this generated native
// instrumentation bridge. Assertions concern the actual shared Go game state.
const instrumentationRunner = `package com.olivierh.battlesquadron;

import android.app.Activity;
import android.app.Instrumentation;
import android.content.Intent;
import android.graphics.Bitmap;
import android.os.Bundle;
import android.os.SystemClock;
import android.view.InputDevice;
import android.view.KeyEvent;
import android.view.MotionEvent;
import java.io.File;
import java.io.FileOutputStream;
import java.nio.charset.StandardCharsets;
import org.json.JSONObject;
import com.olivierh.battlesquadron.mobile.Mobile;

/** Generated emulator-only checks for simultaneous touch and activity suspension. */
public final class TouchRunner extends Instrumentation {
    private long downTime;
    private float scale, offsetX, offsetY;

    @Override public void onCreate(Bundle arguments) {
        super.onCreate(arguments);
        start();
    }

    @Override public void onStart() {
        Bundle result = new Bundle();
        try {
            Intent launch = new Intent(Intent.ACTION_MAIN);
            launch.setClassName("com.olivierh.battlesquadron", "com.olivierh.battlesquadron.MainActivity");
            launch.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_SINGLE_TOP);
            startActivitySync(launch);
            Mobile.setVerificationEnabled(true);
            waitForState(false);
            // The first Go update precedes its first presented GPU frame.
            SystemClock.sleep(1500);
            Bitmap image = getUiAutomation().takeScreenshot();
            require(image != null, "Android screenshot is unavailable");
            scale = Math.min(image.getWidth() / 480f, image.getHeight() / 256f);
            offsetX = (image.getWidth() - 480f * scale) / 2f;
            offsetY = (image.getHeight() - 256f * scale) / 2f;
            image.recycle();
            screenshot("android-title.png");
            tap(240, 210);
            JSONObject before = waitForState(true);

            downTime = SystemClock.uptimeMillis();
            touch(MotionEvent.ACTION_DOWN, new float[][]{{30, 180}});
            SystemClock.sleep(100);
            touch(MotionEvent.ACTION_POINTER_DOWN | (1 << MotionEvent.ACTION_POINTER_INDEX_SHIFT),
                new float[][]{{30, 180}, {440, 210}});
            touch(MotionEvent.ACTION_MOVE, new float[][]{{60, 180}, {440, 210}});
            SystemClock.sleep(500);
            JSONObject together = state();
            require(together.getInt("X") > before.getInt("X") + 10,
                "simultaneous stick movement did not reach the Go game");
            require(together.getInt("Shots") > 0,
                "simultaneous fire did not reach the Go game");
            screenshot("android-touch.png");
            touch(MotionEvent.ACTION_POINTER_UP | (1 << MotionEvent.ACTION_POINTER_INDEX_SHIFT),
                new float[][]{{60, 180}, {440, 210}});
            touch(MotionEvent.ACTION_UP, new float[][]{{60, 180}});
            SystemClock.sleep(150);

            key(KeyEvent.KEYCODE_HOME);
            SystemClock.sleep(500);
            getTargetContext().startActivity(launch);
            SystemClock.sleep(500);
            JSONObject paused = state();
            SystemClock.sleep(500);
            JSONObject held = state();
            require(held.getInt("Frame") == paused.getInt("Frame"),
                "gameplay advanced after an Android suspend/resume");
            require(held.getInt("X") == paused.getInt("X"),
                "a cancelled gesture moved the ship after resume");
            screenshot("android-paused.png");
            tap(240, 210);
            SystemClock.sleep(500);
            JSONObject resumed = state();
            require(resumed.getInt("Frame") > held.getInt("Frame"),
                "touch resume did not continue gameplay");

            JSONObject report = new JSONObject();
            report.put("before", before);
            report.put("simultaneous_touch", together);
            report.put("paused", paused);
            report.put("held_after_resume", held);
            report.put("resumed", resumed);
            report.put("success", true);
            write("android-check.json", report.toString(2).getBytes(StandardCharsets.UTF_8));
            result.putString("success", "true");
            result.putString("checks", "simultaneous touch, cancelled input, suspend, paused resume, touch continuation");
            finish(Activity.RESULT_OK, result);
        } catch (Throwable error) {
            result.putString("success", "false");
            result.putString("error", error.toString());
            error.printStackTrace();
            finish(Activity.RESULT_CANCELED, result);
        }
    }

    private JSONObject waitForState(boolean playing) throws Exception {
        long deadline = SystemClock.uptimeMillis() + 15000;
        do {
            JSONObject snapshot = state();
            if (snapshot.has("Mode") && (!playing || (snapshot.getInt("Mode") == 1
                    && snapshot.getInt("Respawn") == 0))) return snapshot;
            SystemClock.sleep(100);
        } while (SystemClock.uptimeMillis() < deadline);
        throw new AssertionError("the Go game did not reach its expected state: " + Mobile.verificationState());
    }

    private JSONObject state() throws Exception { return new JSONObject(Mobile.verificationState()); }

    private void tap(float x, float y) {
        downTime = SystemClock.uptimeMillis();
        touch(MotionEvent.ACTION_DOWN, new float[][]{{x, y}});
        SystemClock.sleep(100);
        touch(MotionEvent.ACTION_UP, new float[][]{{x, y}});
        SystemClock.sleep(100);
    }

    private void touch(int action, float[][] points) {
        MotionEvent.PointerProperties[] properties = new MotionEvent.PointerProperties[points.length];
        MotionEvent.PointerCoords[] coordinates = new MotionEvent.PointerCoords[points.length];
        for (int index = 0; index < points.length; index++) {
            properties[index] = new MotionEvent.PointerProperties();
            properties[index].id = index;
            properties[index].toolType = MotionEvent.TOOL_TYPE_FINGER;
            coordinates[index] = new MotionEvent.PointerCoords();
            coordinates[index].x = offsetX + points[index][0] * scale;
            coordinates[index].y = offsetY + points[index][1] * scale;
            coordinates[index].pressure = 1;
            coordinates[index].size = 1;
        }
        MotionEvent event = MotionEvent.obtain(downTime, SystemClock.uptimeMillis(), action,
            points.length, properties, coordinates, 0, 0, 1, 1, 0, 0, InputDevice.SOURCE_TOUCHSCREEN, 0);
        require(getUiAutomation().injectInputEvent(event, true), "Android rejected an injected touch");
        event.recycle();
    }

    private void key(int code) {
        long time = SystemClock.uptimeMillis();
        getUiAutomation().injectInputEvent(new KeyEvent(time, time, KeyEvent.ACTION_DOWN, code, 0), true);
        getUiAutomation().injectInputEvent(new KeyEvent(time, SystemClock.uptimeMillis(), KeyEvent.ACTION_UP, code, 0), true);
    }

    private void screenshot(String name) throws Exception {
        Bitmap image = getUiAutomation().takeScreenshot();
        require(image != null, "Android screenshot is unavailable");
        try (FileOutputStream output = new FileOutputStream(new File(getTargetContext().getFilesDir(), name))) {
            require(image.compress(Bitmap.CompressFormat.PNG, 100, output), "Android could not encode its screenshot");
        }
        image.recycle();
    }

    private void write(String name, byte[] data) throws Exception {
        try (FileOutputStream output = new FileOutputStream(new File(getTargetContext().getFilesDir(), name))) {
            output.write(data);
        }
    }

    private static void require(boolean condition, String message) {
        if (!condition) throw new AssertionError(message);
    }
}
`
