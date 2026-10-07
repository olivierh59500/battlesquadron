package engine

import (
	"encoding/binary"
	"fmt"
)

// nativeSpecialState owns the original multipart-boss timers and health bytes.
// Parts retain their record while damaged; removing a gun would lose its phase.
type nativeSpecialState struct {
	initialized, entered, dead                    bool
	hp, phase, timer, dying, flash, pending, path int
	steerX, steerY                                int
	budget                                        byte
}

func (e *Engine) specialPart(slot, kind int) *Enemy {
	for index := range e.Enemies {
		enemy := &e.Enemies[index]
		if enemy.Definition.FlyingPool && enemy.PoolSlot == slot && enemy.Definition.NativeKind == kind {
			return enemy
		}
	}
	return nil
}

func (e *Engine) initializeSpecial(enemy *Enemy) {
	if enemy.special.initialized {
		return
	}
	enemy.special.initialized = true
	enemy.special.hp = enemy.Health
	enemy.special.budget = 255
	enemy.Health = max(1, enemy.Health)
}

// moveNativeSpecial handles the source's kind-two and kind-nine multipart groups.
func (e *Engine) moveNativeSpecial(enemy *Enemy) bool {
	if !enemy.Definition.FlyingPool {
		return false
	}
	switch enemy.Definition.NativeKind {
	case 2:
		e.initializeSpecial(enemy)
		if e.Scroll < 240 {
			return true
		}
		return e.moveFinalPart(enemy)
	case 9:
		e.initializeSpecial(enemy)
		if enemy.PoolSlot != 8 {
			e.applySpecialPending(enemy)
		}
		return e.moveCaveBossPart(enemy)
	case 10:
		// Kind ten is the boss's shared, temporary impact renderer, not a hostile.
		return true
	}
	return false
}

