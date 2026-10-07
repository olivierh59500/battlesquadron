package game

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/olivierh59500/battlesquadron/assets"
	"github.com/olivierh59500/battlesquadron/internal/controls"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/original"
	"github.com/olivierh59500/battlesquadron/internal/replay"
)

func attractFixture(t *testing.T) *Game {
	t.Helper()
	data, err := original.Load(assets.Files)
	if err != nil {
		t.Fatal(err)
	}
	core, err := engine.New(data)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := replay.LoadExpert(assets.Files)
	if err != nil {
		t.Fatal(err)
	}
	scores, err := originalScores()
	if err != nil {
		t.Fatal(err)
	}
	return &Game{Core: core, attract: &attractState{data: data, cursor: cursor}, players: 2,
		scores: scores, musicEnabled: true, effectsEnabled: true, touch: controls.New(480, 256)}
}

func attractTick(t *testing.T, g *Game, activity playerActivity) bool {
	t.Helper()
	handled, err := g.updateAttract(activity)
	if err != nil {
		t.Fatal(err)
	}
	return handled
}

func enterAttract(t *testing.T, g *Game) {
	t.Helper()
	for range attractIdleTicks {
		attractTick(t, g, playerActivity{})
	}
	if !g.DemoActive() {
		t.Fatal("foreground title inactivity did not start the expert")
	}
}

func TestAttractDeadlineAndHumanMenuIsolation(t *testing.T) {
	g := attractFixture(t)
	menu := g.Core
	menu.Options.Lives, menu.Options.StartWeapon, menu.Options.EnemyProjectileSpeed = 5, 2, 5
	before := menu.Digest()
	options := menu.Options
	for range attractIdleTicks - 1 {
		if attractTick(t, g, playerActivity{}) {
			t.Fatal("demo started before the complete idle interval")
		}
	}
	attractTick(t, g, playerActivity{active: true})
	if g.MenuIdleTicks() != 0 {
		t.Fatal("menu activity did not reset the idle interval")
	}
	enterAttract(t, g)
	if g.Core == menu || g.Core.Options != engine.DefaultOptions() || g.Core.Players[1].Active || g.players != 2 {
		t.Fatal("expert reused or changed the human's selected engine/options/player count")
	}
	for range 300 {
		attractTick(t, g, playerActivity{})
	}
	if menu.Digest() != before {
		t.Fatal("expert inputs mutated the saved menu engine")
	}
	attractTick(t, g, playerActivity{active: true, held: true})
	if g.DemoActive() || g.Core != menu || g.Core.Options != options || !g.DemoInputBlocked() {
		t.Fatal("waking input failed to restore the original menu")
	}
	for range 30 {
		if !attractTick(t, g, playerActivity{active: true, held: true}) || !g.DemoInputBlocked() {
			t.Fatal("held waking input escaped quarantine")
		}
	}
	if !attractTick(t, g, playerActivity{}) || g.DemoInputBlocked() || g.pointerSuppressed {
		t.Fatal("release must unblock the first fresh pointer immediately")
	}
	if attractTick(t, g, playerActivity{active: true, held: true}) {
		t.Fatal("a fresh post-release action was swallowed")
	}
	input := [2]engine.Input{}
	g.pointer(160, 200, &input)
	if !input[0].Fire || g.Core.Mode != engine.Title {
		t.Fatal("fresh menu tap could not request an ordinary human game")
	}
	g.start()
	if g.Core != menu || g.Core.Options != options || !g.Core.Players[1].Active || g.DemoActive() {
		t.Fatal("human game did not use restored player count and difficulty")
	}
}

func TestAttractDoesNotInterruptOtherScreensOrManualPlay(t *testing.T) {
	for _, screen := range []string{"options", "scores", "initials", "playing", "paused", "smoke"} {
		t.Run(screen, func(t *testing.T) {
			g := attractFixture(t)
			switch screen {
			case "options":
				g.optionsOpen = true
			case "scores":
				g.showScores = true
			case "initials":
				g.scorePlayers = []int{0}
			case "playing", "paused":
				g.Core.Start(1)
				g.paused = screen == "paused"
			case "smoke":
				g.SmokeFrames = 5000
			}
			for range attractIdleTicks + 1 {
				if attractTick(t, g, playerActivity{}) {
					t.Fatal("attract mode interrupted another screen/session")
				}
			}
			if g.MenuIdleTicks() != 0 || g.DemoActive() {
				t.Fatal("ineligible screens retained a title idle countdown")
			}
		})
	}
}

func TestAttractCompleteCampaignAndLoopNeverSaveScores(t *testing.T) {
	g := attractFixture(t)
	g.dataDir = t.TempDir()
	g.saveSettings()
	path := filepath.Join(g.dataDir, "settings.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	menu, scores := g.Core, append([]scoreEntry(nil), g.scores...)
	enterAttract(t, g)
	initial := g.Core.Digest()
	proof := replay.ExpertMetadata()
	for range proof.Fields {
		attractTick(t, g, playerActivity{})
	}
	digest := g.Core.Digest()
	if hex.EncodeToString(digest[:]) != proof.TerminalSHA256 || g.Core.Mode != engine.Ending {
		t.Fatal("attract inputs did not reach the verified expert terminal state")
	}
	g.queueScores()
	g.saveSettings()
	if len(g.scorePlayers) != 0 || !reflect.DeepEqual(scores, g.scores) {
		t.Fatal("demo submitted a high score")
	}
	for range attractEndingTicks {
		attractTick(t, g, playerActivity{})
	}
	if !g.DemoActive() || g.Core.Frame != 0 || g.Core.Digest() != initial || g.attract.cursor.Position() != 0 {
		t.Fatal("demo loop retained camera, campaign or input state from its ending")
	}
	g.StopDemo()
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || g.Core != menu {
		t.Fatal("demo completion/loop changed persistent settings or the human menu")
	}
}

func TestAttractBackAndSuspensionResetTheFullIdleInterval(t *testing.T) {
	g := attractFixture(t)
	enterAttract(t, g)
	g.Back()
	if g.DemoActive() || !g.AtTitle() || g.MenuIdleTicks() != 0 {
		t.Fatal("Back did not dismiss the demo to its menu")
	}
	attractTick(t, g, playerActivity{})
	enterAttract(t, g)
	g.Pause()
	if g.DemoActive() || !g.AtTitle() || g.paused || g.MenuIdleTicks() != 0 {
		t.Fatal("suspension left a paused demo or retained the old idle deadline")
	}
	attractTick(t, g, playerActivity{})
	for range attractIdleTicks - 1 {
		attractTick(t, g, playerActivity{})
	}
	if g.DemoActive() {
		t.Fatal("resume started the demo before a full new idle interval")
	}
	attractTick(t, g, playerActivity{})
	if !g.DemoActive() {
		t.Fatal("demo did not return after the full new foreground idle interval")
	}
}
