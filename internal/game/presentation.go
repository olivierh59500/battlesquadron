package game

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/internal/engine"
)

const palField = time.Second / 50

type visualSnapshot struct {
	frame, stage, scroll, camera int
	final                        bool
	players                      [2]engine.Player
	enemies                      []engine.Enemy
	playerShots, enemyShots      []engine.Bullet
	pickups                      []engine.Pickup
	explosions                   []engine.Explosion
	rays                         []engine.NovaRay
}

// Presentation retains six original PAL fields, independently of simulation.
// A common two-field display delay allows both 50 Hz and 25 Hz state to move
// continuously without predicting collisions or adding game updates.
type presentation struct {
	smooth             bool
	history            [6]visualSnapshot
	latest, count      int
	tickTime           time.Time
	drawFrame          float64
	density            int
	scroll, camera     float64
	lastPhysicalScroll int
	scrollChanges      int
	frames             int
}

func newPresentation() *presentation { return &presentation{smooth: true, density: 4} }

// SetSmoothRendering selects temporal interpolation of original artwork.
// Native movement, input, collision, score and audio timing remain unchanged.
func (g *Game) SetSmoothRendering(enabled bool) {
	g.presentation.smooth = enabled
	g.presentation.count = 0
}

// RenderingMotion returns viewport-raster terrain movement changes since startup.
// This distinguishes a fast display loop from repeated unchanged PAL images.
func (g *Game) RenderingMotion() (frames, changes, density int) {
	p := g.presentation
	density = p.density
	if !p.smooth {
		density = 1
	}
	return p.frames, p.scrollChanges, density
}

func (g *Game) observePresentation() {
	g.observePresentationAt(time.Now())
}

func (g *Game) observePresentationAt(now time.Time) {
	p, c := g.presentation, g.Core
	if p == nil {
		return
	}
	consecutive := false
	if p.count > 0 {
		last := &p.history[p.latest]
		if c.Frame == last.frame && c.Stage == last.stage && c.FinalBattle() == last.final {
			return
		}
		if c.Frame != last.frame+1 || c.Stage != last.stage || c.FinalBattle() != last.final || abs(c.Scroll-last.scroll) > 8 {
			p.count = 0
		} else {
			consecutive = true
		}
	}
	p.latest = (p.latest + 1) % len(p.history)
	s := &p.history[p.latest]
	s.frame, s.stage, s.scroll, s.camera, s.final = c.Frame, c.Stage, c.Scroll, c.CameraX, c.FinalBattle()
	s.players = c.Players
	s.enemies = append(s.enemies[:0], c.Enemies...)
	s.playerShots = append(s.playerShots[:0], c.PlayerShots...)
	s.enemyShots = append(s.enemyShots[:0], c.EnemyShots...)
	s.pickups = append(s.pickups[:0], c.Pickups...)
	s.explosions = append(s.explosions[:0], c.Explosions...)
	s.rays = append(s.rays[:0], c.NovaRays...)
	p.count = min(len(p.history), p.count+1)
	if consecutive && absDuration(now.Sub(p.tickTime)-palField) < 2*palField {
		// Keep the presentation clock periodic instead of inheriting the VSync
		// quantization of when Ebitengine dispatches each 50 Hz Update.
		p.tickTime = p.tickTime.Add(palField)
	} else {
		p.tickTime = now
	}
}

func (g *Game) preparePresentation(now time.Time) {
	p, c := g.presentation, g.Core
	g.observePresentationAt(now)
	p.drawFrame = float64(c.Frame)
	p.scroll, p.camera = float64(c.Scroll), float64(c.CameraX)
	if p.smooth && p.count == len(p.history) && !g.paused && c.Mode == engine.Playing {
		// Ebitengine dispatches Update on a display frame, which can precede or
		// follow the periodic PAL clock. Keeping that signed phase prevents
		// repeated pixels caused by clamping every early update to zero.
		alpha := min(2.0, max(-1.0, float64(now.Sub(p.tickTime))/float64(palField)))
		p.drawFrame = float64(c.Frame-2) + alpha
		p.drawFrame = min(p.drawFrame, float64(c.Frame-(1-c.Frame&1)))
		before, after, fraction := p.samples(2)
		if before != nil && after != nil {
			p.scroll = mix(before.scroll, after.scroll, fraction)
			p.camera = mix(before.camera, after.camera, fraction)
		}
	}
	physical := int(math.Round(p.scroll * float64(p.density)))
	if physical != p.lastPhysicalScroll {
		p.scrollChanges++
		p.lastPhysicalScroll = physical
	}
	p.frames++
}

