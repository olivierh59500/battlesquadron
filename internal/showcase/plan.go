// Package showcase describes a presentation made from the native game's menus
// and selected intervals of its verified expert input recording.
package showcase

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// FieldsPerSecond is the original PAL simulation cadence, independent of video FPS.
const FieldsPerSecond = 50

// Page identifies the menu or gameplay view presented by a chapter.
type Page string

const (
	PageTitle   Page = "title"
	PagePlayers Page = "players"
	PageOptions Page = "options"
	PageScores  Page = "scores"
	PageIdle    Page = "idle"
	PageExpert  Page = "expert"
	PageOutro   Page = "outro"
)

// Chapter specifies a contiguous interval in the edited presentation.
// InputStart is the number of expert inputs consumed before its first field.
// An expert chapter consumes [InputStart, InputStart+DurationFields).
type Chapter struct {
	Name           string
	DurationFields int
	Page           Page
	InputStart     uint64
}

// Duration expresses the chapter's integer PAL fields as elapsed video time.
func (c Chapter) Duration() time.Duration {
	return time.Duration(c.DurationFields) * time.Second / FieldsPerSecond
}

var chapters = [...]Chapter{
	{Name: "Title", DurationFields: 400, Page: PageTitle},
	{Name: "Player selection", DurationFields: 500, Page: PagePlayers},
	{Name: "Game options", DurationFields: 600, Page: PageOptions},
	{Name: "High scores", DurationFields: 400, Page: PageScores},
	{Name: "Idle demonstration", DurationFields: 750, Page: PageIdle},
	{Name: "Launch", DurationFields: 1150, Page: PageExpert, InputStart: 0},
	{Name: "Weapon capsule", DurationFields: 1200, Page: PageExpert, InputStart: 1450},
	{Name: "First cave entrance", DurationFields: 1050, Page: PageExpert, InputStart: 7000},
	{Name: "Weapon upgrades", DurationFields: 1150, Page: PageExpert, InputStart: 10100},
	{Name: "First cave boss", DurationFields: 1500, Page: PageExpert, InputStart: 12050},
	{Name: "Second cave boss", DurationFields: 1300, Page: PageExpert, InputStart: 42250},
	{Name: "Third cave Nova", DurationFields: 1150, Page: PageExpert, InputStart: 59250},
	{Name: "Final encounter", DurationFields: 2851, Page: PageExpert, InputStart: 65550},
	{Name: "Mission complete", DurationFields: 200, Page: PageOutro, InputStart: 68401},
}

// Plan returns an independent copy of the complete presentation schedule.
func Plan() []Chapter { return append([]Chapter(nil), chapters[:]...) }

// TotalFields returns the presentation's exact length in PAL fields.
func TotalFields() int {
	total := 0
	for _, chapter := range chapters {
		total += chapter.DurationFields
	}
	return total
}

// Duration returns the presentation length before final video-frame rounding.
func Duration() time.Duration {
	return time.Duration(TotalFields()) * time.Second / FieldsPerSecond
}

// ChapterAt selects a chapter and its local field at a global PAL field index.
// The valid interval is [0, TotalFields()); out-of-range fields return false.
func ChapterAt(field int) (Chapter, int, bool) {
	if field < 0 {
		return Chapter{}, 0, false
	}
	for _, chapter := range chapters {
		if field < chapter.DurationFields {
			return chapter, field, true
		}
		field -= chapter.DurationFields
	}
	return Chapter{}, 0, false
}

// Caption is a short English subtitle cue in the edited video's time domain.
// End is exclusive, so adjacent cues cannot appear simultaneously.
type Caption struct {
	Start, End time.Duration
	Lines      []string
}

func seconds(value int) time.Duration { return time.Duration(value) * time.Second }

var captions = [...]Caption{
	{seconds(0), seconds(6), []string{"BATTLE SQUADRON", "A native Go and Ebitengine remake."}},
	{seconds(8), seconds(15), []string{"Fly solo or team up with a second pilot."}},
	{seconds(18), seconds(24), []string{"Choose your lives and starting weapon."}},
	{seconds(25), seconds(30), []string{"Adjust enemy fire to set the challenge."}},
	{seconds(31), seconds(37), []string{"Your best runs stay on the local score table."}},
	{seconds(39), seconds(46), []string{"Leave the menu idle", "and the expert takes over."}},
	{seconds(47), seconds(53), []string{"The demonstration starts after 15 seconds."}},
	{seconds(54), seconds(61), []string{"Three ships. Three Novas.", "Build your firepower from the basic weapon."}},
	{seconds(65), seconds(73), []string{"Highlights from one complete,", "death-free run."}},
	{seconds(77), seconds(84), []string{"Catch weapon capsules", "to upgrade your firepower."}},
	{seconds(87), seconds(95), []string{"Attack the buildings while", "keeping an escape route."}},
	{seconds(102), seconds(112), []string{"A surface gate leads", "into the first cave."}},
	{seconds(123), seconds(130), []string{"Keep collecting. Each upgrade", "changes your firepower."}},
	{seconds(146), seconds(153), []string{"The first cave brings a larger opponent."}},
	{seconds(157), seconds(166), []string{"Stay mobile and attack", "its vulnerable sections."}},
	{seconds(175), seconds(182), []string{"Later in the run: the second cave."}},
	{seconds(184), seconds(192), []string{"A fully upgraded weapon", "meets a multipart boss."}},
	{seconds(194), seconds(200), []string{"Defeat it to return to the surface."}},
	{seconds(201), seconds(207), []string{"The third cave demands room to maneuver."}},
	{seconds(208), seconds(215), []string{"A Nova clears hostile fire", "and unleashes its expanding attack."}},
	{seconds(217), seconds(223), []string{"Replenish the reserve", "before the final battle."}},
	{seconds(224), seconds(231), []string{"All three caves are cleared.", "The final encounter begins."}},
	{seconds(236), seconds(243), []string{"Work through its separate sections."}},
	{seconds(250), seconds(257), []string{"Spend a Nova when", "the pressure closes in."}},
	{seconds(274), seconds(280), []string{"The final opponent falls."}},
	{seconds(280) + 20*time.Millisecond, seconds(284) + 20*time.Millisecond, []string{"Mission complete.", "Zero deaths. Final score: 2,473,920."}},
}

// Captions returns independent cues and line slices for editors and encoders.
func Captions() []Caption {
	result := append([]Caption(nil), captions[:]...)
	for index := range result {
		result[index].Lines = append([]string(nil), result[index].Lines...)
	}
	return result
}

// CaptionAt returns a copy of the subtitle lines visible at a video timestamp.
func CaptionAt(stamp time.Duration) []string {
	for _, caption := range captions {
		if stamp >= caption.Start && stamp < caption.End {
			return append([]string(nil), caption.Lines...)
		}
	}
	return nil
}

// WriteSRT writes the same captions used by the renderer as a standard sidecar.
func WriteSRT(writer io.Writer) error {
	for index, caption := range captions {
		if _, err := fmt.Fprintf(writer, "%d\n%s --> %s\n%s\n\n", index+1,
			srtTime(caption.Start), srtTime(caption.End), strings.Join(caption.Lines, "\n")); err != nil {
			return err
		}
	}
	return nil
}

func srtTime(stamp time.Duration) string {
	milliseconds := stamp.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d,%03d", milliseconds/3600000,
		milliseconds/60000%60, milliseconds/1000%60, milliseconds%1000)
}