func (e *Engine) moveFinalPart(enemy *Enemy) bool {
	state := &enemy.special
	clock := e.nativeClock()
	body, head := e.specialPart(8, 2), e.specialPart(11, 2)
	switch enemy.PoolSlot {
	case 8:
		enemy.Definition.Width, enemy.Definition.Height = 96, 28
		step := 1
		if head != nil && head.special.timer >= 250 {
			step = 2
			state.steerX &^= 1
			state.steerY &^= 1
		}
		if head == nil || head.special.timer == 0 || head.special.timer >= 250 {
			if state.dying == 0 {
				e.finalWander(enemy, step)
			}
		}
		if head != nil && head.special.timer >= 250 && state.dying == 0 && clock&6 == 0 {
			if len(e.Data.Random) != 0 {
				for e.nextRandom() >= 224 {
				}
			}
			e.fireEnemy(*enemy)
		}
		if head != nil && head.special.timer >= 250 && state.pending != 0 {
			previous := state.budget
			state.budget -= byte(state.pending)
			state.pending = 0
			state.flash = 6
			if previous < 128 && state.budget >= 128 {
				state.dying, state.flash = 100, 102
				e.Events = append(e.Events, Event{Kind: "explosion", Value: 29})
			}
		}
		ending := state.dying == 1
		if state.dying > 1 {
			state.dying--
		} else if state.dying == 1 {
			e.Mode = Ending
			e.Events = append(e.Events, Event{Kind: "ending"})
		}
		if state.flash > 0 && !ending {
			state.flash--
		}
		enemy.Frame = 0
		if head != nil && head.special.timer >= 200 {
			enemy.Frame = 1
			if head.special.timer < 250 && head.special.timer&1 != 0 {
				enemy.Frame = 0
			}
			if head.special.timer >= 250 && state.flash&1 != 0 {
				enemy.Frame = 2
			}
			if ending {
				enemy.Frame = 1
			}
		}
		enemy.Definition.Sprite = fmt.Sprintf("final_body_%d", enemy.Frame)
	case 9, 10:
		if body == nil {
			return true
		}
		enemy.Definition.Width, enemy.Definition.Height = 80, 32
		enemy.X, enemy.Y = body.X-80, body.Y+38
		role := "left"
		if enemy.PoolSlot == 10 {
			enemy.X, enemy.Y, role = body.X+96, body.Y+40, "right"
		}
		if state.phase >= 3 && state.phase < 8 && clock&6 == 0 {
			state.phase++
		}
		enemy.Frame = state.phase
		if state.phase < 3 {
			enemy.Frame = 0
			if clock&32 == 0 {
				enemy.Frame++
			}
			fireClock := clock
			if enemy.PoolSlot == 10 {
				fireClock += 2
			}
			if fireClock&14 == 0 {
				e.nextRandom()
				e.nextRandom()
				e.fireEnemy(*enemy)
			}
			e.applySpecialPending(enemy)
			if state.phase >= 3 {
				enemy.Frame = 3
			} else if state.flash > 0 {
				state.flash--
				enemy.Frame = 0
				if state.flash&1 != 0 {
					enemy.Frame = 2
				}
			}
		} else if state.phase >= 8 && head != nil && head.special.timer >= 200 {
			counter := head.special.timer
			if counter >= 250 || counter&1 == 0 {
				enemy.Frame = 9
				vertical := 0
				if counter >= 250 && clock&32 == 0 {
					enemy.Frame++
					vertical = 1
					if enemy.PoolSlot == 10 {
						vertical = 2
					}
				}
				if body.special.flash != 0 {
					vertical = 0
					enemy.Frame = 10
					if body.special.dying != 1 && body.special.flash&1 != 0 {
						enemy.Frame++
					}
				}
				enemy.X -= 2
				if enemy.PoolSlot == 10 {
					enemy.Y += 2 + vertical
				} else {
					enemy.Y += 5 + vertical
				}
			}
		}
		enemy.Definition.Sprite = fmt.Sprintf("final_gun_%s_%d", role, enemy.Frame)
	case 11:
		if body == nil {
			return true
		}
		enemy.Definition.Width, enemy.Definition.Height = 96, 48
		enemy.X, enemy.Y = body.X, body.Y+28
		intro := state.phase >= 2 && state.phase < 7
		if state.phase >= 2 && state.phase < 7 && clock&6 == 0 {
			state.phase++
		}
		enemy.Frame = state.phase
		if state.phase < 2 {
			enemy.Frame = 0
			left, right := e.specialPart(9, 2), e.specialPart(10, 2)
			if left != nil && right != nil && left.special.phase >= 3 && right.special.phase >= 3 {
				e.finalHeadFire(*enemy, true)
			}
			e.applySpecialPending(enemy)
			if state.phase >= 2 {
				enemy.Frame = 2
			} else if state.flash > 0 {
				state.flash--
				enemy.Frame = state.flash & 1
			}
		} else if !intro && state.phase >= 7 {
			enemy.Frame = 7
			if state.timer == 0 {
				if clock&16 != 0 {
					enemy.Frame++
				}
				e.finalHeadFire(*enemy, false)
				e.applySpecialPending(enemy)
				if state.flash > 0 {
					state.flash--
					enemy.Frame = 8 + state.flash&1
				}
			}
			if state.timer > 0 && state.timer < 250 {
				state.timer++
				if state.timer >= 50 || state.timer&1 == 0 {
					enemy.Frame = 10
				}
				if state.timer >= 200 && state.timer&1 != 0 {
					enemy.Frame = 11
					enemy.X -= 2
				}
			} else if state.timer >= 250 {
				enemy.Frame = 12
				enemy.X -= 2
				if body.special.flash == 0 {
					if clock&32 == 0 {
						enemy.Frame = 11
					}
				} else if body.special.dying == 1 || body.special.dying > 0 && body.special.flash&1 == 0 {
					enemy.Frame = 14
				} else if body.special.flash&1 != 0 {
					enemy.Frame = 13
				}
			}
		}
		enemy.Definition.Sprite = fmt.Sprintf("final_head_%d", enemy.Frame)
	}
	enemy.fixedX, enemy.fixedY = enemy.X<<16, enemy.Y<<16
	return true
}

func (e *Engine) finalHeadFire(enemy Enemy, initial bool) {
	clock := e.nativeClock()
	if clock&6 != 0 {
		return
	}
	e.nextRandom()
	if !initial || clock&8 == 0 {
		e.nextRandom()
	}
	e.fireEnemy(enemy)
}

func (e *Engine) finalWander(enemy *Enemy, step int) {
	state := &enemy.special
	if state.steerY == 0 {
		value := int(e.nextRandom()&14 | 2)
		if enemy.Y < -16 {
			value &^= 8
		} else if enemy.Y >= 16 || value&8 != 0 {
			value = int(int8(byte(value | 248)))
		}
		state.steerY = value
	}
	if state.steerY < 0 {
		state.steerY += step
		enemy.Y--
	} else {
		state.steerY -= step
		enemy.Y++
	}
	if state.steerX == 0 {
		value := int(e.nextRandom()&30 | 2)
		x := enemy.X + e.CameraX + 256
		if x < 336 {
			value &^= 16
		} else if x >= 464 || value&16 != 0 {
			value = int(int8(byte(value | 240)))
		}
		state.steerX = value
	}
	if state.steerX < 0 {
		state.steerX += step
		enemy.X--
	} else {
		state.steerX -= step
		enemy.X++
	}
}