func (p *presentation) samples(cadence int) (before, after *visualSnapshot, fraction float64) {
	if !p.smooth || p.count < len(p.history) {
		return nil, nil, 0
	}
	for index := 0; index < p.count; index++ {
		s := &p.history[(p.latest-index+len(p.history))%len(p.history)]
		if cadence == 2 && s.frame&1 == 0 {
			continue
		}
		if float64(s.frame) <= p.drawFrame && (before == nil || s.frame > before.frame) {
			before = s
		}
		if float64(s.frame) >= p.drawFrame && (after == nil || s.frame < after.frame) {
			after = s
		}
	}
	if before == nil || after == nil {
		return nil, nil, 0
	}
	if after.frame != before.frame {
		fraction = (p.drawFrame - float64(before.frame)) / float64(after.frame-before.frame)
	}
	return before, after, fraction
}

func mix(first, second int, fraction float64) float64 {
	return float64(first) + float64(second-first)*fraction
}

func safePosition(x1, y1, x2, y2 int, fraction float64, fallbackX, fallbackY int) (float64, float64) {
	if abs(x2-x1) > 32 || abs(y2-y1) > 32 {
		return float64(fallbackX), float64(fallbackY)
	}
	return mix(x1, x2, fraction), mix(y1, y2, fraction)
}

func (p *presentation) player(index int, current engine.Player) (float64, float64) {
	before, after, alpha := p.samples(1)
	if before != nil && after != nil {
		a, b := before.players[index], after.players[index]
		if a.Active && b.Active && a.Dying == 0 && b.Dying == 0 && a.Lives == current.Lives && b.Lives == current.Lives {
			return safePosition(a.X, a.Y, b.X, b.Y, alpha, current.X, current.Y)
		}
	}
	return float64(current.X), float64(current.Y)
}

func (p *presentation) enemy(current engine.Enemy) (float64, float64) {
	before, after, alpha := p.samples(2)
	if before != nil && after != nil {
		for _, a := range before.enemies {
			if a.ID != current.ID {
				continue
			}
			for _, b := range after.enemies {
				if b.ID == current.ID {
					return safePosition(a.X, a.Y, b.X, b.Y, alpha, current.X, current.Y)
				}
			}
		}
	}
	return float64(current.X), float64(current.Y)
}

func matchingShot(old, current engine.Bullet, elapsed int) bool {
	return old.Player == current.Player && old.Slot == current.Slot && old.Graphic == current.Graphic && old.Nova == current.Nova && old.VX == current.VX && old.VY == current.VY && old.Age+elapsed == current.Age && old.Delay == 0 && abs(old.X-current.X) <= 16*elapsed+2 && abs(old.Y-current.Y) <= 16*elapsed+2
}

func (p *presentation) shot(current engine.Bullet, primary bool, frame int) (float64, float64) {
	before, after, alpha := p.samples(1)
	if before != nil && after != nil {
		aShots, bShots := before.enemyShots, after.enemyShots
		if primary {
			aShots, bShots = before.playerShots, after.playerShots
		}
		for _, a := range aShots {
			if !matchingShot(a, current, frame-before.frame) {
				continue
			}
			for _, b := range bShots {
				if matchingShot(b, current, frame-after.frame) && abs(b.X-a.X) <= 16 && abs(b.Y-a.Y) <= 16 {
					x, y := safePosition(a.X, a.Y, b.X, b.Y, alpha, current.X, current.Y)
					if !primary {
						// Hostile shots share the terrain camera, even though their
						// movement advances on every PAL field.
						x = mix(a.X+before.camera, b.X+after.camera, alpha) - p.camera
					}
					return x, y
				}
			}
		}
	}
	return float64(current.X), float64(current.Y)
}

