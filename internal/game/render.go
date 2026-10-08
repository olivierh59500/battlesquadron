package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/battlesquadron/internal/engine"
)

// Draw composites original decoded artwork; gameplay is never advanced here.
func (g *Game) Draw(screen *ebiten.Image) {
	g.drawAt(screen, time.Now())
}

func (g *Game) drawAt(screen *ebiten.Image, now time.Time) {
	g.preparePresentation(now)
	screen.Fill(color.Black)
	x := (g.windowWidth() - 320) / 2
	if g.Core.Mode == engine.Title {
		if g.art.title != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), 24)
			screen.DrawImage(g.art.title, op)
		}
		g.art.text(screen, fmt.Sprintf("%d PLAYER%s", g.players, map[bool]string{true: "S", false: ""}[g.players == 2]), x+112, 236, gold)
	} else if g.Core.Mode == engine.Ending {
		if staff, ok := g.art.sprites["scene_lodtem"]; ok {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(0.5, 1)
			op.GeoM.Translate(float64(x), 24)
			screen.DrawImage(staff.image, op)
		}
		if !g.DemoActive() {
			g.art.text(screen, "PRESS FIRE TO CONTINUE", x+72, 239, white)
		}
	} else {
		g.drawPlayfield()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(1/float64(g.art.density), 1/float64(g.art.density))
		op.GeoM.Translate(float64(x+16), 24)
		screen.DrawImage(g.view, op)
		g.art.density = 1
		if g.Core.Mode == engine.GameOver {
			g.panel(screen, "GAME OVER", "PRESS FIRE TO CONTINUE")
		}
		if g.Core.Mode == engine.Ending {
			g.panel(screen, "MISSION COMPLETE", "PRESS FIRE TO CONTINUE")
		}
	}
	if g.paused {
		g.panel(screen, "PAUSED", "P OR TAP TO RESUME")
	}
	if g.optionsOpen {
		g.drawOptions(screen)
	}
	if g.showScores {
		g.drawScores(screen)
	}
	if len(g.scorePlayers) > 0 {
		g.panel(screen, fmt.Sprintf("PLAYER %d - HIGH SCORE", g.scorePlayers[0]+1), g.initials)
		g.art.text(screen, "TYPE OR TAP INITIALS", g.windowWidth()/2-80, 157, white)
		g.art.text(screen, "ENTER OR TAP TO SAVE", g.windowWidth()/2-80, 177, gold)
	}
	if g.DemoActive() {
		g.art.text(screen, "DEMO - PRESS ANY CONTROL", g.windowWidth()/2-92, 239, gold)
	}
	if g.touchEnabled {
		g.drawTouch(screen)
	}
	g.saveCapture(screen)
}

