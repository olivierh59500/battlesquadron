package engine

// AimedVelocity retains the original $47FC-$4838 aimed-shot arithmetic. The
// dominant component uses the selected speed; the shorter component uses DIVU
// with an odd divisor and then an eight-bit shift, preserving the Amiga's
// quantized trajectory rather than a floating-point normalized direction.
func AimedVelocity(dx, dy, speed int) (vx, vy int) {
	x, y := abs(dx), abs(dy)
	if y >= x {
		vx = int(uint16(uint32(x)*uint32(speed<<8)/uint32(y|1))) << 8
		vy = speed << 16
	} else {
		vx = speed << 16
		vy = int(uint16(uint32(y)*uint32(speed<<8)/uint32(x|1))) << 8
	}
	if dx < 0 {
		vx = -vx
	}
	if dy < 0 {
		vy = -vy
	}
	return vx, vy
}

// fireNativeAimed emits an original aimed projectile from the controller's
// cached shot point, before that controller moves the object this update.
func (e *Engine) fireNativeAimed(enemy *Enemy, xOffset, yOffset int) {
	x, y := enemy.X+xOffset, enemy.Y+yOffset
	target, distance := -1, int(^uint(0)>>1)
	for index, player := range e.Players {
		if !player.Active || player.Lives == 0 || player.Dying > 0 {
			continue
		}
		candidate := abs(player.X+12-x) + abs(player.Y+16-y)
		if candidate <= distance {
			target, distance = index, candidate
		}
	}
	if target < 0 {
		return
	}
	player := e.Players[target]
	e.fireNativeVector(x, y, player.X+12-x, player.Y+16-y)
}

func (e *Engine) fireNativeVector(x, y, dx, dy int) {
	if len(e.EnemyShots) >= e.Options.EnemyProjectileCap || e.NovaFrames > 0 {
		return
	}
	vx, vy := AimedVelocity(dx, dy, e.Options.EnemyProjectileSpeed)
	e.EnemyShots = append(e.EnemyShots, Bullet{X: x, Y: y, VX: vx, VY: vy, Width: 8, Height: 7, Graphic: 0x58, Player: -1, Damage: 1, fixedX: x << 16, fixedY: y << 16})
}