func (p *presentation) pickup(current engine.Pickup, frame int) (float64, float64) {
	before, after, alpha := p.samples(2)
	if before != nil && after != nil {
		for _, a := range before.pickups {
			if a.PoolSlot != current.PoolSlot || a.Weapon != current.Weapon || a.Nova != current.Nova || a.Age+(frame-before.frame)/2 != current.Age {
				continue
			}
			for _, b := range after.pickups {
				if b.PoolSlot == current.PoolSlot && b.Weapon == current.Weapon && b.Nova == current.Nova && b.Age+(frame-after.frame)/2 == current.Age {
					return safePosition(a.X, a.Y, b.X, b.Y, alpha, current.X, current.Y)
				}
			}
		}
	}
	return float64(current.X), float64(current.Y)
}

func (p *presentation) explosion(current engine.Explosion, frame int) (float64, float64) {
	before, after, alpha := p.samples(2)
	if before != nil && after != nil {
		for _, a := range before.explosions {
			if a.PoolSlot != current.PoolSlot || a.Player != current.Player || a.SpriteKind != current.SpriteKind || a.Age > current.Age {
				continue
			}
			for _, b := range after.explosions {
				if b.PoolSlot == a.PoolSlot && b.Player == a.Player && b.SpriteKind == a.SpriteKind && b.Age >= a.Age && b.Age <= current.Age && abs(b.X-a.X) <= 4 && abs(b.Y-a.Y) <= 4 {
					return safePosition(a.X, a.Y, b.X, b.Y, alpha, current.X, current.Y)
				}
			}
		}
	}
	return float64(current.X), float64(current.Y)
}

func (p *presentation) ray(index int, current engine.NovaRay) (float64, float64) {
	before, after, alpha := p.samples(1)
	if before != nil && after != nil && index < len(before.rays) && index < len(after.rays) {
		a, b := before.rays[index], after.rays[index]
		return safePosition(a.X, a.Y, b.X, b.Y, alpha, current.X, current.Y)
	}
	return float64(current.X), float64(current.Y)
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// DrawFinalScreen preserves fractional movement at physical display resolution.
// Scaling a 320-pixel framebuffer first would discard the intermediate positions.
func (g *Game) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	ebiten.DefaultDrawFinalScreen(screen, offscreen, geoM)
	p := g.presentation
	if p == nil || !p.smooth || g.Core.Mode != engine.Playing || g.paused || g.optionsOpen || g.showScores || len(g.scorePlayers) != 0 {
		return
	}
	// A linear final sample also preserves subpixel transitions on small Android
	// displays, where one original terrain pixel spans fewer than two pixels.
	// Original textures themselves still use nearest-neighbor sampling.
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(1/float64(p.density), 1/float64(p.density))
	op.GeoM.Translate(float64((g.windowWidth()-320)/2+16), 24)
	op.GeoM.Concat(geoM)
	screen.DrawImage(g.view, op)
	if g.touchEnabled {
		g.drawTouchFinal(screen, geoM)
	}
	// Only a size change reallocates the viewport; normal frames reuse it.
	density := min(8, max(4, int(math.Ceil(math.Abs(geoM.Element(0, 0))))))
	if density != p.density {
		p.density = density
	}
}

func (g *Game) drawTouchFinal(screen ebiten.FinalScreen, geoM ebiten.GeoM) {
	if g.art.touchButtons == nil {
		return
	}
	op := &ebiten.DrawImageOptions{GeoM: geoM}
	screen.DrawImage(g.art.touchButtons, op)
	center, stick, active := g.touch.Stick()
	x, y := float64(40), float64(181)
	if active {
		x, y = center.X, center.Y
	}
	op.GeoM.Reset()
	op.GeoM.Translate(x-32, y-32)
	op.GeoM.Concat(geoM)
	screen.DrawImage(g.art.touchStick, op)
	if active {
		dx, dy := max(-25, min(25, stick.X-center.X)), max(-25, min(25, stick.Y-center.Y))
		op.GeoM.Reset()
		op.GeoM.Translate(x+dx-9, y+dy-9)
		op.GeoM.Concat(geoM)
		screen.DrawImage(g.art.touchKnob, op)
	}
}
