package engine

import "slices"

// Clone creates an independent simulation branch for input planning and replay
// checks. Original bitmaps, tables and motion scripts are immutable and shared.
// Stage descriptors are copied because the final encounter changes its stream.
func (e *Engine) Clone() *Engine {
	if e == nil {
		return nil
	}
	copyEngine := *e
	if e.Data != nil {
		data := *e.Data
		data.Stages = slices.Clone(e.Data.Stages)
		copyEngine.Data = &data
	}
	copyEngine.Enemies = slices.Clone(e.Enemies)
	copyEngine.PlayerShots = slices.Clone(e.PlayerShots)
	copyEngine.EnemyShots = slices.Clone(e.EnemyShots)
	copyEngine.Pickups = slices.Clone(e.Pickups)
	copyEngine.Explosions = slices.Clone(e.Explosions)
	copyEngine.Events = slices.Clone(e.Events)
	copyEngine.NovaRays = slices.Clone(e.NovaRays)
	copyEngine.groundSpawns = slices.Clone(e.groundSpawns)
	copyEngine.Campaign.Gates = slices.Clone(e.Campaign.Gates)
	return &copyEngine
}

// EnemyTouchable exposes the same source immunity checks used by collisions.
func (e *Engine) EnemyTouchable(enemy Enemy) bool {
	return enemy.Health >= 0 && enemy.Definition.Kind != 39 && e.nativeGroundTouchable(enemy) && e.nativeSpecialTouchable(enemy)
}

// EnemyContactDangerous distinguishes airborne hazards from ground scenery.
func (e *Engine) EnemyContactDangerous(enemy Enemy) bool {
	return !enemy.Definition.Ground && enemy.Health >= 0 && e.nativeSpecialContact(enemy)
}

// EnemyCollisionBounds exposes original weak points to the validation player.
func (e *Engine) EnemyCollisionBounds(enemy Enemy) (x, y, width, height int) {
	box := e.originalEnemyBox(enemy)
	return box.left, box.top, box.right - box.left, box.bottom - box.top
}

// GroundWreckBounds returns the source's inclusive canonical-position rectangle.
func (e *Engine) GroundWreckBounds(enemy Enemy) (x, y, width, height int, collectable bool) {
	state := enemy.ground
	if !enemy.Definition.Ground || !state.initialized || state.template.Kind != 32 || state.hidden || state.frame != state.template.FinalFrame {
		return 0, 0, 0, 0, false
	}
	x, y = enemy.X-8, enemy.Y-12
	if e.Stage < len(e.Data.Stages) && e.Data.Stages[e.Stage].Mode == 3 {
		x -= 8
	}
	return x, y, 28, 28, true
}