func (e *Engine) moveCaveBossPart(enemy *Enemy) bool {
	state := &enemy.special
	body := e.specialPart(8, 9)
	mode := 0
	if e.Stage < len(e.Data.Stages) {
		mode = e.Data.Stages[e.Stage].Mode
	}
	if enemy.PoolSlot == 9 {
		if body == nil {
			return true
		}
		enemy.X, enemy.Y = body.X, body.Y+32
		enemy.Definition.Width, enemy.Definition.Height = 96, 48
		if mode != 1 {
			enemy.X, enemy.Y = body.X+29, body.Y+48
			enemy.Definition.Width, enemy.Definition.Height = 64, 39
		}
		enemy.Frame = 0
		if state.dead || state.hp < 16 {
			enemy.Frame = 3
		} else if state.flash > 0 {
			state.flash--
			if state.flash&1 != 0 {
				enemy.Frame = 2
			}
		} else if mode == 1 && e.nativeClock()&16 == 0 {
			enemy.Frame = 1
		}
		enemy.Definition.Sprite = fmt.Sprintf("boss_%d_turret_%d", mode, enemy.Frame)
		enemy.fixedX, enemy.fixedY = enemy.X<<16, enemy.Y<<16
	} else if enemy.PoolSlot == 8 {
		if state.dead {
			enemy.fixedY += 1 << 16
			enemy.Y = enemy.fixedY >> 16
			if enemy.Y >= 208 {
				for index := range e.Enemies {
					if e.Enemies[index].Definition.FlyingPool && e.Enemies[index].PoolSlot >= 8 {
						e.Enemies[index].Health = -1
					}
				}
				enemy.Health = -1
			}
		} else if mode == 1 {
			if len(enemy.Script) != 0 {
				motion := enemy.Script[state.path%len(enemy.Script)]
				vx := motion.VX
				if e.Scroll >= 4000 {
					vx = -vx
				}
				enemy.fixedX += vx
				enemy.fixedY += motion.VY
				state.path++
			}
			if state.timer < 250 {
				state.timer++
				enemy.fixedY += 0x4e20
			}
			enemy.X, enemy.Y = enemy.fixedX>>16, enemy.fixedY>>16
		} else {
			e.moveCaveBossWander(enemy)
		}
		if mode != 1 {
			enemy.Definition.Width, enemy.Definition.Height = 128, 48
		}
		e.caveBossFire(*enemy, mode)
		e.applySpecialPending(enemy)
		enemy.Frame = 0
		if state.hp < 16 || state.dead {
			enemy.Frame = 3
			if mode != 1 {
				enemy.Frame = 7
			}
		} else if mode == 1 {
			child := e.specialPart(9, 9)
			if child != nil && child.special.dead && e.nativeClock()&32 == 0 {
				enemy.Frame = 1
			}
		} else {
			enemy.Frame = int(e.residentByte(0x78de + uint32(byte(e.nativeClock())>>3)))
		}
		if state.flash > 0 {
			state.flash--
			if state.flash&1 != 0 {
				enemy.Frame = 2
				if mode != 1 {
					enemy.Frame = 6
				}
			}
		}
		enemy.Definition.Sprite = fmt.Sprintf("boss_%d_body_%d", mode, enemy.Frame)
	}
	return true
}

func (e *Engine) caveBossFire(enemy Enemy, mode int) {
	if enemy.special.dead {
		return
	}
	clock := e.nativeClock()
	if mode == 1 {
		period := 48
		if e.Scroll >= 6000 {
			period = 40
		}
		remainder := ((clock | 1) % period) &^ 1
		if remainder == 8 || remainder == 14 {
			e.nextRandom()
			e.nextRandom()
		}
		if remainder == 2 || remainder == 20 || remainder == 8 || remainder == 14 {
			e.fireEnemy(enemy)
		}
		return
	}
	low := int(byte(clock))
	if low >= 80 && low < 176 && (low&15 == 0 || low&15 == 2) {
		e.nextRandom()
		e.nextRandom()
		e.fireEnemy(enemy)
	}
	if (low+8)&127 == 0 {
		e.fireEnemy(enemy)
	}
}

func (e *Engine) nativeSpecialContact(enemy Enemy) bool {
	if !enemy.Definition.FlyingPool {
		return true
	}
	switch enemy.Definition.NativeKind {
	case 2:
		body := e.specialPart(8, 2)
		if body != nil && body.special.dying != 0 {
			return false
		}
		if enemy.PoolSlot == 9 || enemy.PoolSlot == 10 {
			return enemy.special.phase < 3
		}
	case 9:
		body := e.specialPart(8, 9)
		if body != nil && body.special.dead {
			return false
		}
	case 10:
		return false
	}
	return true
}