func (g *Game) drawPlayfield() {
	density := 1
	if g.presentation.smooth {
		density = g.presentation.density
	}
	if g.view.Bounds().Dx() != 288*density {
		g.view.Dispose()
		g.view = ebiten.NewImage(288*density, 208*density)
	}
	g.art.density = density
	g.view.Fill(color.Black)
	if g.Core.FinalBattle() {
		if backdrop, ok := g.art.sprites["final_backdrop"]; ok {
			start := max(0, 240-g.presentation.scroll)
			end := min(240, start+208)
			if end > start {
				x := max(0, min(96, g.presentation.camera))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(float64(density), float64(density))
				op.GeoM.Translate(-x*float64(density), -start*float64(density))
				g.view.DrawImage(backdrop.image, op)
			}
		}
	} else {
		g.art.drawTerrainAt(g.view, g.Core.Stage, g.presentation.scroll, g.presentation.camera)
	}
	if g.Core.Stage == 0 {
		if gate, active := g.Core.Campaign.GateAtScroll(g.Core.Scroll); active {
			frame := 1
			if g.Core.Frame&31 < 8 {
				frame = 0
			}
			id := g.art.objects[objectKey{0, gate.Definition.Address, frame}]
			g.art.drawSpriteAt(g.view, id, float64(gate.WorldX)-g.presentation.camera, float64(gate.NativeY(g.Core.Scroll))+g.presentation.scroll-float64(g.Core.Scroll), 0)
		}
	}
	for _, enemy := range g.Core.Enemies {
		if !g.Core.GroundVisible(enemy) {
			continue
		}
		x, y := g.presentation.enemy(enemy)
		if enemy.Definition.FlyingPool && enemy.Definition.NativeKind == 7 {
			if enemy.Frame >= 3 {
				g.art.drawSpriteAt(g.view, g.art.flying[flyingKey{g.Core.Stage, 10, enemy.Frame - 3}], x, y, 0)
			} else {
				g.art.drawGrowingSpriteAt(g.view, g.art.flying[flyingKey{g.Core.Stage, 7, enemy.Frame}], x, y, 0, enemy.Definition.Height)
			}
			continue
		}
		if enemy.Definition.NativeKind == 2 && g.Core.FinalBattle() || enemy.Definition.NativeKind == 9 {
			g.art.drawIndexedSpriteAt(g.view, enemy.Definition.Sprite, x, y, enemy.Frame)
			continue
		}
		id := g.art.objects[objectKey{g.Core.Stage, enemy.Definition.Address, enemy.Frame}]
		if id == "" {
			id = g.art.objects[objectKey{g.Core.Stage, enemy.Definition.Address, 0}]
		}
		if id == "" {
			id = enemy.Definition.Sprite
		}
		if id == "" {
			id = fmt.Sprintf("object_%d", enemy.Definition.Graphic)
		}
		g.art.drawSpriteAt(g.view, id, x, y, 0)
	}
	for _, shot := range g.Core.EnemyShots {
		g.drawProjectile(shot, false)
	}
	for _, shot := range g.Core.PlayerShots {
		if shot.Delay == 0 {
			g.drawProjectile(shot, true)
		}
	}
	for _, pickup := range g.Core.Pickups {
		id := g.art.flying[flyingKey{g.Core.Stage, 5, pickup.Frame}]
		x, y := g.presentation.pickup(pickup, g.Core.Frame)
		g.art.drawSpriteAt(g.view, id, x, y, 0)
	}
	for _, explosion := range g.Core.Explosions {
		frame := min(8, explosion.Age/4)
		if explosion.Native {
			frame = explosion.Frame
		}
		kind := 10
		if explosion.Native {
			kind = explosion.SpriteKind
		}
		id := g.art.flying[flyingKey{g.Core.Stage, kind, frame}]
		if explosion.Player {
			id = g.art.explosions[min(9, explosion.Age/7)]
		}
		x, y := g.presentation.explosion(explosion, g.Core.Frame)
		g.art.drawSpriteAt(g.view, id, x, y, 0)
	}
	for index, player := range g.Core.Players {
		if !player.Active || player.Lives == 0 || player.Dying > 0 {
			continue
		}
		if player.Invulnerable > 0 && g.Core.Frame%4 < 2 {
			continue
		}
		x, y := g.presentation.player(index, player)
		g.art.drawSpriteAt(g.view, g.art.players[index][min(6, max(0, player.Tilt))], x, y, 0)
	}
	for index, ray := range g.Core.NovaRays {
		x, y := g.presentation.ray(index, ray)
		g.art.drawSpriteAt(g.view, g.art.bullets[int(ray.Graphic)], x, y, 0)
	}
	if g.Core.Players[0].Respawn >= 45 && g.Core.Frame < 130 {
		g.art.text(g.view, "GET READY", 108, 103, white)
	}
	g.art.text(g.view, "1UP", 22, 1, color.RGBA{136, 187, 255, 255})
	g.art.text(g.view, "HIGH", 125, 1, gold)
	g.art.text(g.view, "2UP", 234, 1, gold)
	g.art.text(g.view, scoreText(g.Core.Players[0].Score), 9, 12, white)
	g.art.text(g.view, scoreText(g.highScore()), 109, 12, white)
	g.art.text(g.view, scoreText(g.Core.Players[1].Score), 209, 12, white)
	for index, player := range g.Core.Players {
		x := 0
		if index == 1 {
			x = 240
		}
		for charge := 0; charge < min(8, player.Nova); charge++ {
			g.art.drawSprite(g.view, g.art.hudNova[index], x+charge*16, 194, 0)
		}
	}
}

