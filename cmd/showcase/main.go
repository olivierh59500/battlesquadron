// Command showcase records an edited native-game presentation with original audio.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/internal/game"
	"github.com/olivierh59500/battlesquadron/internal/replay"
	"github.com/olivierh59500/battlesquadron/internal/showcase"
)

const (
	videoWidth      = 1280
	videoHeight     = 960
	videoFPS        = 60
	sampleRate      = 44100
	samplesPerFrame = sampleRate / videoFPS
	samplesPerField = sampleRate / 50
	bytesPerSample  = 4 // Two little-endian signed16 channels.
)

var epoch = time.Unix(0, 0).UTC()

type configuration struct {
	output, ffmpeg string
	limit          time.Duration
	validate       bool
}

type chapterEvidence struct {
	Name             string         `json:"name"`
	Page             showcase.Page  `json:"page"`
	StartField       int            `json:"timeline_start_field"`
	PlannedFields    int            `json:"planned_fields"`
	CapturedFields   int            `json:"captured_fields"`
	SourceInputStart uint64         `json:"source_input_start"`
	StartState       game.FilmState `json:"start_state"`
	EndState         game.FilmState `json:"end_state"`
}

type evidence struct {
	SourceSHA256       string            `json:"source_sha256"`
	ProgramSHA256      string            `json:"program_sha256"`
	InputSHA256        string            `json:"input_sha256"`
	EncodedFPS         int               `json:"encoded_fps"`
	NativeFieldsPerSec int               `json:"native_fields_per_second"`
	Width              int               `json:"width"`
	Height             int               `json:"height"`
	Frames             int               `json:"frames"`
	AudioSampleRate    int               `json:"audio_sample_rate"`
	AudioChannels      int               `json:"audio_channels"`
	AudioSamples       int64             `json:"audio_sample_frames"`
	DurationSeconds    float64           `json:"duration_seconds"`
	PlannedSeconds     float64           `json:"planned_seconds"`
	EncodingSeconds    float64           `json:"encoding_wall_seconds"`
	CompletePlan       bool              `json:"complete_plan"`
	Chapters           []chapterEvidence `json:"chapters"`
	FinalState         game.FilmState    `json:"final_state"`
	Scope              string            `json:"scope"`
}

type capture struct {
	film         filmSource
	video        io.Writer
	wav          *os.File
	target       *ebiten.Image
	pixels       []byte
	pcm          [samplesPerField * bytesPerSample]byte
	totalFrames  int
	frames       int
	field        int
	audioSamples int64
	started      bool
	err          error
	chapters     []chapterEvidence
}

type filmSource interface {
	Step(showcase.Chapter, int, time.Time) error
	DrawFrame(*ebiten.Image, time.Time, []string)
	ReadAudio([]byte) (int, error)
	State() game.FilmState
}

func main() {
	c := configuration{}
	flag.StringVar(&c.output, "output", "captures/battlesquadron-presentation.mp4", "ignored MP4 presentation output")
	flag.StringVar(&c.ffmpeg, "ffmpeg", "ffmpeg", "FFmpeg executable with libx264 and AAC support")
	flag.DurationVar(&c.limit, "limit", 0, "optional preview duration, for example 12s; zero records the complete presentation")
	flag.BoolVar(&c.validate, "validate", false, "verify the complete native presentation and terminal checksum without rendering or encoding")
	flag.Parse()
	if err := run(c); err != nil {
		log.Fatal(err)
	}
}

