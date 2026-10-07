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
	instrument := []string{"-s", c.serial, "shell", "am", "instrument", "-w"}
	if c.performance {
		instrument = append(instrument, "-e", "performance", "true")
	}
	if c.novaPerformance {
		instrument = append(instrument, "-e", "nova_performance", "true")
	}
	instrument = append(instrument, applicationID+".test/"+applicationID+".TouchRunner")
	command := exec.Command(adb, instrument...)
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
	names := []string{"android-title.png", "android-touch.png", "android-paused.png", "android-nova.png", "android-cave.png", "android-final.png", "android-check.json"}
	if c.performance {
		names = append(names, "android-performance.json", "android-smooth.png")
	}
	if c.novaPerformance {
		names = append(names, "android-nova-performance.json", "android-nova-restored.png")
	}
	for _, name := range names {
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
    private boolean measurePerformance;
    private boolean measureNova;

    @Override public void onCreate(Bundle arguments) {
        super.onCreate(arguments);
        measurePerformance = arguments != null && "true".equals(arguments.getString("performance"));
        measureNova = arguments != null && "true".equals(arguments.getString("nova_performance"));
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
            require(together.getInt("Fired") > before.getInt("Fired"),
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

            int charges = resumed.getInt("Nova");
            tap(440, 126);
            SystemClock.sleep(150);
            JSONObject nova = state();
            require(nova.getInt("Nova") < charges, "the Nova touch did not consume its original charge");
            screenshot("android-nova.png");

            Mobile.setVerificationScene(1);
            SystemClock.sleep(8000);
            screenshot("android-cave.png");
            Mobile.setVerificationScene(3);
            SystemClock.sleep(3000);
            screenshot("android-final.png");

            if (measurePerformance) {
                JSONObject performance = new JSONObject();
                performance.put("surface_original_cadence", measure(4, false));
                performance.put("surface_smooth", measure(4, true));
                screenshot("android-smooth.png");
                performance.put("cave_one_smooth", measure(1, true));
                performance.put("cave_two_smooth", measure(2, true));
                performance.put("cave_three_smooth", measure(5, true));
                performance.put("final_smooth", measure(3, true));
                write("android-performance.json", performance.toString(2).getBytes(StandardCharsets.UTF_8));
            }
            if (measureNova) {
                Mobile.setVerificationScene(4);
                Mobile.setVerificationSmoothRendering(true);
                SystemClock.sleep(500);
                Mobile.startPerformanceMeasurement();
                SystemClock.sleep(2500);
                JSONObject novaBefore = state();
                tap(440, 126);
                SystemClock.sleep(300);
                JSONObject novaVisible = state();
                require(novaVisible.getInt("Nova") < novaBefore.getInt("Nova"),
                    "restored Nova did not consume its native charge");
                require(novaVisible.getInt("NovaFrames") > 0,
                    "restored Nova was not active during its actual Android capture");
                screenshot("android-nova-restored.png");
                SystemClock.sleep(8500);
                Mobile.finishPerformanceMeasurement();
                JSONObject performance = awaitPerformance();
                performance.put("native_before_nova", novaBefore);
                performance.put("native_visible_nova", novaVisible);
                performance.put("native_after_measurement", state());
                write("android-nova-performance.json", performance.toString(2).getBytes(StandardCharsets.UTF_8));
            }

            JSONObject report = new JSONObject();
            report.put("before", before);
            report.put("simultaneous_touch", together);
            report.put("paused", paused);
            report.put("held_after_resume", held);
            report.put("resumed", resumed);
            report.put("nova", nova);
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

    private JSONObject measure(int scene, boolean smooth) throws Exception {
        Mobile.setVerificationScene(scene);
        Mobile.setVerificationSmoothRendering(smooth);
        SystemClock.sleep(500);
        Mobile.startPerformanceMeasurement();
        // Two seconds warm up, then ten seconds of actual presented frames.
        SystemClock.sleep(12000);
        Mobile.finishPerformanceMeasurement();
        JSONObject report = awaitPerformance();
        report.put("scene", scene);
        report.put("smooth", smooth);
        return report;
    }

    private JSONObject awaitPerformance() throws Exception {
        long deadline = SystemClock.uptimeMillis() + 5000;
        do {
            JSONObject report = new JSONObject(Mobile.performanceState());
            if (report.optInt("draws") >= 100) {
                require(report.getDouble("elapsed_seconds") >= 9,
                    "the performance sample was shorter than the requested steady interval");
                require(report.getInt("simulation_tps") == 50,
                    "rendering diagnostics changed native PAL simulation time");
                return report;
            }
            SystemClock.sleep(100);
        } while (SystemClock.uptimeMillis() < deadline);
        throw new AssertionError("the Go performance measurement was not published");
    }

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