func (g *Game) drawProjectile(shot engine.Bullet, primary bool) {
	id := g.art.bullets[int(shot.Graphic)]
	x, y := g.presentation.shot(shot, primary, g.Core.Frame)
	g.art.drawSpriteAt(g.view, id, x, y, 0)
}

func (g *Game) panel(screen *ebiten.Image, title, subtitle string) {
	x := (g.windowWidth() - 320) / 2
	vector.DrawFilledRect(screen, float32(x+8), 90, 304, 62, color.RGBA{0, 0, 0, 225}, false)
	g.art.text(screen, title, x+160-len(title)*4, 103, gold)
	g.art.text(screen, subtitle, x+160-len(subtitle)*4, 126, white)
}

func (g *Game) drawOptions(screen *ebiten.Image) {
	x := (g.windowWidth() - 320) / 2
	vector.DrawFilledRect(screen, float32(x+8), 45, 304, 176, color.RGBA{0, 0, 0, 255}, false)
	g.art.text(screen, "GAME OPTIONS", x+112, 55, gold)
	o := g.Core.Options
	rows := []string{fmt.Sprintf("LIVES                  %d", o.Lives), fmt.Sprintf("ENEMY BULLETS         %2d", o.EnemyProjectileCap), fmt.Sprintf("BULLET SPEED           %d", o.EnemyProjectileSpeed), fmt.Sprintf("FIRE DELAY            %2d", o.EnemyFireDelay), fmt.Sprintf("INITIAL WEAPON         %d", o.StartWeapon+1)}
	for row, label := range rows {
		ink := color.Color(white)
		if row == g.selectedOption {
			ink = gold
			g.art.text(screen, ">", x+22, 82+row*20, ink)
		}
		g.art.text(screen, label, x+38, 82+row*20, ink)
	}
	g.art.text(screen, "ARROWS: CHANGE  ENTER: SAVE", x+32, 191, white)
}

func (g *Game) drawTouch(screen *ebiten.Image) {
	if g.art.touchButtons == nil {
		g.cacheTouchArtwork()
	}
	screen.DrawImage(g.art.touchButtons, nil)
	center, stick, active := g.touch.Stick()
	cx, cy := float64(40), float64(181)
	if active {
		cx, cy = center.X, center.Y
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(cx-32, cy-32)
	screen.DrawImage(g.art.touchStick, op)
	if active {
		dx, dy := max(-25, min(25, stick.X-center.X)), max(-25, min(25, stick.Y-center.Y))
		op.GeoM.Reset()
		op.GeoM.Translate(cx+dx-9, cy+dy-9)
		screen.DrawImage(g.art.touchKnob, op)
	}
}

// Invariant touch geometry is drawn once at the original logical resolution.
// This only caches existing UI pixels; it adds no game resource or artwork.
func (g *Game) cacheTouchArtwork() {
	g.art.touchButtons = ebiten.NewImage(480, 256)
	g.art.touchStick = ebiten.NewImage(64, 64)
	g.art.touchKnob = ebiten.NewImage(18, 18)
	ink := color.RGBA{119, 136, 170, 255}
	for _, button := range []struct {
		label   string
		x, y, r float32
	}{{"PAUSE", 440, 23, 16}, {"SOUND", 440, 51, 12}, {"MENU", 440, 74, 9}, {"NOVA", 440, 131, 25}, {"FIRE", 440, 213, 30}} {
		vector.StrokeCircle(g.art.touchButtons, button.x, button.y, button.r, 1.5, ink, true)
		g.art.text(g.art.touchButtons, button.label, int(button.x)-len(button.label)*4, int(button.y)-5, white)
	}
	vector.StrokeCircle(g.art.touchStick, 32, 32, 28, 1.5, ink, true)
	vector.StrokeLine(g.art.touchStick, 20, 32, 44, 32, 1, ink, true)
	vector.StrokeLine(g.art.touchStick, 32, 20, 32, 44, 1, ink, true)
	vector.DrawFilledCircle(g.art.touchKnob, 9, 9, 8, ink, true)
	g.art.text(g.art.touchButtons, "MOVE", 24, 224, white)
}
