// Command android builds an Android APK from the shared Go game.
// The native activity and Gradle project are generated build artifacts.
package main

import (
	"archive/zip"
	"bytes"
	"debug/elf"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	applicationID = "com.olivierh.battlesquadron"
	compileSDK    = "36"
	buildTools    = "36.0.0"
	minimumSDK    = "23"
	gradleVersion = "8.13"
)

type configuration struct {
	root, project, cache, sdk, ndk, java, gradle    string
	target, serial, seed                            string
	prepare, skipBind, aarOnly, run, offline, check bool
	performance                                     bool
	novaPerformance                                 bool
	environment                                     []string
}

func main() {
	var cfg configuration
	flag.StringVar(&cfg.target, "target", "android/arm64", "Ebitengine targets, for example android/arm64,android/amd64")
	flag.StringVar(&cfg.sdk, "sdk", "", "Android SDK directory (defaults to ANDROID_HOME)")
	flag.StringVar(&cfg.ndk, "ndk", "", "Android NDK directory (defaults to SDK NDK 28.2.13676358)")
	flag.StringVar(&cfg.java, "java-home", "", "Java 17 or 21 home directory")
	flag.StringVar(&cfg.gradle, "gradle", "", "Gradle 8.13 executable; PATH and local caches are searched")
	flag.StringVar(&cfg.seed, "seed-cache", "", "Copy Go module and Gradle dependency caches from an existing Android cache")
	flag.StringVar(&cfg.serial, "serial", "", "ADB device serial used with -run")
	flag.BoolVar(&cfg.prepare, "prepare", false, "Generate the Android host project without compiling")
	flag.BoolVar(&cfg.skipBind, "skip-bind", false, "Reuse an already generated Go AAR")
	flag.BoolVar(&cfg.aarOnly, "aar-only", false, "Build the Go AAR without assembling an APK")
	flag.BoolVar(&cfg.run, "run", false, "Install and launch the successfully verified debug APK")
	flag.BoolVar(&cfg.offline, "offline", false, "Disable dependency downloads after seeding existing caches")
	flag.BoolVar(&cfg.check, "check", false, "Run generated simultaneous-touch and lifecycle checks on an emulator selected by -serial")
	flag.BoolVar(&cfg.performance, "performance", false, "Include real frame-time and smooth-motion measurements with -check")
	flag.BoolVar(&cfg.novaPerformance, "nova-performance", false, "Include a short restored-Nova rendering measurement with -check")
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := cfg.build(); err != nil {
		fmt.Fprintln(os.Stderr, "Android build:", err)
		os.Exit(1)
	}
}

