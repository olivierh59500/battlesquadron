package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// checkDevice installs only this application's packages on the selected device.
func (c *configuration) checkDevice(apk string) error {
	args := []string{"--no-daemon", "--console=plain", "-p", c.project, ":app:assembleDebugAndroidTest"}
	if c.offline {
		args = append(args, "--offline")
	}
	if err := c.command(c.root, c.gradle, args...); err != nil {
		return err
	}
	adb := filepath.Join(c.sdk, "platform-tools", "adb")
	captures := filepath.Join(c.root, "captures")
	var apkSHA string
	if c.physicalCheck {
		var err error
		apkSHA, err = apkChecksum(apk)
		if err != nil {
			return err
		}
		run := apkSHA[:12] + "-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		captures = filepath.Join(captures, "pixel-10a", run)
		release, err := acquireDeviceLease(c.serial)
		if err != nil {
			return err
		}
		defer release()
	}
	// Keep the foreground test window finite, including package installation.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	commandOutput := func(arguments ...string) ([]byte, error) {
		command := exec.CommandContext(ctx, adb, append([]string{"-s", c.serial}, arguments...)...)
		command.Env, command.Dir = c.environment, c.root
		return command.CombinedOutput()
	}
	if state, err := commandOutput("get-state"); err != nil || strings.TrimSpace(string(state)) != "device" {
		return fmt.Errorf("selected Android device is unavailable: %s (%v)", strings.TrimSpace(string(state)), err)
	}
	if !c.physicalCheck {
		// A fresh emulator's fullscreen hint intercepts its first gesture.
		if output, err := commandOutput("shell", "settings", "put", "secure", "immersive_mode_confirmations", "confirmed"); err != nil {
			return fmt.Errorf("dismiss emulator hint: %s (%w)", output, err)
		}
	}
	if err := os.MkdirAll(captures, 0755); err != nil {
		return err
	}
	if c.physicalCheck {
		identity := map[string]string{"serial": c.serial, "started_utc": time.Now().UTC().Format(time.RFC3339)}
		identity["apk_sha256"] = apkSHA
		for key, property := range map[string]string{"model": "ro.product.model", "android_release": "ro.build.version.release", "api": "ro.build.version.sdk"} {
			value, err := commandOutput("shell", "getprop", property)
			if err != nil {
				return err
			}
			identity[key] = strings.TrimSpace(string(value))
		}
		if pageSize, err := commandOutput("shell", "getconf", "PAGE_SIZE"); err == nil {
			identity["memory_page_bytes"] = strings.TrimSpace(string(pageSize))
		}
		if activity, err := commandOutput("shell", "dumpsys", "activity", "activities"); err == nil {
			identity["previous_foreground"] = foregroundActivity(string(activity))
		}
		data, _ := json.MarshalIndent(identity, "", "  ")
		if err := os.WriteFile(filepath.Join(captures, "device.json"), data, 0644); err != nil {
			return err
		}
		if identity["previous_foreground"] == "" || identity["previous_foreground"] == "unknown" {
			return fmt.Errorf("shared Android device foreground is unavailable; refusing package installation and launch")
		}
	}
	testAPK := filepath.Join(c.project, "app", "build", "outputs", "apk", "androidTest", "debug", "app-debug-androidTest.apk")
	for _, artifact := range []string{apk, testAPK} {
		output, err := commandOutput("install", "-r", artifact)
		fmt.Print(string(output))
		if err != nil {
			return fmt.Errorf("install Battle Squadron package: %w", err)
		}
	}
	instrument := []string{"-s", c.serial, "shell", "am", "instrument", "-w"}
	if c.physicalCheck {
		instrument = append(instrument, "-e", "physical_device", "true")
	}
	if c.performance {
		instrument = append(instrument, "-e", "performance", "true")
	}
	if c.novaPerformance {
		instrument = append(instrument, "-e", "nova_performance", "true")
	}
	instrument = append(instrument, applicationID+".test/"+applicationID+".TouchRunner")
	command := exec.CommandContext(ctx, adb, instrument...)
	command.Env, command.Dir = c.environment, c.root
	result, err := command.CombinedOutput()
	fmt.Print(string(result))
	if writeErr := os.WriteFile(filepath.Join(captures, "android-instrumentation.txt"), result, 0644); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("Android instrumentation: %w", err)
	}
	if !strings.Contains(string(result), "INSTRUMENTATION_RESULT: success=true") {
		return fmt.Errorf("Android instrumentation did not report success")
	}
	names := []string{"android-title.png", "android-demo.png", "android-demo-wake.png", "android-demo-performance.json", "android-touch.png", "android-paused.png", "android-nova.png", "android-cave.png", "android-final.png", "android-check.json"}
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
		pull := exec.CommandContext(ctx, adb, "-s", c.serial, "exec-out", "run-as", applicationID, "cat", "files/"+name)
		pull.Env, pull.Stdout, pull.Stderr = c.environment, file, os.Stderr
		pullErr, closeErr := pull.Run(), file.Close()
		if pullErr != nil {
			return fmt.Errorf("retrieve %s: %w", name, pullErr)
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Println("Android evidence:", path)
	}
	if c.physicalCheck {
		activity, err := commandOutput("shell", "dumpsys", "activity", "activities")
		foreground := foregroundActivity(string(activity))
		if err == nil && (strings.Contains(foreground, applicationID+"/") || strings.Contains(foreground, "com.android.launcher3/")) {
			if output, err := commandOutput("shell", "am", "force-stop", applicationID); err != nil {
				return fmt.Errorf("close Battle Squadron diagnostic process: %s (%w)", output, err)
			}
			if output, err := commandOutput("shell", "am", "start", "-n", applicationID+"/.MainActivity"); err != nil {
				return fmt.Errorf("launch normal Battle Squadron menu: %s (%w)", output, err)
			}
			fmt.Println("Pixel ready at the normal Battle Squadron menu.")
		} else {
			fmt.Println("Shared device foreground changed; preserving its current activity.")
		}
	}
	return nil
}

func apkChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, hashErr := io.Copy(hash, file)
	closeErr := file.Close()
	if hashErr != nil {
		return "", hashErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// acquireDeviceLease advertises a bounded device window to cooperating agents.
// A foreground guard also stops injections if an unrelated application takes over.
func acquireDeviceLease(serial string) (func(), error) {
	directory := filepath.Join(os.TempDir(), "codex-android-device-"+strings.ReplaceAll(serial, ":", "_")+".lock")
	if err := os.Mkdir(directory, 0700); err != nil {
		return nil, fmt.Errorf("shared Android device lease is unavailable at %s: %w", directory, err)
	}
	owner := filepath.Join(directory, "owner.txt")
	contents := fmt.Sprintf("Battle Squadron Android validation\npid=%d\nserial=%s\nexpires_utc=%s\n", os.Getpid(), serial, time.Now().Add(5*time.Minute).UTC().Format(time.RFC3339))
	if err := os.WriteFile(owner, []byte(contents), 0600); err != nil {
		_ = os.Remove(directory)
		return nil, err
	}
	return func() {
		if actual, err := os.ReadFile(owner); err == nil && string(actual) == contents {
			_ = os.Remove(owner)
			_ = os.Remove(directory)
		}
	}, nil
}

func foregroundActivity(dump string) string {
	for _, line := range strings.Split(dump, "\n") {
		if strings.Contains(line, "topResumedActivity=") {
			return strings.TrimSpace(line)
		}
	}
	return "unknown"
}

// Android's input-injection and lifecycle APIs require this generated native
// instrumentation bridge. Assertions concern the actual shared Go game state.
const instrumentationRunner = `package com.olivierh.battlesquadron;

import android.app.Activity;
import android.app.Instrumentation;
import android.content.Intent;
import android.graphics.Bitmap;
import android.os.Bundle;
import android.os.ParcelFileDescriptor;
import android.os.SystemClock;
import android.view.InputDevice;
import android.view.KeyEvent;
import android.view.MotionEvent;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import org.json.JSONObject;
import com.olivierh.battlesquadron.mobile.Mobile;

/** Generated package-specific checks for idle demonstrations, touch and lifecycle. */
public final class TouchRunner extends Instrumentation {
    private long downTime;
    private float scale, offsetX, offsetY;
    private boolean measurePerformance;
    private boolean measureNova;
    private boolean physicalDevice;
    private long testDeadline;

    @Override public void onCreate(Bundle arguments) {
        super.onCreate(arguments);
        measurePerformance = arguments != null && "true".equals(arguments.getString("performance"));
        measureNova = arguments != null && "true".equals(arguments.getString("nova_performance"));
        physicalDevice = arguments != null && "true".equals(arguments.getString("physical_device"));
        start();
    }

    @Override public void onStart() {
        testDeadline = SystemClock.uptimeMillis() + 180000;
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
            JSONObject initialTitle = state();
            require(!initialTitle.getBoolean("Demo"), "the initial menu skipped its idle delay");
            JSONObject demo = waitForDemo(true);
            require(demo.getInt("Mode") == 1, "the idle demonstration did not enter native gameplay");
            int demoFrame = demo.getInt("Frame");
            Mobile.startPerformanceMeasurement();
            // Exercise the actual input-only expert demonstration on the device.
            SystemClock.sleep(12000);
            Mobile.finishPerformanceMeasurement();
            JSONObject demoPerformance = awaitPerformance();
            JSONObject runningDemo = state();
            require(runningDemo.getBoolean("Demo") && runningDemo.getInt("Frame") > demoFrame + 400,
                "the expert demonstration did not advance while awaiting input");
            demoPerformance.put("native_start", demo);
            demoPerformance.put("native_end", runningDemo);
            write("android-demo-performance.json", demoPerformance.toString(2).getBytes(StandardCharsets.UTF_8));
            screenshot("android-demo.png");

            key(KeyEvent.KEYCODE_HOME);
            SystemClock.sleep(500);
            getTargetContext().startActivity(launch);
            SystemClock.sleep(500);
            JSONObject demoAfterResume = state();
            require(!demoAfterResume.getBoolean("Demo") && demoAfterResume.getInt("Mode") == 0,
                "suspending the demonstration did not restore the human menu");
            require(demoAfterResume.getInt("MenuIdleTicks") < 100,
                "demonstration suspension did not reset the foreground idle delay");
            waitForDemo(true);

            downTime = SystemClock.uptimeMillis();
            touch(MotionEvent.ACTION_DOWN, new float[][]{{240, 210}});
            SystemClock.sleep(400);
            JSONObject waking = state();
            require(!waking.getBoolean("Demo") && waking.getInt("Mode") == 0,
                "touching the demonstration did not return to the menu");
            require(waking.getBoolean("DemoInputBlocked"), "the held wake touch was not quarantined");
            touch(MotionEvent.ACTION_UP, new float[][]{{240, 210}});
            SystemClock.sleep(250);
            JSONObject awake = state();
            require(awake.getInt("Mode") == 0 && !awake.getBoolean("DemoInputBlocked"),
                "the wake touch accidentally started a session or stayed blocked after release");
            screenshot("android-demo-wake.png");
            tap(240, 210);
            JSONObject before = waitForState(true);
            require(!before.getBoolean("Demo"), "a fresh menu touch did not start a human session");

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
            report.put("initial_title", initialTitle);
            report.put("demo", demo);
            report.put("demo_progress", runningDemo);
            report.put("demo_after_suspend_resume", demoAfterResume);
            report.put("held_wake_touch", waking);
            report.put("released_wake_touch", awake);
            report.put("before", before);
            report.put("simultaneous_touch", together);
            report.put("paused", paused);
            report.put("held_after_resume", held);
            report.put("resumed", resumed);
            report.put("nova", nova);
            report.put("success", true);
            write("android-check.json", report.toString(2).getBytes(StandardCharsets.UTF_8));
            result.putString("success", "true");
            result.putString("checks", "idle expert demo, held wake quarantine, real demo performance, simultaneous touch, cancelled input, suspend, paused resume, touch continuation");
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

    private JSONObject state() throws Exception {
        requireOwnForeground();
        return new JSONObject(Mobile.verificationState());
    }

    private JSONObject waitForDemo(boolean active) throws Exception {
        long deadline = SystemClock.uptimeMillis() + 22000;
        do {
            JSONObject snapshot = state();
            if (snapshot.optBoolean("Demo") == active) return snapshot;
            SystemClock.sleep(100);
        } while (SystemClock.uptimeMillis() < deadline);
        throw new AssertionError("the menu did not reach its expected demonstration state: " + Mobile.verificationState());
    }

    private void requireOwnForeground() throws Exception {
        require(SystemClock.uptimeMillis() < testDeadline, "the bounded Android test window expired");
        if (!physicalDevice) return;
        ParcelFileDescriptor pipe = getUiAutomation().executeShellCommand("dumpsys activity activities");
        String dump;
        try (InputStream input = new ParcelFileDescriptor.AutoCloseInputStream(pipe)) {
            java.io.ByteArrayOutputStream bytes = new java.io.ByteArrayOutputStream();
            byte[] buffer = new byte[4096];
            int count;
            while ((count = input.read(buffer)) >= 0) bytes.write(buffer, 0, count);
            dump = bytes.toString("UTF-8");
        }
        for (String line : dump.split("\n")) {
            if (line.contains("topResumedActivity=")) {
                require(line.contains("com.olivierh.battlesquadron/"),
                    "shared device foreground changed; stopping injections: " + line.trim());
                return;
            }
        }
        throw new AssertionError("shared device foreground is unavailable; stopping injections");
    }

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

    private void tap(float x, float y) throws Exception {
        downTime = SystemClock.uptimeMillis();
        touch(MotionEvent.ACTION_DOWN, new float[][]{{x, y}});
        SystemClock.sleep(100);
        touch(MotionEvent.ACTION_UP, new float[][]{{x, y}});
        SystemClock.sleep(100);
    }

    private void touch(int action, float[][] points) throws Exception {
        requireOwnForeground();
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

    private void key(int code) throws Exception {
        requireOwnForeground();
        long time = SystemClock.uptimeMillis();
        getUiAutomation().injectInputEvent(new KeyEvent(time, time, KeyEvent.ACTION_DOWN, code, 0), true);
        getUiAutomation().injectInputEvent(new KeyEvent(time, SystemClock.uptimeMillis(), KeyEvent.ACTION_UP, code, 0), true);
    }

    private void screenshot(String name) throws Exception {
        requireOwnForeground();
        Bitmap image = getUiAutomation().takeScreenshot();
        require(image != null, "Android screenshot is unavailable");
        try {
            requireOwnForeground();
            try (FileOutputStream output = new FileOutputStream(new File(getTargetContext().getFilesDir(), name))) {
                require(image.compress(Bitmap.CompressFormat.PNG, 100, output), "Android could not encode its screenshot");
            }
        } finally {
            image.recycle();
        }
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
