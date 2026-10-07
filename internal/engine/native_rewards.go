package engine

// enemyExplosion keeps the original eight-step flying-object explosion counter.
// Other presentation animations retain their existing decoded duration.
func enemyExplosion(enemy Enemy) Explosion {
	explosion := Explosion{X: enemy.X, Y: enemy.Y, Duration: 32, SpriteKind: 10, PoolSlot: enemy.PoolSlot}
	if enemy.Definition.FlyingPool {
		explosion.Native = true
		explosion.nativeCount = 8
		explosion.Formation = enemy.Definition.NativeKind == 0
		if enemy.Definition.NativeKind == 6 {
			explosion.WeaponCarrier = true
			explosion.nativeCount = 6
			explosion.Frame = 5
			explosion.SpriteKind = 6
		}
	}
	return explosion
}

func (e *Engine) advanceExplosions() {
	remaining := 0
	for _, enemy := range e.Enemies {
		if enemy.Health >= 0 && enemy.Definition.FlyingPool && enemy.Definition.NativeKind == 0 {
			remaining++
		}
	}
	for _, explosion := range e.Explosions {
		if explosion.Formation {
			remaining++
		}
	}
	kept := e.Explosions[:0]
	for _, explosion := range e.Explosions {
		explosion.Age++
		if explosion.Native {
			clock := e.nativeClock()
			if e.Mode == Playing {
				if e.Frame&1 == 0 {
					kept = append(kept, explosion)
					continue
				}
				clock = e.Frame - 1
			}
			if clock&2 == 0 {
				explosion.nativeCount--
			}
			if explosion.nativeCount != 0 {
				if explosion.WeaponCarrier {
					explosion.Frame = 11 - int(explosion.nativeCount)
				} else {
					explosion.Frame = 8 - int(explosion.nativeCount)
				}
				kept = append(kept, explosion)
				continue
			}
			if explosion.Formation {
				// The original counts every active kind-zero record, including other
				// explosions. The last completed record becomes a Nova capsule.
				if remaining == 1 {
					oldPhase := e.npcPhase
					if e.Mode == Playing {
						e.npcPhase = true
					}
					e.releaseNativeCapsuleSlot(explosion.X, explosion.Y, true, explosion.PoolSlot)
					e.npcPhase = oldPhase
				}
				remaining--
			}
			if explosion.WeaponCarrier {
				oldPhase := e.npcPhase
				if e.Mode == Playing {
					e.npcPhase = true
				}
				e.releaseNativeCapsuleSlot(explosion.X, explosion.Y, false, explosion.PoolSlot)
				e.npcPhase = oldPhase
			}
			continue
		}
		if explosion.Age < explosion.Duration {
			kept = append(kept, explosion)
		}
	}
	e.Explosions = kept
}

func (e *Engine) releaseNativeCapsule(x, y int, nova bool) {
	slot := e.availableFlyingSlot()
	if slot < 0 {
		return
	}
	e.releaseNativeCapsuleSlot(x, y, nova, slot)
}

func (e *Engine) releaseNativeCapsuleSlot(x, y int, nova bool, slot int) {
	definition := Definition{Graphic: 5, Sprite: "flying_5_0", Collectable: true}
	if e.Data != nil && len(e.Data.Loader) != 0 {
		if table, err := nativeFlyingTable(e.Data.Loader, e.Data.LoaderBase); err == nil {
			if original, err := decodeFlyingDefinition(e.Data.Loader, e.Data.LoaderBase, table+5*32, 5); err == nil {
				definition = original
			}
		}
	}
	pickup := Pickup{X: x + 8, Y: y + 12, Graphic: definition.Graphic, Sprite: definition.Sprite, PoolSlot: slot, SlotHeld: true}
	if nova {
		pickup.Nova = 1
	} else {
		pickup.Weapon = int(e.nextRandom()&6) >> 1
	}
	// The source reprocesses the transformed flying slot immediately, so its
	// capsule movement and source frame selection occur in this same update.
	if e.updateNativePickup(&pickup) {
		e.Pickups = append(e.Pickups, pickup)
	}
}

// availableFlyingSlot counts each original slot once even while the Go enemy
// and its newly staged explosion temporarily represent that same record.
func (e *Engine) availableFlyingSlot() int {
	var occupied [EnemyShotLimit]bool
	mark := func(slot int) {
		if slot >= 0 && slot < EnemyShotLimit {
			occupied[slot] = true
		}
	}
	for _, enemy := range e.Enemies {
		if enemy.Definition.FlyingPool {
			mark(enemy.PoolSlot)
		}
	}
	for _, explosion := range e.Explosions {
		if explosion.Native {
			mark(explosion.PoolSlot)
		}
	}
	for _, pickup := range e.Pickups {
		if pickup.SlotHeld {
			mark(pickup.PoolSlot)
		}
	}
	for slot := EnemyShotLimit - 1; slot >= 0; slot-- {
		if !occupied[slot] {
			return slot
		}
	}
	return -1
}