func (e *Engine) moveCaveBossWander(enemy *Enemy) {
	state := &enemy.special
	clock := e.nativeClock()
	if !state.entered {
		if byte(clock) != 0 {
			return
		}
		state.entered = true
	}
	if clock>>8&1 != 0 {
		if byte(clock) == 0 {
			player := 0
			if clock>>8&2 != 0 {
				player = 1
			}
			if !e.Players[player].Active || e.Players[player].Respawn != 0 || e.Players[player].Dying != 0 {
				player ^= 1
			}
			enemy.VX = 0
			if e.Players[player].Active && e.Players[player].Respawn == 0 && e.Players[player].Dying == 0 {
				enemy.VX = 0x8000
				if e.Players[player].X-48 < enemy.X {
					enemy.VX = -enemy.VX
				}
			}
		}
		index := int(byte(clock) >> 1)
		sign := 1
		if index >= 64 {
			index, sign = 127-index, -1
		}
		value := int(e.residentByte(0x78fe+uint32(index))) * sign
		enemy.fixedX += enemy.VX
		enemy.fixedY += value << 13
	} else {
		if state.steerX == 0 {
			value := e.nextRandom()&0x8f | 3
			x := enemy.X + e.CameraX + 256
			if x < 276 {
				value &^= 128
			} else if x >= 492 {
				value |= 128
			}
			if value&128 != 0 {
				value |= 112
			}
			state.steerX = int(int8(value))
		}
		if state.steerX < 0 {
			enemy.fixedX -= 0xc000
			state.steerX++
		} else {
			enemy.fixedX += 0xc000
			state.steerX--
		}
		if state.steerY == 0 {
			value := e.nextRandom()&0x8f | 3
			if enemy.Y < -8 {
				value &^= 128
			} else if enemy.Y >= 40 {
				value |= 128
			}
			if value&128 != 0 {
				value |= 112
			}
			state.steerY = int(int8(value))
		}
		if state.steerY < 0 {
			enemy.fixedY -= 0xc000
			state.steerY++
		} else {
			enemy.fixedY += 0xc000
			state.steerY--
		}
	}
	enemy.X, enemy.Y = enemy.fixedX>>16, enemy.fixedY>>16
}

func (e *Engine) residentByte(address uint32) byte {
	data, err := (memory{e.Data.Loader, e.Data.LoaderBase}).at(address, 1)
	if err != nil {
		return 0
	}
	return data[0]
}

func (e *Engine) nativeSpecialTouchable(enemy Enemy) bool {
	if !enemy.Definition.FlyingPool {
		return true
	}
	switch enemy.Definition.NativeKind {
	case 2:
		if enemy.PoolSlot == 8 {
			return false
		}
		body, head := e.specialPart(8, 2), e.specialPart(11, 2)
		if body != nil && body.special.dying != 0 {
			return false
		}
		if enemy.PoolSlot == 9 || enemy.PoolSlot == 10 {
			return enemy.special.phase < 3 || head != nil && head.special.timer >= 250
		}
		if enemy.PoolSlot == 11 {
			return enemy.special.phase < 2 || enemy.special.phase >= 7 && enemy.special.timer == 0 || enemy.special.timer >= 250
		}
	case 9:
		if enemy.special.dead {
			return false
		}
	case 10:
		return false
	}
	return true
}

// damageNativeSpecial preserves the source parts' gated, signed-byte damage phases.
func (e *Engine) damageNativeSpecial(index, damage, player int) bool {
	enemy := &e.Enemies[index]
	kind := enemy.Definition.NativeKind
	if !enemy.Definition.FlyingPool || kind != 2 && kind != 9 {
		return false
	}
	if !e.nativeSpecialTouchable(*enemy) {
		return true
	}
	e.initializeSpecial(enemy)
	state := &enemy.special
	if kind == 2 {
		head, body := e.specialPart(11, 2), e.specialPart(8, 2)
		if head != nil && head.special.timer >= 250 && body != nil {
			body.special.pending = (body.special.pending + damage) & 255
		} else {
			state.pending = (state.pending + damage) & 255
		}
	} else {
		state.pending = (state.pending + damage) & 255
	}
	e.Events = append(e.Events, Event{Kind: "hit", Player: player})
	return true
}

