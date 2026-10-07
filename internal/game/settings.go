package game

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/battlesquadron/assets"
	"github.com/olivierh59500/battlesquadron/internal/engine"
)

type scoreEntry struct {
	Name  string
	Score int
}

type preferences struct {
	Music, Effects bool
	Players        int
	Options        engine.Options
	Scores         []scoreEntry
}

func originalScores() ([]scoreEntry, error) {
	data, err := fs.ReadFile(assets.Files, "unpacked/lodsco.bin")
	if err != nil {
		return nil, err
	}
	if len(data) != 240 {
		return nil, fmt.Errorf("the original twelve-entry score table is invalid")
	}
	var scores []scoreEntry
	for row := 0; row < 12; row++ {
		record := data[row*20 : (row+1)*20]
		score, err := strconv.Atoi(string(record[12:20]))
		if err != nil {
			return nil, err
		}
		scores = append(scores, scoreEntry{string(record[8:11]), score})
	}
	return scores, nil
}

// SetDataDir loads preferences while retaining the original score table as fallback.
func (g *Game) SetDataDir(directory string) {
	if directory == "" {
		if root, err := os.UserConfigDir(); err == nil {
			directory = filepath.Join(root, "battlesquadron")
		}
	}
	g.dataDir = directory
	if directory == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(directory, "settings.json"))
	if err != nil {
		return
	}
	var saved preferences
	if json.Unmarshal(data, &saved) != nil {
		return
	}
	if saved.Options.Lives >= 3 && saved.Options.Lives <= 5 && saved.Options.EnemyProjectileCap >= 1 && saved.Options.EnemyProjectileCap <= 12 && saved.Options.EnemyProjectileSpeed >= 1 && saved.Options.EnemyProjectileSpeed <= 5 && saved.Options.EnemyFireDelay >= 8 && saved.Options.EnemyFireDelay <= 120 && saved.Options.StartWeapon >= 0 && saved.Options.StartWeapon < 4 {
		saved.Options.Invulnerable = false
		g.Core.Options = saved.Options
	}
	g.musicEnabled, g.effectsEnabled = saved.Music, saved.Effects
	g.players = max(1, min(2, saved.Players))
	if len(saved.Scores) == 12 {
		valid := true
		for _, score := range saved.Scores {
			if score.Score < 0 || len(score.Name) != 3 {
				valid = false
			}
		}
		if valid {
			g.scores = saved.Scores
			slices.SortStableFunc(g.scores, func(a, b scoreEntry) int { return b.Score - a.Score })
		}
	}
}

// Atomic replacement preserves the last complete save if the application stops.
func (g *Game) saveSettings() {
	if g.dataDir == "" || g.SmokeFrames > 0 || g.DemoActive() {
		return
	}
	p := preferences{Music: g.musicEnabled, Effects: g.effectsEnabled, Players: g.players, Options: g.Core.Options, Scores: g.scores}
	p.Options.Invulnerable = false
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		g.storageErr = err
		return
	}
	if err = os.MkdirAll(g.dataDir, 0755); err != nil {
		g.storageErr = err
		return
	}
	file, err := os.CreateTemp(g.dataDir, "settings-*.tmp")
	if err != nil {
		g.storageErr = err
		return
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(append(data, '\n')); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, filepath.Join(g.dataDir, "settings.json"))
	}
	g.storageErr = err
}

func (g *Game) queueScores() {
	if g.DemoActive() {
		return
	}
	g.scorePlayers = nil
	for index, p := range g.Core.Players {
		if p.Active && len(g.scores) > 0 && p.Score > g.scores[len(g.scores)-1].Score {
			g.scorePlayers = append(g.scorePlayers, index)
		}
	}
	g.initials = "AAA"
	g.initialIndex = 0
	g.CancelInput()
}

func (g *Game) enterInitials() {
	if len(g.scorePlayers) == 0 {
		return
	}
	name := []byte(g.initials)
	for _, char := range ebiten.AppendInputChars(nil) {
		char = []rune(strings.ToUpper(string(char)))[0]
		if char >= 'A' && char <= 'Z' {
			name[g.initialIndex] = byte(char)
			g.initialIndex = min(2, g.initialIndex+1)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		g.initialIndex = (g.initialIndex + 2) % 3
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		g.initialIndex = (g.initialIndex + 1) % 3
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		name[g.initialIndex] = 'A' + (name[g.initialIndex]-'A'+1)%26
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		name[g.initialIndex] = 'A' + (name[g.initialIndex]-'A'+25)%26
	}
	g.initials = string(name)
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.finishInitials()
	}
}

func (g *Game) finishInitials() {
	if len(g.scorePlayers) == 0 {
		return
	}
	player := g.scorePlayers[0]
	g.scores = append(g.scores, scoreEntry{g.initials, g.Core.Players[player].Score})
	slices.SortStableFunc(g.scores, func(a, b scoreEntry) int { return b.Score - a.Score })
	g.scores = g.scores[:12]
	g.scorePlayers = g.scorePlayers[1:]
	g.initials = "AAA"
	g.initialIndex = 0
	g.saveSettings()
}

func (g *Game) highScore() int {
	high := 0
	if len(g.scores) > 0 {
		high = g.scores[0].Score
	}
	return max(high, max(g.Core.Players[0].Score, g.Core.Players[1].Score))
}

func (g *Game) drawScores(screen *ebiten.Image) {
	x := (g.windowWidth() - 320) / 2
	g.panel(screen, "HIGH SCORES", "")
	for index, score := range g.scores {
		g.art.text(screen, fmt.Sprintf("%2d. %s %08d", index+1, score.Name, score.Score), x+88, 50+index*12, white)
	}
}