func run(c configuration) error {
	if c.limit < 0 {
		return fmt.Errorf("preview limit must be nonnegative")
	}
	if c.validate {
		if c.limit != 0 {
			return fmt.Errorf("native validation requires the complete presentation without -limit")
		}
		return validateSource(c.output)
	}
	ffmpeg, err := exec.LookPath(c.ffmpeg)
	if err != nil {
		return err
	}
	duration := showcase.Duration()
	if c.limit > 0 && c.limit < duration {
		duration = c.limit
	}
	frames := frameCount(duration)
	if err := os.MkdirAll(filepath.Dir(c.output), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(".cache/showcase", 0755); err != nil {
		return err
	}
	work, err := os.MkdirTemp(".cache/showcase", "render-")
	if err != nil {
		return err
	}
	fmt.Printf("recording %d frames at %d FPS; temporary media: %s\n", frames, videoFPS, work)
	silent := filepath.Join(work, "video.mp4")
	audio := filepath.Join(work, "audio.wav")
	wav, err := os.Create(audio)
	if err != nil {
		return err
	}
	defer wav.Close()
	if _, err := wav.Write(make([]byte, 44)); err != nil {
		return err
	}
	encoder := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-f", "rawvideo", "-pixel_format", "rgba", "-video_size", "1280x960", "-framerate", "60", "-i", "pipe:0", "-an", "-c:v", "libx264", "-preset", "veryfast", "-crf", "18", "-pix_fmt", "yuv420p", "-movflags", "+faststart", silent)
	var encoderErrors bytes.Buffer
	encoder.Stderr = &encoderErrors
	stdin, err := encoder.StdinPipe()
	if err != nil {
		return err
	}
	if err := encoder.Start(); err != nil {
		return err
	}
	waited := false
	defer func() {
		stdin.Close()
		if !waited {
			encoder.Process.Kill()
			encoder.Wait()
		}
	}()
	film, err := game.NewFilm()
	if err != nil {
		return err
	}
	started := time.Now()
	recorder := &capture{film: film, video: stdin, wav: wav, totalFrames: frames}
	ebiten.SetWindowTitle("Battle Squadron presentation recording")
	ebiten.SetWindowSize(videoWidth/2, videoHeight/2)
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetVsyncEnabled(false)
	runErr := ebiten.RunGame(recorder)
	closeErr := stdin.Close()
	encodeErr := encoder.Wait()
	waited = true
	if encodeErr != nil {
		return fmt.Errorf("encode video: %w: %s", encodeErr, strings.TrimSpace(encoderErrors.String()))
	}
	if runErr != nil {
		return runErr
	}
	if recorder.err != nil {
		return recorder.err
	}
	if recorder.frames != frames {
		return fmt.Errorf("recording stopped after %d of %d frames", recorder.frames, frames)
	}
	proof := replay.ExpertMetadata()
	if frames == frameCount(showcase.Duration()) && film.State().NativeSHA256 != proof.TerminalSHA256 {
		return fmt.Errorf("full presentation diverged from the verified native terminal checksum")
	}
	if closeErr != nil {
		return closeErr
	}
	if err := finishWAV(wav, recorder.audioSamples); err != nil {
		return err
	}
	if err := wav.Close(); err != nil {
		return err
	}
	stem := strings.TrimSuffix(c.output, filepath.Ext(c.output))
	subtitles := stem + ".srt"
	if err := writeSubtitles(subtitles); err != nil {
		return err
	}
	if err := mux(ffmpeg, silent, audio, c.output, frames); err != nil {
		return err
	}
	if len(recorder.chapters) != 0 {
		recorder.chapters[len(recorder.chapters)-1].EndState = film.State()
	}
	report := evidence{SourceSHA256: proof.SourceSHA256, ProgramSHA256: proof.ProgramSHA256, InputSHA256: proof.RecordingSHA256, EncodedFPS: videoFPS, NativeFieldsPerSec: 50, Width: videoWidth, Height: videoHeight, Frames: frames, AudioSampleRate: sampleRate, AudioChannels: 2, AudioSamples: recorder.audioSamples, DurationSeconds: float64(frames) / videoFPS, PlannedSeconds: showcase.Duration().Seconds(), EncodingSeconds: time.Since(started).Seconds(), CompletePlan: frames == frameCount(showcase.Duration()), Chapters: recorder.chapters, FinalState: film.State(), Scope: "Edited excerpts of native Go gameplay, original reconstructed artwork and native original PCM. Encoded 60 FPS describes the recording timeline, not a runtime performance benchmark or complete Amiga parity."}
	if err := writeJSON(stem+".json", report); err != nil {
		return err
	}
	fmt.Printf("presentation ready: %s; %.3f seconds, %d frames, %d stereo samples; subtitles: %s\n", c.output, report.DurationSeconds, frames, recorder.audioSamples, subtitles)
	return nil
}

func validateSource(output string) error {
	film, err := game.NewFilm()
	if err != nil {
		return err
	}
	var pcm [samplesPerField * bytesPerSample]byte
	started := time.Now()
	for field := 0; field < showcase.TotalFields(); field++ {
		chapter, local, _ := showcase.ChapterAt(field)
		if err := film.Step(chapter, local, epoch.Add(time.Duration(field)*time.Second/50)); err != nil {
			return err
		}
		if n, err := film.ReadAudio(pcm[:]); err != nil {
			return err
		} else if n != len(pcm) {
			return io.ErrUnexpectedEOF
		}
	}
	proof, state := replay.ExpertMetadata(), film.State()
	if state.NativeSHA256 != proof.TerminalSHA256 {
		return fmt.Errorf("presentation source terminal checksum mismatch: %s", state.NativeSHA256)
	}
	path := strings.TrimSuffix(output, filepath.Ext(output)) + ".source-validation.json"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	result := struct {
		SourceSHA256, InputSHA256, ExpectedTerminalSHA256 string
		Verified                                          bool
		PresentationFields                                int
		Terminal                                          game.FilmState
		ElapsedSeconds                                    float64
		Scope                                             string
	}{proof.SourceSHA256, proof.RecordingSHA256, proof.TerminalSHA256, true, showcase.TotalFields(), state, time.Since(started).Seconds(), "A separate, headless native replay of the presentation and all omitted inputs with original PCM. This verifies source progression and its terminal state; it does not inspect an encoded video's pixels or establish Amiga parity."}
	if err := writeJSON(path, result); err != nil {
		return err
	}
	if err := attachValidation(output, path, state, proof); err != nil {
		return err
	}
	fmt.Printf("native presentation verified: input=%d deaths=%d score=%d terminal=%s report=%s\n", state.InputPosition, state.Deaths, state.Score, state.NativeSHA256, path)
	return nil
}

func attachValidation(output, validationPath string, state game.FilmState, proof replay.ExpertProof) error {
	path := strings.TrimSuffix(output, filepath.Ext(output)) + ".json"
	encoded, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var recorded evidence
	if err := json.Unmarshal(encoded, &recorded); err != nil {
		return err
	}
	if !recorded.CompletePlan || recorded.SourceSHA256 != proof.SourceSHA256 || recorded.ProgramSHA256 != proof.ProgramSHA256 || recorded.InputSHA256 != proof.RecordingSHA256 {
		return fmt.Errorf("existing presentation metadata does not describe the selected complete source run")
	}
	// Captures without a digest can still supply independently compared scalars.
	// A recorded checksum, when present, must also match this separate replay.
	recordedState := recorded.FinalState
	if recordedState.NativeSHA256 != "" && recordedState.NativeSHA256 != state.NativeSHA256 {
		return fmt.Errorf("captured terminal checksum does not match the separate native validation")
	}
	digestObserved := recordedState.NativeSHA256 != ""
	recordedState.NativeSHA256 = state.NativeSHA256
	if recordedState != state {
		return fmt.Errorf("captured terminal scalars do not match the separate native validation")
	}
	var annotated map[string]any
	if err := json.Unmarshal(encoded, &annotated); err != nil {
		return err
	}
	annotated["native_source_validation"] = map[string]any{
		"report":                         validationPath,
		"terminal_sha256":                state.NativeSHA256,
		"capture_scalar_state_matches":   true,
		"digest_observed_during_capture": digestObserved,
		"scope":                          "Separate headless native replay; the captured scalar state matches. This replay does not inspect encoded pixels or measure runtime frame rate.",
	}
	return writeJSON(path, annotated)
}

func (c *capture) Update() error {
	if c.err != nil {
		return c.err
	}
	if c.frames == c.totalFrames {
		return ebiten.Termination
	}
	if !c.started {
		c.started = true
		return c.stepField(0)
	}
	// A PCM interval ending exactly on a PAL boundary leaves that next state
	// for this Update, before its coincident video frame is drawn.
	for int64(c.field+1)*samplesPerField <= c.audioSamples {
		c.field++
		if err := c.stepField(c.field); err != nil {
			return err
		}
	}
	return nil
}

func (c *capture) Draw(screen *ebiten.Image) {
	if c.err != nil || c.frames == c.totalFrames {
		return
	}
	if c.target == nil {
		c.target = ebiten.NewImage(videoWidth, videoHeight)
		c.pixels = make([]byte, videoWidth*videoHeight*4)
	}
	stamp := time.Duration(int64(c.frames) * int64(time.Second) / videoFPS)
	c.film.DrawFrame(c.target, epoch.Add(stamp), showcase.CaptionAt(stamp))
	c.target.ReadPixels(c.pixels)
	if n, err := c.video.Write(c.pixels); err != nil {
		c.err = fmt.Errorf("stream video frame %d: %w", c.frames, err)
		return
	} else if n != len(c.pixels) {
		c.err = fmt.Errorf("stream video frame %d: %w", c.frames, io.ErrShortWrite)
		return
	}
	screen.DrawImage(c.target, nil)
	c.frames++
	if err := c.fillAudio(int64(c.frames) * samplesPerFrame); err != nil {
		c.err = err
		return
	}
	if c.frames%(videoFPS*10) == 0 {
		fmt.Printf("captured %d/%d frames (%.1f video seconds)\n", c.frames, c.totalFrames, float64(c.frames)/videoFPS)
	}
}

func (c *capture) Layout(_, _ int) (int, int) { return videoWidth, videoHeight }

func (c *capture) stepField(field int) error {
	chapter, local, ok := showcase.ChapterAt(field)
	if !ok {
		// Ceil(duration*60) can add a fraction of a final PAL interval. Preserve
		// the last state and continue its original audio for that short tail.
		return nil
	}
	if local == 0 && len(c.chapters) != 0 {
		c.chapters[len(c.chapters)-1].EndState = c.film.State()
	}
	if err := c.film.Step(chapter, local, epoch.Add(time.Duration(field)*time.Second/50)); err != nil {
		return fmt.Errorf("chapter %q field %d: %w", chapter.Name, local, err)
	}
	if local == 0 {
		c.chapters = append(c.chapters, chapterEvidence{Name: chapter.Name, Page: chapter.Page, StartField: field, PlannedFields: chapter.DurationFields, SourceInputStart: chapter.InputStart, StartState: c.film.State()})
		fmt.Printf("chapter: %s\n", chapter.Name)
	}
	c.chapters[len(c.chapters)-1].CapturedFields++
	return nil
}

func (c *capture) fillAudio(target int64) error {
	for c.audioSamples < target {
		boundary := int64(c.field+1) * samplesPerField
		if c.audioSamples == boundary {
			c.field++
			if err := c.stepField(c.field); err != nil {
				return err
			}
			boundary += samplesPerField
		}
		end := min(target, boundary)
		buffer := c.pcm[:int(end-c.audioSamples)*bytesPerSample]
		n, err := c.film.ReadAudio(buffer)
		if err != nil {
			return fmt.Errorf("render original PCM at sample %d: %w", c.audioSamples, err)
		}
		if n != len(buffer) {
			return io.ErrUnexpectedEOF
		}
		if _, err := c.wav.Write(buffer); err != nil {
			return err
		}
		c.audioSamples = end
	}
	return nil
}

func frameCount(duration time.Duration) int {
	return int((int64(duration)*videoFPS + int64(time.Second) - 1) / int64(time.Second))
}

func finishWAV(file *os.File, samples int64) error {
	dataBytes := samples * bytesPerSample
	if dataBytes < 0 || dataBytes+36 > int64(^uint32(0)) {
		return fmt.Errorf("PCM exceeds the RIFF WAV size limit")
	}
	var header [44]byte
	copy(header[:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(dataBytes+36))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 2)
	binary.LittleEndian.PutUint32(header[24:], sampleRate)
	binary.LittleEndian.PutUint32(header[28:], sampleRate*bytesPerSample)
	binary.LittleEndian.PutUint16(header[32:], bytesPerSample)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(dataBytes))
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := file.Write(header[:])
	return err
}

func writeSubtitles(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	err = showcase.WriteSRT(file)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func mux(ffmpeg, video, audio, output string, frames int) error {
	temporary, err := os.CreateTemp(filepath.Dir(output), ".showcase-*.mp4")
	if err != nil {
		return err
	}
	path := temporary.Name()
	temporary.Close()
	defer os.Remove(path)
	duration := strconv.FormatFloat(float64(frames)/videoFPS, 'f', 9, 64)
	command := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-i", video, "-i", audio, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", "-t", duration, path)
	if encoded, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("mux original audio: %w: %s", err, strings.TrimSpace(string(encoded)))
	}
	return os.Rename(path, output)
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(value)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
