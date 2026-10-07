package engine

// nativeAdaptiveState contains the original flying record's mode and alternating
// steering counters. These are Go state fields, never emulated registers.
type nativeAdaptiveState struct {
	mode, direction, opposite byte
	hit                       byte
	fire                      byte
	entered, turned           bool
}

// moveNativeAdditional translates the original kind-four dive and kind-six
// weaving movement, including the fractional velocity and sprite direction.
func (e *Engine) moveNativeAdditional(enemy *Enemy) bool {
	shotCount := len(e.EnemyShots)
	cave := len(e.Data.Stages) > e.Stage && e.Data.Stages[e.Stage].Mode != 0
	switch enemy.Definition.NativeKind {
	case 4:
		state := &enemy.native
		maximum, acceleration, lead, vertical := 3<<16, 0x1000, 72, 3<<16
		if cave {
			maximum, acceleration, lead, vertical = 4<<16, 0x2000, 64, 4<<16
		}
		if state.mode == 0 {
			state.mode = 1
			enemy.Frame = 8
			enemy.VX = 0
			enemy.VY = vertical
			e.nativeSteering(state)
		}
		if state.mode == 1 {
			enemy.Frame = 8
			if !state.entered && enemy.Y >= 0 {
				state.entered = true
				e.fireNativeAimed(enemy, 11, 32)
			}
			target := -1
			distance := int(^uint(0) >> 1)
			for index, player := range e.Players {
				if !player.Active || player.Lives == 0 || player.Dying >= 100 {
					continue
				}
				difference := abs(player.X - enemy.X)
				if difference <= distance {
					target, distance = index, difference
				}
			}
			if target >= 0 && e.Players[target].Y-lead < enemy.Y {
				state.mode = 2
				state.direction = 0
				if e.Players[target].X < enemy.X {
					state.direction = 1
				}
			} else {
				e.nativeWeave(enemy, state)
			}
		}
		if state.mode == 2 {
			if !state.turned && (enemy.Frame == 4 || enemy.Frame == 12) {
				state.turned = true
				if enemy.Frame == 4 {
					e.fireNativeAimed(enemy, 32, 11)
				} else {
					e.fireNativeAimed(enemy, -8, 11)
				}
			}
			if abs(enemy.VX) < maximum {
				if state.direction&1 != 0 {
					enemy.VX -= acceleration
				} else {
					enemy.VX += acceleration
				}
			}
			enemy.VY -= acceleration
			enemy.Frame = nativeVelocityFrame(enemy.VX, enemy.VY)
		}
		if state.hit != 0 {
			state.hit--
			if state.hit&2 != 0 {
				enemy.Frame += 16
			}
		}
		enemy.fixedX += enemy.VX
		enemy.fixedY += enemy.VY
		enemy.X = enemy.fixedX >> 16
		enemy.Y = enemy.fixedY >> 16
		if enemy.X <= -32 || enemy.X >= 288 || enemy.Y <= -32 || enemy.Y >= 208 {
			enemy.Health = -1
			e.EnemyShots = e.EnemyShots[:shotCount]
		}
		return true
	case 6:
		state := &enemy.native
		enemy.Frame = 0
		if state.hit != 0 {
			state.hit--
			enemy.Frame = 4 - int(state.hit)
		}
		if state.direction == 0 {
			e.nativeSteering(state)
		}
		e.nativeWeave(enemy, state)
		enemy.fixedX += enemy.VX
		enemy.X = enemy.fixedX >> 16
		if state.fire == 0 {
			state.fire = byte(e.Options.EnemyFireDelay)
		}
		state.fire--
		switch state.fire {
		case 16:
			e.fireNativeAimed(enemy, 11, 40)
		case 8:
			e.fireNativeVector(enemy.X+2, enemy.Y+40, -32, e.nativeClock()&127|64)
		case 0:
			e.fireNativeVector(enemy.X+20, enemy.Y+40, 32, e.nativeClock()&127|64)
		}
		enemy.fixedY += 0xc000
		enemy.Y = enemy.fixedY >> 16
		if enemy.Y >= 208 {
			enemy.Health = -1
			e.EnemyShots = e.EnemyShots[:shotCount]
		}
		return true
	}
	return false
}

// nativeDamage stages the original surviving-enemy damage animation. Collision
// resolution has already applied the original health subtraction when called.
func (e *Engine) nativeDamage(enemy *Enemy) {
	if !enemy.Definition.FlyingPool {
		return
	}
	switch enemy.Definition.NativeKind {
	case 1:
		enemy.native.hit = 6
	case 4:
		enemy.native.hit = 8
	case 6:
		enemy.native.hit = 4
	}
}

func (e *Engine) nativeSteering(state *nativeAdaptiveState) {
	value := e.nextRandom() | 7
	direction := value & 15
	if value&32 != 0 {
		direction = byte(-int8(direction))
	}
	state.direction = direction
	state.opposite = byte(-int8(direction))
}

func (e *Engine) nativeWeave(enemy *Enemy, state *nativeAdaptiveState) {
	if int8(state.direction) < 0 {
		enemy.VX += 0x1000
		state.direction++
	} else {
		enemy.VX -= 0x1000
		state.direction--
	}
	if state.direction == 0 {
		if state.opposite == 0 {
			e.nativeSteering(state)
			e.nativeWeave(enemy, state)
		} else {
			state.direction = state.opposite
			state.opposite = 0
		}
	}
}

// nativeVelocityFrame retains the signed-word denominator of the source's
// DIVS after a logical long shift, then selects its original angle thresholds.
func nativeVelocityFrame(vx, vy int) int {
	denominator := int(int16(uint32(vx) >> 6))
	sign := 0
	if denominator < 0 {
		sign = 8
	} else if denominator == 0 {
		denominator = 64
	}
	quotient := (-vy) / denominator
	if quotient < -32768 || quotient > 32767 {
		quotient = int(int16(-vy))
	}
	frame := 0
	switch {
	case quotient >= 322:
		frame = 0
	case quotient >= 96:
		frame = 1
	case quotient >= 43:
		frame = 2
	case quotient >= 13:
		frame = 3
	case quotient >= -13:
		frame = 4
	case quotient >= -43:
		frame = 5
	case quotient >= -96:
		frame = 6
	case quotient >= -322:
		frame = 7
	default:
		frame = 8
	}
	return (frame + sign) & 15
}