func (c *configuration) build() error {
	var err error
	c.root, err = projectRoot()
	if err != nil {
		return err
	}
	c.project = filepath.Join(c.root, "android", "generated")
	c.cache = filepath.Join(c.root, ".cache", "android")
	if c.check && !strings.HasPrefix(c.serial, "emulator-") {
		return errors.New("-check requires -serial emulator-NNNN and never installs on physical devices")
	}
	if c.performance && !c.check {
		return errors.New("-performance requires -check")
	}
	if c.novaPerformance && !c.check {
		return errors.New("-nova-performance requires -check")
	}
	if err := c.discover(); err != nil {
		return err
	}
	if c.seed != "" {
		seed, err := filepath.Abs(c.seed)
		if err != nil {
			return err
		}
		if seed == c.cache {
			return errors.New("the cache seed must be a different directory")
		}
		for _, relative := range []string{"go/pkg/mod", "gradle/caches/modules-2"} {
			fmt.Println("Seeding cached dependencies:", relative)
			if err := copyTree(filepath.Join(seed, relative), filepath.Join(c.cache, relative)); err != nil {
				return err
			}
		}
	}
	if err := c.generate(); err != nil {
		return err
	}
	if c.prepare {
		fmt.Println("Generated Android host:", c.project)
		return nil
	}
	if !c.skipBind {
		if err := c.bind(); err != nil {
			return err
		}
	}
	aar := filepath.Join(c.project, "app", "libs", "battlesquadron.aar")
	if !regularFile(aar) {
		return fmt.Errorf("Go AAR is missing: %s", aar)
	}
	if c.aarOnly {
		fmt.Println("Android library:", aar)
		return nil
	}
	if c.gradle == "" {
		return errors.New("Gradle 8.13 was not found; set -gradle to its executable")
	}
	key := filepath.Join(c.cache, "debug.keystore")
	if !regularFile(key) {
		if err := c.command(c.root, filepath.Join(c.java, "bin", "keytool"), "-genkeypair", "-keystore", key,
			"-storetype", "PKCS12", "-storepass", "android", "-alias", "androiddebugkey", "-keypass", "android",
			"-dname", "CN=Android Debug,O=Android,C=US", "-keyalg", "RSA", "-keysize", "2048", "-validity", "10000"); err != nil {
			return err
		}
	}
	args := []string{"--no-daemon", "--console=plain", "-p", c.project, ":app:assembleDebug"}
	if c.offline {
		args = append(args, "--offline")
	}
	if err := c.command(c.root, c.gradle, args...); err != nil {
		return err
	}
	apk := filepath.Join(c.project, "app", "build", "outputs", "apk", "debug", "app-debug.apk")
	if err := c.verify(apk); err != nil {
		return err
	}
	output := filepath.Join(c.root, "bin", "battlesquadron-debug.apk")
	if err := copyFile(apk, output, 0644); err != nil {
		return err
	}
	fmt.Println("Verified Android APK:", output)
	if c.check {
		return c.checkEmulator(output)
	}
	if c.run {
		adb := filepath.Join(c.sdk, "platform-tools", "adb")
		var device []string
		if c.serial != "" {
			device = []string{"-s", c.serial}
		}
		if err := c.command(c.root, adb, append(device, "install", "-r", output)...); err != nil {
			return err
		}
		return c.command(c.root, adb, append(device, "shell", "am", "start", "-n", applicationID+"/.MainActivity")...)
	}
	return nil
}

func projectRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(current, "go.mod"))
		if err == nil && bytes.Contains(data, []byte("module github.com/olivierh59500/battlesquadron")) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("run this command from the Battle Squadron checkout")
		}
		current = parent
	}
}

func (c *configuration) discover() error {
	userHome, _ := os.UserHomeDir()
	c.sdk = firstDirectory(c.sdk, os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"),
		"/opt/homebrew/share/android-commandlinetools", filepath.Join(userHome, "Library", "Android", "sdk"), filepath.Join(userHome, "Android", "Sdk"))
	if !regularFile(filepath.Join(c.sdk, "platforms", "android-"+compileSDK, "android.jar")) {
		return errors.New("Android SDK platform 36 is required; set -sdk or ANDROID_HOME")
	}
	c.ndk = firstDirectory(c.ndk, os.Getenv("ANDROID_NDK_HOME"), filepath.Join(c.sdk, "ndk", "28.2.13676358"))
	if !regularFile(filepath.Join(c.ndk, "meta", "platforms.json")) {
		return errors.New("Android NDK 28.2.13676358 is required; set -ndk or ANDROID_NDK_HOME")
	}
	c.java = firstDirectory(c.java, os.Getenv("JAVA_HOME"), "/opt/homebrew/opt/openjdk@21", "/opt/homebrew/opt/openjdk@17",
		"/Applications/Android Studio.app/Contents/jbr/Contents/Home")
	if c.java == "" && runtime.GOOS == "linux" {
		if executable, err := exec.LookPath("java"); err == nil {
			if resolved, err := filepath.EvalSymlinks(executable); err == nil {
				c.java = filepath.Dir(filepath.Dir(resolved))
			}
		}
	}
	if !regularFile(filepath.Join(c.java, "bin", "java")) {
		return errors.New("Java 17 or 21 is required; set -java-home or JAVA_HOME")
	}
	for _, tool := range []string{"aapt", "apksigner", "zipalign"} {
		if !regularFile(filepath.Join(c.sdk, "build-tools", buildTools, tool)) {
			return fmt.Errorf("Android Build Tools %s is incomplete: %s", buildTools, tool)
		}
	}
	if c.gradle == "" {
		c.gradle, _ = exec.LookPath("gradle")
	}
	if c.gradle == "" {
		patterns := []string{
			filepath.Join(userHome, ".gradle", "wrapper", "dists", "gradle-"+gradleVersion+"-bin", "*", "gradle-"+gradleVersion, "bin", "gradle"),
			filepath.Join(filepath.Dir(c.root), "*", ".cache", "android", "gradle", "wrapper", "dists", "gradle-"+gradleVersion+"-bin", "*", "gradle-"+gradleVersion, "bin", "gradle"),
		}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			if len(matches) != 0 {
				c.gradle = matches[0]
				break
			}
		}
	}
	c.environment = environment(os.Environ(), map[string]string{
		"ANDROID_HOME": c.sdk, "ANDROID_SDK_ROOT": c.sdk, "ANDROID_NDK_HOME": c.ndk,
		"ANDROID_USER_HOME": filepath.Join(c.cache, "sdk-user"), "JAVA_HOME": c.java,
		"PATH":   filepath.Join(c.java, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
		"GOWORK": "off", "GOTOOLCHAIN": "local", "GOPATH": filepath.Join(c.cache, "go"),
		"GOMODCACHE": filepath.Join(c.cache, "go", "pkg", "mod"), "GOCACHE": filepath.Join(c.root, ".cache", "go-build"),
		"GRADLE_USER_HOME": filepath.Join(c.cache, "gradle"),
	})
	if c.offline {
		c.environment = environment(c.environment, map[string]string{"GOPROXY": "off", "GOSUMDB": "off"})
	}
	return nil
}

