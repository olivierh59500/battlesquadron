package showcase

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestPlanCoversExactDurationAndVerifiedInputIntervals(t *testing.T) {
	plan := Plan()
	if len(plan) != 14 || TotalFields() != 14201 || Duration() != 284*time.Second+20*time.Millisecond {
		t.Fatalf("unexpected presentation length: chapters=%d fields=%d duration=%s", len(plan), TotalFields(), Duration())
	}
	field, previousInputEnd := 0, uint64(0)
	for _, chapter := range plan {
		if chapter.DurationFields <= 0 {
			t.Fatalf("empty chapter: %+v", chapter)
		}
		first, offset, ok := ChapterAt(field)
		if !ok || first != chapter || offset != 0 {
			t.Fatalf("chapter boundary at field %d: %+v, offset=%d, available=%t", field, first, offset, ok)
		}
		last, offset, ok := ChapterAt(field + chapter.DurationFields - 1)
		if !ok || last != chapter || offset != chapter.DurationFields-1 {
			t.Fatalf("chapter end at field %d did not remain in %s", field, chapter.Name)
		}
		if chapter.Page == PageExpert {
			end := chapter.InputStart + uint64(chapter.DurationFields)
			if chapter.InputStart < previousInputEnd || end > 68401 {
				t.Fatalf("expert interval rewinds or exceeds the verified recording: %+v", chapter)
			}
			previousInputEnd = end
		}
		field += chapter.DurationFields
	}
	if previousInputEnd != 68401 {
		t.Fatal("the final encounter omits the completed ending payout")
	}
	for _, invalid := range []int{-1, TotalFields()} {
		if _, _, ok := ChapterAt(invalid); ok {
			t.Fatalf("out-of-range field %d selected a chapter", invalid)
		}
	}
	// Sixty video frames per second cover the final fractional PAL interval.
	frames := (int64(Duration())*60 + int64(time.Second) - 1) / int64(time.Second)
	if frames != 17042 {
		t.Fatalf("unexpected rounded video frame count: %d", frames)
	}
	plan[0].Name = "modified"
	if Plan()[0].Name == "modified" {
		t.Fatal("an editor changed the default schedule through a returned slice")
	}
}

func TestSelectedChaptersContainTheirVerifiedHighlights(t *testing.T) {
	// These are observed input positions from the selected ordinary expert run.
	highlights := map[string][]uint64{
		"Weapon capsule":      {1581},
		"First cave entrance": {7393, 7455},
		"Weapon upgrades":     {10329, 10437, 10585},
		"First cave boss":     {12145, 13427},
		"Second cave boss":    {43503},
		"Third cave Nova":     {59623, 60245},
		"Final encounter":     {65697, 67023, 68301, 68400},
	}
	for _, chapter := range Plan() {
		for _, input := range highlights[chapter.Name] {
			if input < chapter.InputStart || input >= chapter.InputStart+uint64(chapter.DurationFields) {
				t.Fatalf("%s cuts out verified input %d", chapter.Name, input)
			}
		}
		delete(highlights, chapter.Name)
	}
	if len(highlights) != 0 {
		t.Fatalf("missing showcase chapters: %v", highlights)
	}
}

func TestCaptionsStayReadableAndWithinTheFilm(t *testing.T) {
	previousEnd := time.Duration(0)
	for _, caption := range Captions() {
		if caption.Start < previousEnd || caption.End <= caption.Start || caption.End > Duration() {
			t.Fatalf("caption is overlapping or outside the film: %+v", caption)
		}
		if len(caption.Lines) == 0 || len(caption.Lines) > 2 {
			t.Fatalf("caption needs one or two lines: %+v", caption)
		}
		for _, line := range caption.Lines {
			if len([]rune(line)) > 48 || strings.TrimSpace(line) == "" || strings.ContainsAny(line, "\r\n") {
				t.Fatalf("caption line does not fit the presentation: %q", line)
			}
		}
		if got := CaptionAt(caption.Start); strings.Join(got, "\n") != strings.Join(caption.Lines, "\n") {
			t.Fatalf("caption start does not select its text: %+v", caption)
		}
		previousEnd = caption.End
	}
	if CaptionAt(Duration()) != nil || CaptionAt(-time.Nanosecond) != nil {
		t.Fatal("subtitle text leaked outside the presentation interval")
	}
	copied := Captions()
	copied[0].Lines[0] = "modified"
	if Captions()[0].Lines[0] == "modified" {
		t.Fatal("an editor changed the original captions through a returned line slice")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSRTMatchesPresentationTimingAndReportsWriteErrors(t *testing.T) {
	var encoded bytes.Buffer
	if err := WriteSRT(&encoded); err != nil {
		t.Fatal(err)
	}
	text := encoded.String()
	if !strings.HasPrefix(text, "1\n00:00:00,000 --> 00:00:06,000\nBATTLE SQUADRON\n") ||
		!strings.Contains(text, "00:04:40,020 --> 00:04:44,020\nMission complete.\nZero deaths. Final score: 2,473,920.\n\n") {
		t.Fatalf("SRT lost its exact first or final cue: %s", text)
	}
	if got := strings.Count(text, " --> "); got != len(Captions()) {
		t.Fatalf("SRT cue count %d does not match the renderer", got)
	}
	if err := WriteSRT(failingWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("subtitle output error was lost: %v", err)
	}
}
