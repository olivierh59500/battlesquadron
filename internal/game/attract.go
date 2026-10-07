package game

import (
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/replay"
)

// The menu waits fifteen foreground seconds before showing the expert replay.
const attractIdleTicks = 15 * 50
const attractEndingTicks = 4 * 50

type attractState struct {
	data       *engine.Data
	cursor     *replay.Cursor
	menu       *engine.Engine
	idle       int
	endingHold int
	blocked    bool
}

type playerActivity struct {
	active bool
	held   bool
}

// DemoActive distinguishes the expert presentation from a human session.
func (g *Game) DemoActive() bool { return g.attract != nil && g.attract.menu != nil }

// DemoInputBlocked remains true until the gesture that woke the menu ends.
func (g *Game) DemoInputBlocked() bool { return g.attract != nil && g.attract.blocked }

// MenuIdleTicks exposes foreground idle time for platform diagnostics.
func (g *Game) MenuIdleTicks() int {
	if g.attract == nil {
		return 0
	}
	return g.attract.idle
}

func (g *Game) resetMenuIdle() {
	if g.attract != nil {
		g.attract.idle = 0
	}
}

// StopDemo restores the untouched human menu and consumes the waking gesture.
// Settings, score entry and the selected player count belong to that menu.
func (g *Game) StopDemo() {
	if !g.DemoActive() {
		g.resetMenuIdle()
		return
	}
	a := g.attract
	g.Core, a.menu = a.menu, nil
	a.idle, a.endingHold, a.blocked = 0, 0, true
	g.paused = false
	g.lastFire = [2]bool{}
	g.CancelInput()
	g.resetPresentation()
	if g.sound != nil {
		g.sound.UseMenu(true)
		g.sound.PlayTrack(1)
	}
	if g.audioPlayer != nil && !g.mute {
		g.audioPlayer.Play()
	}
}

func (g *Game) resetPresentation() {
	if g.presentation != nil {
		g.presentation.count = 0
	}
}

func (g *Game) restartDemo() error {
	a := g.attract
	// A fresh engine also resets the camera, input latch and private counters.
	// Engine.Start alone intentionally retains some state of a human session.
	core, err := engine.New(a.data)
	if err != nil {
		return err
	}
	core.Options = engine.DefaultOptions()
	core.Start(1)
	g.Core = core
	a.cursor.Reset()
	a.idle, a.endingHold, a.blocked = 0, 0, false
	g.paused = false
	g.CancelInput()
	g.resetPresentation()
	if g.sound != nil {
		g.sound.UseMenu(false)
		g.sound.PlayTrack(1)
	}
	return nil
}

// updateAttract runs before hotkeys and pointer handlers, which have side effects.
// Playback supplies only recorded inputs to the ordinary native simulation.
func (g *Game) updateAttract(activity playerActivity) (bool, error) {
	a := g.attract
	if a == nil {
		return false, nil
	}
	if a.blocked {
		a.idle = 0
		if !activity.held {
			a.blocked = false
			g.lastFire = [2]bool{}
			g.pointerSuppressed = false
			g.previousPointers = nil
		}
		return true, nil
	}
	if g.DemoActive() {
		if activity.active {
			g.StopDemo()
			return true, nil
		}
		inputs, ok := a.cursor.Next()
		if ok {
			g.Core.Tick(inputs)
			g.playEvents()
			return true, nil
		}
		// Keep the original staff card visible briefly, then replay the full run.
		// Demo scores never enter the high-score or persistent-settings flow.
		a.endingHold++
		if g.Core.Mode != engine.Ending || a.endingHold >= attractEndingTicks {
			return true, g.restartDemo()
		}
		return true, nil
	}
	eligible := g.Core.Mode == engine.Title && !g.optionsOpen && !g.showScores && len(g.scorePlayers) == 0 && g.SmokeFrames == 0
	if !eligible || activity.active {
		a.idle = 0
		return false, nil
	}
	a.idle++
	if a.idle < attractIdleTicks {
		return false, nil
	}
	a.menu = g.Core
	if err := g.restartDemo(); err != nil {
		g.Core, a.menu = a.menu, nil
		return true, err
	}
	return true, nil
}