func (c *configuration) generate() error {
	for _, dir := range []string{c.cache, filepath.Join(c.project, "app", "libs"), filepath.Join(c.cache, "gradle")} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	files := map[string]string{
		"settings.gradle":                  settingsGradle,
		"build.gradle":                     "plugins { id 'com.android.application' version '8.10.1' apply false }\n",
		"gradle.properties":                "org.gradle.jvmargs=-Xmx2048m -Dfile.encoding=UTF-8\nandroid.useAndroidX=false\n",
		"app/build.gradle":                 strings.ReplaceAll(appGradle, "DEBUG_KEY", strconv.Quote(filepath.Join(c.cache, "debug.keystore"))),
		"app/src/main/AndroidManifest.xml": androidManifest,
		"app/src/main/java/com/olivierh/battlesquadron/MainActivity.java":       nativeActivity,
		"app/src/androidTest/java/com/olivierh/battlesquadron/TouchRunner.java": instrumentationRunner,
	}
	for relative, data := range files {
		path := filepath.Join(c.project, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (c *configuration) bind() error {
	// Read the game's pinned Ebitengine version rather than maintaining two pins.
	command := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/hajimehoshi/ebiten/v2")
	command.Dir, command.Env = c.root, c.environment
	versionData, err := command.Output()
	if err != nil {
		return fmt.Errorf("read pinned Ebitengine version: %w", err)
	}
	version := strings.TrimSpace(string(versionData))
	tools := filepath.Join(c.cache, "bind-tools")
	if err := os.MkdirAll(tools, 0755); err != nil {
		return err
	}
	module := "module battlesquadron-android-tools\n\ngo 1.26.0\n\nrequire github.com/hajimehoshi/ebiten/v2 " + version + "\n"
	if err := os.WriteFile(filepath.Join(tools, "go.mod"), []byte(module), 0644); err != nil {
		return err
	}
	binder := filepath.Join(tools, "ebitenmobile")
	// The host tools disable Cgo; the binder enables it for the Android JNI library.
	buildEnvironment := environment(c.environment, map[string]string{"CGO_ENABLED": "0"})
	if err := execute(tools, buildEnvironment, "go", "build", "-mod=mod", "-o", binder, "github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile"); err != nil {
		return err
	}
	linkFlags := strings.TrimSpace(os.Getenv("CGO_LDFLAGS") + " -Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384")
	bindEnvironment := environment(buildEnvironment, map[string]string{"CGO_LDFLAGS": linkFlags})
	fmt.Println("Binding the shared Go game for", c.target)
	return execute(c.root, bindEnvironment, binder, "bind", "-target", c.target, "-androidapi", minimumSDK,
		"-javapkg", applicationID, "-o", filepath.Join(c.project, "app", "libs", "battlesquadron.aar"), "./mobile")
}

func (c *configuration) verify(apk string) error {
	tools := filepath.Join(c.sdk, "build-tools", buildTools)
	metadata := exec.Command(filepath.Join(tools, "aapt"), "dump", "badging", apk)
	metadata.Env = c.environment
	badging, err := metadata.Output()
	if err != nil {
		return fmt.Errorf("read APK metadata: %w", err)
	}
	for _, expected := range []string{"package: name='" + applicationID + "'", "sdkVersion:'" + minimumSDK + "'",
		"targetSdkVersion:'" + compileSDK + "'", "uses-gl-es: '0x30000'", "launchable-activity: name='" + applicationID + ".MainActivity'"} {
		if !bytes.Contains(badging, []byte(expected)) {
			return fmt.Errorf("APK metadata is missing %s", expected)
		}
	}
	if err := c.command(c.root, filepath.Join(tools, "apksigner"), "verify", apk); err != nil {
		return err
	}
	if err := c.command(c.root, filepath.Join(tools, "zipalign"), "-c", "-P", "16", "4", apk); err != nil {
		return err
	}
	return verifyNativeAlignment(apk)
}

// Archive alignment and ELF alignment both matter on Android's 16 KiB devices.
func verifyNativeAlignment(apk string) error {
	archive, err := zip.OpenReader(apk)
	if err != nil {
		return err
	}
	defer archive.Close()
	libraries := 0
	for _, entry := range archive.File {
		if !strings.HasPrefix(entry.Name, "lib/") || !strings.HasSuffix(entry.Name, ".so") {
			continue
		}
		libraries++
		reader, err := entry.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return err
		}
		library, err := elf.NewFile(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", entry.Name, err)
		}
		if err := verifySegments(library.Progs); err != nil {
			return fmt.Errorf("%s: %w", entry.Name, err)
		}
	}
	if libraries == 0 {
		return errors.New("APK contains no native Go library")
	}
	return nil
}

func verifySegments(programs []*elf.Prog) error {
	loads := 0
	for _, segment := range programs {
		switch segment.Type {
		case elf.PT_LOAD:
			loads++
			if segment.Align < 16384 {
				return fmt.Errorf("ELF LOAD alignment is %d; at least 16384 is required", segment.Align)
			}
		case elf.PT_GNU_RELRO:
			if (segment.Vaddr+segment.Memsz)%16384 != 0 {
				return errors.New("ELF RELRO segment does not end on a 16 KiB boundary")
			}
		}
	}
	if loads == 0 {
		return errors.New("ELF has no LOAD segments")
	}
	return nil
}

func (c *configuration) command(directory, executable string, arguments ...string) error {
	return execute(directory, c.environment, executable, arguments...)
}

func execute(directory string, env []string, executable string, arguments ...string) error {
	command := exec.Command(executable, arguments...)
	command.Dir, command.Env = directory, env
	command.Stdout, command.Stderr, command.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(executable), err)
	}
	return nil
}

func environment(base []string, overrides map[string]string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	for _, variable := range base {
		key, _, _ := strings.Cut(variable, "=")
		if _, exists := overrides[key]; !exists {
			result = append(result, variable)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func firstDirectory(candidates ...string) string {
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		// Lock files describe another process, and symlinks must not escape a cache.
		if entry.Type()&os.ModeSymlink != 0 || strings.HasSuffix(path, ".lock") || strings.HasSuffix(path, ".lck") || regularFile(target) {
			return nil
		}
		return copyFile(path, target, 0644)
	})
}

func copyFile(source, destination string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	return errors.Join(copyErr, output.Close())
}
