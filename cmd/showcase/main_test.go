package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/internal/game"
	"github.com/olivierh59500/battlesquadron/internal/showcase"
)

type sampleFilm struct {
	fields []int
	times  []time.Time
	field  int
}

func (f *sampleFilm) Step(_ showcase.Chapter, tick int, stamp time.Time) error {
	f.field = tick
	f.fields = append(f.fields, tick)
	f.times = append(f.times, stamp)
	return nil
}
func (f *sampleFilm) DrawFrame(*ebiten.Image, time.Time, []string) {}
func (f *sampleFilm) State() game.FilmState                        { return game.FilmState{} }
func (f *sampleFilm) ReadAudio(out []byte) (int, error) {
	for index := 0; index < len(out); index += bytesPerSample {
		binary.LittleEndian.PutUint16(out[index:], uint16(f.field))
		binary.LittleEndian.PutUint16(out[index+2:], uint16(f.field))
	}
	return len(out), nil
}

func TestAudioEventsStayOnPALBoundariesAcrossVideoFrames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clock.wav")
	wav, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer wav.Close()
	if _, err := wav.Write(make([]byte, 44)); err != nil {
		t.Fatal(err)
	}
	film := &sampleFilm{}
	c := &capture{film: film, wav: wav, totalFrames: 7}
	for frame := 0; frame < 6; frame++ {
		if err := c.Update(); err != nil {
			t.Fatal(err)
		}
		if err := c.fillAudio(int64(frame+1) * samplesPerFrame); err != nil {
			t.Fatal(err)
		}
	}
	if len(film.fields) != 5 || c.audioSamples != 4410 {
		t.Fatalf("six video intervals must contain five PAL audio intervals: fields=%v samples=%d", film.fields, c.audioSamples)
	}
	if err := finishWAV(wav, c.audioSamples); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded[:4]) != "RIFF" || string(encoded[8:16]) != "WAVEfmt " || binary.LittleEndian.Uint32(encoded[24:]) != 44100 || binary.LittleEndian.Uint16(encoded[22:]) != 2 || binary.LittleEndian.Uint32(encoded[40:]) != 4410*4 {
		t.Fatal("WAV header does not describe the exact stereo PCM stream")
	}
	for sample := 0; sample < 4410; sample++ {
		want := uint16(sample / 882)
		if left, right := binary.LittleEndian.Uint16(encoded[44+sample*4:]), binary.LittleEndian.Uint16(encoded[46+sample*4:]); left != want || right != want {
			t.Fatalf("sample %d: channels %d/%d, expected PAL event %d", sample, left, right, want)
		}
	}
	if err := c.Update(); err != nil {
		t.Fatal(err)
	}
	if len(film.fields) != 6 || film.fields[5] != 5 || film.times[5] != epoch.Add(100*time.Millisecond) {
		t.Fatal("the coincident 100ms PAL/video boundary was not applied before the next frame")
	}
}

func TestCompletePlanRoundsVideoUpWithoutLosingItsLastField(t *testing.T) {
	if got := frameCount(showcase.Duration()); got != 17042 {
		t.Fatalf("full presentation needs 17042 video frames, got %d", got)
	}
	if got := frameCount(12 * time.Second); got != 720 {
		t.Fatalf("12-second preview needs 720 video frames, got %d", got)
	}
}
