package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/battlesquadron/internal/controls"
	"github.com/olivierh59500/battlesquadron/internal/engine"
)

func held(keys ...ebiten.Key) bool {
	for _, key := range keys {
		if ebiten.IsKeyPressed(key) {
			return true
		}
	}
	return false
}

func axis(negative, positive bool) int {
	v := 0
	if negative {
		v--
	}
	if positive {
		v++
	}
	return v
}

func (g *Game) input() [2]engine.Input {
	out := [2]engine.Input{
		{X: axis(held(ebiten.KeyArrowLeft), held(ebiten.KeyArrowRight)), Y: axis(held(ebiten.KeyArrowUp), held(ebiten.KeyArrowDown)), Fire: held(ebiten.KeySpace, ebiten.KeyControlRight), Nova: held(ebiten.KeyX, ebiten.KeyShiftRight)},
		{X: axis(held(ebiten.KeyA), held(ebiten.KeyD)), Y: axis(held(ebiten.KeyW), held(ebiten.KeyS)), Fire: held(ebiten.KeyControlLeft, ebiten.KeyC), Nova: held(ebiten.KeyV, ebiten.KeyShiftLeft)},
	}
	g.padIDs = ebiten.AppendGamepadIDs(g.padIDs[:0])
	for index, id := range g.padIDs {
		if index > 1 {
			break
		}
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		x, y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal), ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		out[index].X += axis(x < -0.3 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft), x > 0.3 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight))
		out[index].Y += axis(y < -0.3 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop), y > 0.3 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom))
		out[index].Fire = out[index].Fire || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom)
		out[index].Nova = out[index].Nova || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightRight)
		if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonCenterRight) {
			g.togglePause()
		}
	}
	g.touchIDs = ebiten.AppendTouchIDs(g.touchIDs[:0])
	points := make(map[int]controls.Point, len(g.touchIDs))
	for _, id := range g.touchIDs {
		x, y := ebiten.TouchPosition(id)
		points[int(id)] = controls.Point{X: float64(x), Y: float64(y)}
	}
	if g.touchEnabled && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		points[-2] = controls.Point{X: float64(x), Y: float64(y)}
	}
	if g.pointerSuppressed {
		if len(points) == 0 {
			g.pointerSuppressed = false
		}
		g.previousPointers = points
		return out
	}
	for id, p := range points {
		if id == -2 {
			continue
		}
		if _, already := g.previousPointers[id]; !already {
			g.pointer(int(p.X), int(p.Y), &out)
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		g.pointer(x, y, &out)
	}
	if g.touchEnabled {
		state := g.touch.Update(points)
		out[0].X += int(state.X)
		out[0].Y += int(state.Y)
		out[0].Fire = out[0].Fire || state.Fire
		out[0].Nova = out[0].Nova || state.Nova
	} else if g.Core.Mode == engine.Playing && !g.paused {
		out[0].Fire = out[0].Fire || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		out[0].Nova = out[0].Nova || ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	}
	g.previousPointers = points
	return out
}

func (g *Game) pointer(x, y int, out *[2]engine.Input) {
	if len(g.scorePlayers) > 0 {
		center := g.windowWidth() / 2
		if y > 112 && y < 147 && x >= center-40 && x <= center+40 {
			index := max(0, min(2, (x-center+12)/8))
			name := []byte(g.initials)
			name[index] = 'A' + (name[index]-'A'+1)%26
			g.initials = string(name)
			g.initialIndex = index
		} else if y >= 156 && y <= 194 {
			g.finishInitials()
		}
		return
	}
	if g.showScores {
		g.showScores = false
		return
	}
	if g.touchEnabled && x > 400 && y < 82 {
		switch {
		case y < 40:
			g.togglePause()
		case y < 64:
			g.SetMuted(!g.mute)
		default:
			g.Back()
		}
		return
	}
	offset := (g.windowWidth() - 320) / 2
	if g.optionsOpen {
		row := (y - 82) / 20
		if y >= 82 && row >= 0 && row < 5 {
			g.selectedOption = row
			delta := 1
			if x < g.windowWidth()/2 {
				delta = -1
			}
			switch row {
			case 0:
				g.Core.Options.Lives = max(3, min(5, g.Core.Options.Lives+delta))
			case 1:
				g.Core.Options.EnemyProjectileCap = max(1, min(12, g.Core.Options.EnemyProjectileCap+delta))
			case 2:
				g.Core.Options.EnemyProjectileSpeed = max(1, min(5, g.Core.Options.EnemyProjectileSpeed+delta))
			case 3:
				g.Core.Options.EnemyFireDelay = max(8, min(120, g.Core.Options.EnemyFireDelay+4*delta))
			case 4:
				g.Core.Options.StartWeapon = max(0, min(3, g.Core.Options.StartWeapon+delta))
			}
		} else if y > 186 {
			g.optionsOpen = false
			g.saveSettings()
		}
		return
	}
	if g.Core.Mode == engine.Title {
		x -= offset
		if y > 100 && y < 157 {
			switch {
			case x >= 76 && x < 112:
				g.players = 1
			case x >= 112 && x < 152:
				g.players = 2
			case x >= 152 && x < 192:
				g.effectsEnabled = !g.effectsEnabled
				if g.sound != nil {
					g.sound.SetEffectsEnabled(g.effectsEnabled)
				}
			case x >= 192 && x < 232:
				g.musicEnabled = !g.musicEnabled
				if g.sound != nil {
					g.sound.SetMusicEnabled(g.musicEnabled)
				}
			}
			return
		}
		if x >= 76 && x < 116 && y >= 158 && y < 190 {
			g.optionsOpen = true
			return
		}
		if x >= 0 && x < 320 && y >= 24 && y <= 232 {
			out[0].Fire = true
		}
		return
	}
	if g.paused {
		g.togglePause()
		return
	}
	if g.Core.Mode == engine.GameOver || g.Core.Mode == engine.Ending {
		g.Back()
	}
}
