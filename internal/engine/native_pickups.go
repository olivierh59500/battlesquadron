package engine

// nativePickupState preserves the original capsule's fractional movement and
// subtype. A Nova uses subtype ten; weapon families use zero, two, four and six.
type nativePickupState struct {
	initialized        bool
	x, y, vx           int
	direction, subtype byte
}

// updateNativePickup translates the supplied disk's kind-five handler $89EA.
// Its live vertical step is $6000; the source attract mode uses $4000 instead.
func (e *Engine) updateNativePickup(pickup *Pickup) bool {
	state := &pickup.native
	if !state.initialized {
		state.initialized = true
		state.x = pickup.X << 16
		state.y = pickup.Y << 16
		if pickup.Nova > 0 {
			state.subtype = 10
		} else {
			state.subtype = byte(pickup.Weapon * 2)
		}
		state.vx = 2 << 16
		if pickup.X >= 136 {
			state.vx = -2 << 16
			state.direction = 255
		}
	}
	if int8(state.direction) < 0 {
		if state.x>>16 < 71 {
			state.direction = ^state.direction
		}
		if state.vx > -4<<16 {
			state.vx -= 0x2000
		}
	} else {
		if state.x>>16 > 201 {
			state.direction = ^state.direction
		}
		if state.vx < 4<<16 {
			state.vx += 0x2000
		}
	}
	state.y += 0x6000
	if state.y>>16 >= 208 {
		return false
	}
	state.x += state.vx
	pickup.X, pickup.Y = state.x>>16, state.y>>16
	if state.subtype == 10 {
		pickup.Nova = 1
		pickup.Frame = 10 + (e.nativeClock() >> 2 & 1)
	} else {
		if state.vx == 0 {
			state.subtype = (state.subtype + 2) & 6
		}
		pickup.Weapon = int(state.subtype >> 1)
		pickup.Frame = int(state.subtype) + (e.nativeClock() >> 2 & 1)
	}
	return true
}

// nativeClock is the original display-frame word: $5468 advances it at both
// halves of every PAL game frame, while Engine.Frame counts complete updates.
func (e *Engine) nativeClock() int { return e.Frame * 2 }

// Clock returns the original two-phase display word for authentic animation.
func (e *Engine) Clock() int { return e.nativeClock() }