func (e *Engine) applySpecialPending(enemy *Enemy) {
	state := &enemy.special
	if state.pending == 0 || enemy.PoolSlot == 8 && enemy.Definition.NativeKind == 2 {
		return
	}
	damage := state.pending
	state.pending = 0
	if enemy.Definition.NativeKind == 2 {
		if enemy.PoolSlot == 11 && state.phase < 2 {
			left, right := e.specialPart(9, 2), e.specialPart(10, 2)
			if left == nil || right == nil || left.special.phase < 3 || right.special.phase < 3 {
				return
			}
		}
		state.hp = int(int8(byte(state.hp) - byte(damage)))
		if state.hp < 0 {
			if enemy.PoolSlot == 9 || enemy.PoolSlot == 10 {
				state.phase = 3
			} else if state.phase < 2 {
				state.phase, state.hp = 2, 127
			} else {
				state.timer = 1
			}
			e.Events = append(e.Events, Event{Kind: "explosion", Value: 29})
		} else {
			state.flash = 6
		}
	} else {
		mode := 0
		if e.Stage < len(e.Data.Stages) {
			mode = e.Data.Stages[e.Stage].Mode
		}
		if enemy.PoolSlot == 8 {
			child := e.specialPart(9, 9)
			if (mode == 1 || state.hp >= 16) && (child == nil || !child.special.dead) {
				return
			}
			if mode != 1 && state.hp >= 16 {
				clock := int(byte(e.nativeClock()))
				if clock < 80 || clock >= 176 {
					return
				}
			}
		}
		if enemy.PoolSlot == 8 || mode == 1 {
			damage = max(1, damage>>1)
		}
		state.hp = int(int8(byte(state.hp) - byte(damage)))
		if state.hp < 0 {
			state.dead, enemy.Frame = true, 3
			e.Events = append(e.Events, Event{Kind: "explosion", Value: 29})
		} else {
			state.flash = 6
		}
	}
}

// NativeBossBlocksScroll retains the original two cave locks and final-arena stop.
func (e *Engine) NativeBossBlocksScroll() bool {
	for _, enemy := range e.Enemies {
		if enemy.Definition.NativeKind == 2 && e.Scroll >= 240 {
			return true
		}
		if enemy.Definition.NativeKind == 9 && enemy.PoolSlot == 8 && !enemy.special.dead && (e.Scroll == 3100 || e.Scroll == 8150) {
			return true
		}
	}
	return false
}

// StartFinalBattle installs the loader's separate schedule when the surface wraps
// after all three caves. Existing objects remain until the original clear event.
func (e *Engine) StartFinalBattle() error {
	stream, err := decodeFinalSchedule(e.Data.Loader, e.Data.LoaderBase)
	if err != nil {
		return err
	}
	if len(e.Data.Stages) == 0 {
		return fmt.Errorf("the original surface is missing")
	}
	if e.initialSurfaceEvents == nil {
		e.initialSurfaceEvents = e.Data.Stages[0].Events
	}
	e.Data.Stages[0].Events = stream
	e.Stage, e.Scroll, e.nextEvent, e.modeFrames = 0, 0, 0, 0
	e.finalBattle = true
	return nil
}

// FinalBattle identifies the arena whose original art uses the LODFIN bank.
func (e *Engine) FinalBattle() bool { return e.finalBattle }

func decodeFinalSchedule(loader []byte, base uint32) ([]Spawn, error) {
	reader := memory{loader, base}
	definitions, err := nativeFlyingTable(loader, base)
	if err != nil {
		return nil, err
	}
	for offset := 0; offset+8 < len(loader); offset += 2 {
		instruction := loader[offset:]
		if instruction[0] != 0x2b || instruction[1] != 0x7c || instruction[6] != 0xf4 || instruction[7] != 0xb0 {
			continue
		}
		pointer := binary.BigEndian.Uint32(instruction[2:])
		data, err := reader.at(pointer, 62)
		if err != nil || binary.BigEndian.Uint16(data) != 198 || signedWord(data[2:]) != -1 || binary.BigEndian.Uint16(data[60:]) != 65535 {
			continue
		}
		spawns := []Spawn{{Progress: 198, ClearObjects: true}}
		valid := true
		for part := 0; part < 4; part++ {
			row := data[(part+1)*12:]
			if binary.BigEndian.Uint16(row) != 200 || row[6] != 2 {
				valid = false
				break
			}
			definition, err := decodeFlyingDefinition(loader, base, definitions+64, 2)
			if err != nil {
				return nil, err
			}
			spawns = append(spawns, Spawn{Progress: 200, X: signedWord(row[2:]), Y: signedWord(row[4:]), Definition: definition})
		}
		if valid {
			return spawns, nil
		}
	}
	return nil, fmt.Errorf("the original five-record final-battle surface stream was not found")
}
