package engine

// NewCameraTarget translates the original $9BC2-$9BFE canonical-ship formula.
// The source includes dying ships (marker100) and ignores respawning or disabled
// ships (negative marker150/255). The returned offset is relative to $100.
func NewCameraTarget(players [2]Player) int {
	sum, living := uint16(0), 0
	for _, player := range players {
		if !player.Active || player.Respawn > 0 || player.Lives <= 0 && player.Dying == 0 {
			continue
		}
		sum += uint16(player.X)
		living++
	}
	if living == 0 {
		return 48
	}
	quarter := sum >> uint(1+living)
	return int(quarter + quarter>>1)
}

// updateCamera keeps world objects attached to the same original terrain while
// ships and their primary projectiles remain in canonical viewport coordinates.
func (e *Engine) updateCamera() {
	target := NewCameraTarget(e.Players)
	previous := e.CameraX
	if e.CameraX < target {
		e.CameraX++
	} else if e.CameraX > target {
		e.CameraX--
	}
	delta := e.CameraX - previous
	if delta == 0 {
		return
	}
	for index := range e.Enemies {
		enemy := &e.Enemies[index]
		enemy.X -= delta
		enemy.fixedX -= delta << 16
	}
	for index := range e.EnemyShots {
		shot := &e.EnemyShots[index]
		shot.X -= delta
		shot.fixedX -= delta << 16
	}
	for index := range e.Explosions {
		if !e.Explosions[index].Player {
			e.Explosions[index].X -= delta
		}
	}
}
