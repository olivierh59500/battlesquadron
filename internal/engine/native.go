package engine

// nativeTrackerState is the source kind-eight timer and firing countdown.
type nativeTrackerState struct {
	tracking, shotTimer int
	flags               byte
	initialized         bool
}

// moveNativeEnemy handles verified non-scripted flying-object movement seams.
// Remaining state machines keep their decoded state until a translation exists.
func (e *Engine) moveNativeEnemy(enemy *Enemy) bool {
	if !enemy.Definition.FlyingPool {
		return false
	}
	switch enemy.Definition.NativeKind {
	case 1:
		// The original homing object accelerates on X and scrolls at a fixed Y rate.
		cave := len(e.Data.Stages) > e.Stage && e.Data.Stages[e.Stage].Mode != 0
		maximum, delta, vertical := 131072, 8192, 2
		if cave {
			maximum, delta, vertical = 196608, 12288, 3
		}
		target, distance, passed := -1, int(^uint(0)>>1), true
		for index, player := range e.Players {
			if !player.Active || player.Lives == 0 || player.Dying > 0 {
				continue
			}
			candidate := abs(player.X - enemy.X)
			if candidate < distance {
				target, distance = index, candidate
			}
			passed = passed && enemy.Y-30 > player.Y
		}
		if passed || target < 0 {
			if enemy.VX < 0 {
				enemy.VX = min(0, enemy.VX+2048)
			} else {
				enemy.VX = max(0, enemy.VX-2048)
			}
		} else if e.Players[target].X < enemy.X {
			enemy.VX = max(-maximum, enemy.VX-delta)
		} else {
			enemy.VX = min(maximum, enemy.VX+delta)
		}
		enemy.fixedX += enemy.VX
		enemy.X = enemy.fixedX >> 16
		enemy.Y += vertical
		enemy.fixedY = enemy.Y << 16
		return true
	case 7:
		// Kind seven is anchored to one terrain-scroll pixel per game update.
		enemy.Y++
		enemy.fixedY = enemy.Y << 16
		if enemy.Y >= 208 {
			enemy.Health = -1
		}
		return true
	case 8:
		return e.moveNativeTracker(enemy)
	case 12:
		// The source adds one scroll pixel and an extra pixel on odd game frames.
		enemy.Y++
		if e.nativeClock()&2 == 0 {
			enemy.Y++
		}
		enemy.fixedY = enemy.Y << 16
		return true
	}
	return e.moveNativeAdditional(enemy)
}

func (e *Engine) moveNativeTracker(enemy *Enemy) bool {
	state := &enemy.tracker
	if !state.initialized {
		state.initialized = true
		state.tracking = enemy.Definition.TrackingFrames
		if state.tracking == 0 {
			state.tracking = 200
		}
		state.shotTimer = 20
	}
	direction := state.flags
	if state.tracking == 0 {
		if enemy.X <= -32 || enemy.X > 288 {
			enemy.Health = -1
			return true
		}
		// The source's retreat line is the inverted low byte of the game frame.
		line := int(byte(e.nativeClock()) ^ 255)
		if enemy.Y > line {
			direction |= 2
		}
	} else {
		state.tracking--
		if state.tracking == 0 && enemy.X+e.CameraX <= 176 {
			state.flags |= 1
		}
		firstX := e.Players[0].X + 12 - enemy.X
		firstY := e.Players[0].Y + 16 - enemy.Y
		secondX := e.Players[1].X + 12 - enemy.X
		secondY := e.Players[1].Y + 16 - enemy.Y
		if abs(firstX)+abs(firstY) < abs(secondX)+abs(secondY) {
			if firstX < 0 {
				direction = 1
			} else {
				direction = 0
			}
			if firstY < 0 {
				direction |= 2
			}
		} else {
			if secondX < 0 {
				direction = 1
			} else {
				direction = 0
			}
			if secondY < 0 {
				direction |= 2
			}
		}
	}
	const maximum, acceleration = 0x20000, 0x2000
	// Original CMP guards include equality, allowing a one-step overshoot.
	if direction&1 != 0 {
		if enemy.VX >= -maximum {
			enemy.VX -= acceleration
		}
	} else if enemy.VX <= maximum {
		enemy.VX += acceleration
	}
	if direction&2 != 0 && enemy.VY >= -maximum {
		enemy.VY -= acceleration
	} else if enemy.VY <= maximum {
		enemy.VY += acceleration
	}
	enemy.Frame = nativeVelocityFrame(enemy.VX, enemy.VY)
	state.shotTimer = (state.shotTimer - 1) & 255
	if state.shotTimer == 0 {
		state.shotTimer = e.Options.EnemyFireDelay & 255
		e.fireEnemy(*enemy)
	}
	enemy.fixedX += enemy.VX
	enemy.fixedY += enemy.VY
	enemy.X, enemy.Y = enemy.fixedX>>16, enemy.fixedY>>16
	return true
}
