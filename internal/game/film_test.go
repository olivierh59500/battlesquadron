package game

import (
	"testing"
	"time"

	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/showcase"
)

func TestFilmMenusLeadThroughTheRealIdleDeadlineBeforeExpertInputs(t *testing.T) {
	film, err := NewFilm()
	if err != nil {
		t.Fatal(err)
	}
	field := 0
	for _, chapter := range showcase.Plan() {
		if chapter.Page == showcase.PageExpert {
			if !film.game.DemoActive() || film.State().InputPosition != 0 {
				t.Fatal("menu sequence skipped the real idle deadline or consumed gameplay early")
			}
			if err := film.Step(chapter, 0, time.Unix(0, 0).Add(time.Duration(field)*palField)); err != nil {
				t.Fatal(err)
			}
			state := film.State()
			if state.Frame != 1 || state.InputPosition != 1 || state.Lives != 3 || state.Nova != 3 || state.Deaths != 0 || film.game.Core.Options != engine.DefaultOptions() {
				t.Fatalf("presentation did not launch the ordinary verified expert: %+v", state)
			}
			return
		}
		for tick := 0; tick < chapter.DurationFields; tick++ {
			if err := film.Step(chapter, tick, time.Unix(0, 0).Add(time.Duration(field)*palField)); err != nil {
				t.Fatal(err)
			}
			field++
			if chapter.Page != showcase.PageIdle && film.game.DemoActive() {
				t.Fatal("scripted menu interaction unexpectedly started a demonstration")
			}
		}
	}
	t.Fatal("presentation has no expert footage")
}
