package engine

type box struct{ left, top, right, bottom int }

// The original collision pass includes the lower edge and excludes the upper.
func overlap(first, second box) bool {
	return first.left < second.right && first.right >= second.left && first.top < second.bottom && first.bottom >= second.top
}

func playerBox(player Player) box {
	return box{player.X + 7, player.Y + 7, player.X + 25, player.Y + 23}
}

func enemyBox(enemy Enemy) box {
	return box{enemy.X, enemy.Y, enemy.X + enemy.Definition.Width, enemy.Y + enemy.Definition.Height}
}

func (e *Engine) collisions() {
	shots := e.PlayerShots[:0]
	for _, bullet := range e.PlayerShots {
		if bullet.Delay > 0 {
			shots = append(shots, bullet)
			continue
		}
		hit := false
		shot := box{bullet.X, bullet.Y - 10, bullet.X + bullet.Width, bullet.Y + bullet.Height + 10}
		if bullet.Nova {
			shot = box{bullet.X - 32, bullet.Y - 32, bullet.X + 48, bullet.Y + 48}
		}
		for enemyIndex, enemy := range e.Enemies {
			if enemy.Health < 0 || enemy.Definition.Kind == 0x27 || !overlap(shot, enemyBox(enemy)) {
				continue
			}
			damage := bullet.Damage
			if damage < 0 {
				damage = 2
			}
			e.damageEnemy(enemyIndex, damage, bullet.Player)
			hit = bullet.Damage >= 0
			break
		}
		if !hit {
			shots = append(shots, bullet)
		}
	}
	e.PlayerShots = shots
	for index, player := range e.Players {
		if !player.Active || player.Lives == 0 || player.Dying > 0 || player.Respawn > 0 {
			continue
		}
		bounds := playerBox(player)
		if player.Invulnerable == 0 && !e.Options.Invulnerable {
			for _, enemy := range e.Enemies {
				// Ground scenery is below the ships; original hazards use a separate list.
				if !enemy.Definition.Ground && enemy.Health >= 0 && overlap(bounds, enemyBox(enemy)) {
					e.HitPlayer(index)
					break
				}
			}
			kept := e.EnemyShots[:0]
			for _, bullet := range e.EnemyShots {
				if overlap(bounds, box{bullet.X, bullet.Y, bullet.X + bullet.Width, bullet.Y + bullet.Height}) {
					e.HitPlayer(index)
				} else {
					kept = append(kept, bullet)
				}
			}
			e.EnemyShots = kept
		}
		pickups := e.Pickups[:0]
		for _, pickup := range e.Pickups {
			if overlap(bounds, box{pickup.X, pickup.Y, pickup.X + 24, pickup.Y + 24}) {
				e.collect(index, pickup)
			} else {
				pickups = append(pickups, pickup)
			}
		}
		e.Pickups = pickups
	}
}

func (e *Engine) damageEnemy(index, damage, player int) {
	enemy := &e.Enemies[index]
	if enemy.Health < 0 {
		return
	}
	enemy.Health -= damage
	// Original records die only after damage exceeds their remaining health byte.
	if enemy.Health < 0 {
		if player >= 0 && player < 2 {
			e.Players[player].Score += enemy.Definition.Score
		}
		e.Explosions = append(e.Explosions, Explosion{X: enemy.X, Y: enemy.Y, Duration: 32})
		e.Events = append(e.Events, Event{Kind: "explosion", Player: player, Value: int(enemy.Definition.Kind)})
	} else {
		e.nativeDamage(enemy)
		e.Events = append(e.Events, Event{Kind: "hit", Player: player})
	}
}

// HitPlayer enters the original seventy-frame ship-explosion sequence.
func (e *Engine) HitPlayer(index int) {
	if index < 0 || index >= 2 {
		return
	}
	p := &e.Players[index]
	if !p.Active || p.Lives == 0 || p.Dying > 0 || p.Invulnerable > 0 || e.Options.Invulnerable {
		return
	}
	p.Dying, p.Invulnerable = 70, 9999
	p.Y -= 12
	e.Explosions = append(e.Explosions, Explosion{X: p.X, Y: p.Y, Duration: 70, Player: true})
	kept := e.PlayerShots[:0]
	for _, bullet := range e.PlayerShots {
		if bullet.Player != index {
			kept = append(kept, bullet)
		}
	}
	e.PlayerShots = kept
	e.Events = append(e.Events, Event{Kind: "death", Player: index})
}

func (e *Engine) collect(index int, pickup Pickup) {
	p := &e.Players[index]
	if pickup.Nova > 0 {
		p.Nova = min(8, p.Nova+pickup.Nova)
	} else {
		p.Weapon, p.Level = max(0, min(3, pickup.Weapon)), min(5, p.Level+1)
		p.Cooldown, p.Repeat = 0, 0
		kept := e.PlayerShots[:0]
		for _, bullet := range e.PlayerShots {
			if bullet.Player != index {
				kept = append(kept, bullet)
			}
		}
		e.PlayerShots = kept
	}
	e.Events = append(e.Events, Event{Kind: "pickup", Player: index})
}
