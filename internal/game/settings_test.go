package game

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

func TestOriginalScoresAndAtomicPreferences(t *testing.T) {
	scores, err := originalScores()
	if err != nil {
		t.Fatal(err)
	}
	if len(scores) != 12 || scores[0].Name != "MBP" || scores[0].Score != 1000000 {
		t.Fatalf("original scores: %+v", scores)
	}
	directory := t.TempDir()
	g := &Game{Core: &engine.Engine{Options: engine.DefaultOptions()}, dataDir: directory, scores: scores, players: 2, musicEnabled: true, effectsEnabled: false}
	g.Core.Options.StartWeapon = 2
	g.scores[0] = scoreEntry{"XYZ", 1234567}
	g.saveSettings()
	if g.storageErr != nil {
		t.Fatal(g.storageErr)
	}
	restored := &Game{Core: &engine.Engine{Options: engine.DefaultOptions()}, scores: scores}
	restored.SetDataDir(directory)
	if restored.players != 2 || !restored.musicEnabled || restored.effectsEnabled || restored.Core.Options.StartWeapon != 2 || restored.scores[0].Name != "XYZ" || restored.highScore() != 1234567 {
		t.Fatalf("preferences were not restored: %+v", restored)
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary saves: %v %v", files, err)
	}
}

func TestMalformedSaveRetainsOriginals(t *testing.T) {
	scores, err := originalScores()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte("incomplete"), 0644); err != nil {
		t.Fatal(err)
	}
	g := &Game{Core: &engine.Engine{Options: engine.DefaultOptions()}, scores: scores, players: 1}
	g.SetDataDir(directory)
	if g.highScore() != 1000000 || g.Core.Options.Lives != 3 {
		t.Fatal("damaged save replaced original defaults")
	}
}
