package engine

import (
	"encoding/binary"
	"fmt"
)

// GroundTemplate names the original scenery record's immutable control bytes.
// Mutable mailboxes and countdowns live in nativeGroundState instead of RAM.
type GroundTemplate struct {
	Kind, Parameter, LiveLimit, InitialFrame                   byte
	Armor, SecondaryArmor, FinalFrame, FlashFrame, NormalFrame byte
	Flags, Flash, CounterA, CounterB, FireMask                 byte
	CollisionOffset, CollisionWidth                            int
}

// DecodeGroundTemplate recovers one verified forty-eight-byte scenery header.
func DecodeGroundTemplate(loader []byte, base, address uint32) (GroundTemplate, error) {
	record, err := (memory{loader, base}).at(address, 48)
	if err != nil {
		return GroundTemplate{}, err
	}
	if _, err := DecodeDefinition(loader, base, address); err != nil {
		return GroundTemplate{}, err
	}
	return GroundTemplate{Kind: record[17], Parameter: record[18], LiveLimit: record[19], InitialFrame: record[25], Armor: record[28], SecondaryArmor: record[32], FinalFrame: record[33], FlashFrame: record[34], NormalFrame: record[35], Flags: record[31], Flash: record[30], CounterA: record[36], CounterB: record[37], FireMask: record[43], CollisionOffset: int(int16(binary.BigEndian.Uint16(record[20:]))), CollisionWidth: int(int16(binary.BigEndian.Uint16(record[22:])))}, nil
}

type nativeGroundState struct {
	initialized                                                                          bool
	template                                                                             GroundTemplate
	frame, flags, armor, secondary, param, flash, counterA, counterB, fire, damage, burn byte
	trapReset                                                                            byte
	owner                                                                                int
	scored, hidden                                                                       bool
	scroll                                                                               int
}

func (e *Engine) initializeGround(enemy *Enemy) bool {
	state := &enemy.ground
	if state.initialized {
		return true
	}
	if e.Data == nil || !enemy.Definition.Ground || enemy.Definition.FlyingPool || enemy.Definition.Address == 0 {
		return false
	}
	template, err := DecodeGroundTemplate(e.Data.Loader, e.Data.LoaderBase, enemy.Definition.Address)
	if err != nil {
		return false
	}
	state.initialized = true
	state.template = template
	state.frame = byte(enemy.Frame)
	state.flags = template.Flags
	state.armor = byte(enemy.Health)
	state.secondary = template.SecondaryArmor
	state.param = template.Parameter
	state.flash = template.Flash
	state.counterA = template.CounterA
	state.counterB = template.CounterB
	state.fire = (e.nextRandom() & template.FireMask) + 1
	state.owner = -1
	// The captured original game starts on the medium preset: trap100,
	// repeating burst70 and fan/beam50. Low/high presets are75/125 for traps.
	state.trapReset = 100
	state.scroll = e.Scroll - 1
	// The original allocator reduces armour by one quarter for a single ship.
	if !(e.Players[0].Active && e.Players[1].Active) {
		state.armor -= byte((int(state.armor) + 1) >> 2)
	}
	enemy.Health = int(state.armor)
	return true
}

// moveNativeGround replaces the original ground-pool controller and its
// common draw tail. The caller runs this once per original NPC/render update.
func (e *Engine) moveNativeGround(enemy *Enemy) bool {
	if enemy.Definition.FlyingPool && enemy.Definition.NativeKind == 7 {
		return e.moveNativeGroundBeam(enemy)
	}
	if !e.initializeGround(enemy) {
		return false
	}
	state := &enemy.ground
	state.hidden = false
	step := e.Scroll - state.scroll
	if step < 0 || step > 1 {
		step = 1
	}
	state.scroll = e.Scroll
	enemy.Y += step
	enemy.fixedY = enemy.Y << 16
	state.flags ^= 8
	if enemy.Y >= 208 {
		if enemy.Y >= 208 {
			enemy.Health = -1
		}
		state.hidden = true
		return true
	}
	if enemy.Y+enemy.Definition.Height <= 0 {
		state.hidden = true
	}
	mode := e.groundMode()
	kind := state.template.Kind
	clock := e.nativeClock()
	if kind >= 32 && e.groundCommon(enemy, mode, clock) {
		enemy.Frame = int(state.frame)
		return true
	}
	switch kind {
	case 0:
		e.groundBunker(enemy, clock)
	case 1:
		if clock&2 != 0 {
			if state.frame == 0 {
				if enemy.Y >= 0 {
					state.fire--
					if state.fire == 0 {
						state.frame = 1
					}
				}
			} else if state.frame < 9 {
				state.frame++
			} else if state.counterA == 0 {
				state.counterA = 255
				e.queueGroundFlying(7, enemy.X+14, enemy.Y+31, 0)
			}
		}
	case 2:
		e.groundDualLauncher(enemy, clock)
	case 3:
		e.groundHeavyLauncher(enemy, clock)
	case 4:
		e.groundFanBattery(enemy, clock)
	case 5:
		e.groundCaveCells(enemy, clock)
	case 32:
		e.groundRotatingTurret(enemy, mode)
	case 33:
	case 34:
		state.fire--
		if state.fire == 0 {
			state.fire = 8
			state.frame ^= 1
		}
	case 37, 38:
		if int8(state.counterA) >= 0 {
			if state.counterA == 0 {
				r := e.nextRandom()
				if r&64 != 0 {
					state.counterA = 255
					break
				}
				state.counterA = r&63 + 64
			}
			state.counterA--
			if state.counterA == 0 {
				state.counterA = 255
				x, y := 2, 7
				if kind == 38 {
					x, y = 20, 9
				}
				e.fireNativeAimed(enemy, x, y)
			}
		}
	case 39:
		state.flags |= 4
		state.frame = 0
		if byte(clock)&31 >= 8 {
			state.frame = 1
		}
	case 40:
		if state.param == 12 {
			e.fireNativeAimed(enemy, 4, 4)
		}
		if state.param == 0 {
			state.param = state.trapReset
		}
		state.param--
		if int8(state.param) < 24 {
			state.frame = e.groundTableByte(0x5e70 + uint32(state.param))
		} else {
			if clock&2 == 0 {
				state.frame++
			}
			state.frame &= 7
		}
	case 41:
		state.flags |= 4
		state.hidden = true
		if enemy.Y >= 158 {
			enemy.Health = -1
			break
		}
		if int8(state.counterA) < 0 {
			break
		}
		if state.counterA == 0 {
			r := e.nextRandom()
			if int8(r) >= 0 {
				r = r&63 + 64
			}
			state.counterA = r
		}
		state.counterA--
		if state.counterA == 0 {
			state.counterA = 255
			e.queueGroundFlying(8, enemy.X+6, enemy.Y-6, 0)
		} else if state.counterA == 20 {
			e.groundSound(24)
		}
	}
	enemy.Frame = int(state.frame)
	return true
}

func (e *Engine) groundMode() int {
	if e.Data != nil && e.Stage < len(e.Data.Stages) {
		return e.Data.Stages[e.Stage].Mode
	}
	return 0
}
func (e *Engine) groundTableByte(address uint32) byte { return e.residentByte(address) }
func (e *Engine) groundSound(id int) {
	e.Events = append(e.Events, Event{Kind: "original-sfx", Player: -1, Value: id})
}

// groundCommon preserves delayed damage, hit flashes, death frames and wrecks.
func (e *Engine) groundCommon(enemy *Enemy, mode, clock int) bool {
	s := &enemy.ground
	t := s.template
	if int8(s.frame) >= int8(t.LiveLimit) {
		if t.Kind == 32 && s.frame > t.FinalFrame {
			s.hidden = true
		}
		if int8(s.frame) < int8(t.FinalFrame) {
			if clock&2 == 0 {
				s.frame++
			}
		} else if t.Kind == 32 && s.frame == t.FinalFrame {
			e.claimGroundWreck(enemy, mode)
		}
		return true
	}
	if s.damage != 0 {
		damage := s.damage
		s.damage = 0
		if t.Kind == 34 {
			s.flash++
		} else if t.Kind == 32 && mode == 2 {
			if s.flash == 0 {
				s.flash = 15
			}
		} else {
			s.flash = 3
		}
		if e.consumeGroundArmor(enemy, damage, false) {
			s.frame = t.LiveLimit
			s.flags |= 4
			return true
		}
		e.renderGroundFlash(enemy, mode)
		return true
	}
	if s.flash != 0 {
		s.flash--
		e.renderGroundFlash(enemy, mode)
		return true
	}
	return false
}

func (e *Engine) renderGroundFlash(enemy *Enemy, mode int) {
	s := &enemy.ground
	if s.template.Kind == 34 && mode == 0 {
		s.frame = 0
		if s.flash != 0 {
			s.frame = 6 - s.flash
		}
		return
	}
	if s.template.Kind == 32 && mode == 2 {
		s.frame = e.groundTableByte(0x5e8a + uint32(s.flash))
		if s.flash == 6 {
			e.fireNativeAimed(enemy, 33, 14)
		} else if s.flash == 5 {
			e.fireNativeAimed(enemy, 6, 14)
		}
		return
	}
	s.frame = s.template.FlashFrame
	if s.flash&1 == 0 {
		s.frame = s.template.NormalFrame
	}
}

func (e *Engine) consumeGroundArmor(enemy *Enemy, damage byte, secondary bool) bool {
	s := &enemy.ground
	armor := &s.armor
	if secondary {
		armor = &s.secondary
	}
	*armor -= damage
	dead := int8(*armor) < 0
	if !secondary {
		enemy.Health = max(0, int(int8(*armor)))
	}
	if dead && !s.scored && s.owner >= 0 && s.owner < 2 {
		s.scored = true
		e.Players[s.owner].Score += enemy.Definition.Score
		e.Events = append(e.Events, Event{Kind: "building-destroyed", Player: s.owner, Value: int(s.template.Kind)})
		e.Events = append(e.Events, Event{Kind: "explosion", Player: s.owner, Value: int(s.template.Kind)})
	}
	return dead
}

func (e *Engine) claimGroundWreck(enemy *Enemy, mode int) {
	x, y := enemy.X, enemy.Y
	if mode == 3 {
		x -= 8
	}
	claimed := false
	for index := range e.Players {
		p := &e.Players[index]
		if !p.Active || p.Lives <= 0 || p.Dying > 0 || p.Respawn > 0 || p.WreckBonus >= 99 {
			continue
		}
		if p.X >= x-8 && p.X <= x+20 && p.Y >= y-12 && p.Y <= y+16 {
			p.WreckBonus++
			claimed = true
			e.Events = append(e.Events, Event{Kind: "wreck-bonus", Player: index, Value: p.WreckBonus})
		}
	}
	if claimed {
		enemy.ground.frame = enemy.ground.template.FinalFrame + 1
		enemy.ground.hidden = true
		e.groundSound(53)
	}
}

// nativeGroundTouchable checks the original invulnerable/death flag before a hit.
func (e *Engine) nativeGroundTouchable(enemy Enemy) bool {
	if enemy.Definition.FlyingPool {
		return true
	}
	if enemy.Definition.Kind == 1 || enemy.Definition.Kind == 39 || enemy.Definition.Kind == 41 {
		return false
	}
	if !enemy.ground.initialized {
		return enemy.Definition.Kind != 1 && enemy.Definition.Kind != 39 && enemy.Definition.Kind != 41
	}
	return enemy.ground.flags&4 == 0 && int8(enemy.ground.armor) >= 0
}

// damageNativeGround stages byte-sized collision damage for the next NPC pass.
func (e *Engine) damageNativeGround(index, damage, player int) bool {
	enemy := &e.Enemies[index]
	if enemy.Definition.FlyingPool && enemy.Definition.NativeKind == 7 {
		enemy.ground.damage += byte(damage)
		enemy.ground.owner = player
		return true
	}
	if !e.initializeGround(enemy) {
		return false
	}
	if !e.nativeGroundTouchable(*enemy) {
		return true
	}
	enemy.ground.damage += byte(damage)
	enemy.ground.owner = player
	return true
}

// GroundVisible lets presentation honor original branches that skip draw emission.
func (e *Engine) GroundVisible(enemy Enemy) bool { return !enemy.ground.hidden }

func (e *Engine) queueGroundFlying(kind byte, x, y int, parameter uint32) bool {
	table, err := nativeFlyingTable(e.Data.Loader, e.Data.LoaderBase)
	if err != nil {
		return false
	}
	definition, err := decodeFlyingDefinition(e.Data.Loader, e.Data.LoaderBase, table+uint32(kind)*32, kind)
	if err != nil {
		return false
	}
	definition.Collectable = false
	// The shared flying allocator applies its player-count armor rule once.
	spawn := Spawn{X: x, Y: y, Definition: definition}
	if kind == 13 {
		if parameter != 0 {
			spawn.Definition.TrackingFrames = 1
		}
		spawn.VX = int(int16(parameter >> 16))
	}
	e.groundSpawns = append(e.groundSpawns, spawn)
	return true
}

// takeNativeGroundSpawns drains allocations after the stable ground-pool scan.
func (e *Engine) takeNativeGroundSpawns() []Spawn {
	spawns := e.groundSpawns
	e.groundSpawns = nil
	return spawns
}

func (e *Engine) groundBunker(enemy *Enemy, clock int) {
	s := &enemy.ground
	if s.flags&2 != 0 {
		if s.flags&16 != 0 {
			s.damage = 0
			if s.flags&8 == 0 {
				s.frame--
				if int8(s.frame) <= 3 {
					s.frame = 0
					s.flags &^= 2
				}
			}
			return
		}
		if s.param == 0 {
			s.damage = 0
			if s.flags&8 == 0 {
				s.frame++
				if int8(s.frame) >= 7 {
					s.param = 100
				}
			}
			return
		}
		if s.frame >= 11 {
			if s.flags&8 == 0 && s.frame < 15 {
				s.frame++
			}
			return
		}
		if s.flash != 0 {
			e.bunkerFlash(enemy)
			return
		}
		if s.damage != 0 {
			damage := s.damage
			s.damage = 0
			if e.consumeGroundArmor(enemy, damage, true) {
				s.frame = 10
				s.flags |= 4
				s.flags &^= 8
				return
			}
			s.flash = 4
			e.bunkerFlash(enemy)
			return
		}
		v := s.param
		if int8(v) > 50 {
			v -= 50
		}
		s.frame = 7
		if int8(v) >= 20 && int8(v) < 30 {
			s.frame++
			switch s.param {
			case 25:
				e.fireNativeAimed(enemy, 26, 30)
			case 73:
				e.fireNativeAimed(enemy, 30, 16)
			case 77:
				e.fireNativeAimed(enemy, 15, 17)
			}
		}
		s.param--
		if s.param == 0 {
			s.flags |= 16
			s.flags &^= 8
		}
		return
	}
	if s.burn != 0 {
		if s.burn != 1 {
			s.burn--
			if s.frame == 10 {
				s.frame = 1
			} else {
				s.frame = 10
			}
		}
		return
	}
	if s.flags&16 == 0 && enemy.Y >= 0 {
		s.fire--
		if s.fire == 0 {
			s.flags |= 2
			s.flags &^= 8
			s.frame = 4
			e.groundSound(25)
			return
		}
	}
	if s.frame == 0 {
		if s.damage == 0 {
			return
		}
		damage := s.damage
		s.damage = 0
		if e.consumeGroundArmor(enemy, damage, false) {
			s.burn = 16
			s.flags |= 4
		}
	}
	s.frame++
	if s.frame >= 4 {
		s.frame = 0
	}
}
func (e *Engine) bunkerFlash(enemy *Enemy) {
	s := &enemy.ground
	s.frame = 9
	s.flash--
	if s.flash&2 == 0 {
		s.frame = 8
	}
}

func (e *Engine) groundDualLauncher(enemy *Enemy, clock int) {
	s := &enemy.ground
	s.flags |= 4
	if enemy.Y < 0 {
		s.hidden = true
		return
	}
	if s.fire != 0 {
		s.fire--
		if s.fire == 0 {
			e.groundSound(30)
		}
		return
	}
	if s.counterA < 23 {
		s.counterA++
		s.frame = s.counterA >> 3
		return
	}
	if s.frame >= 6 {
		if s.frame < 11 && clock&6 == 0 {
			s.frame++
		}
		return
	}
	s.flags &^= 4
	if s.counterB == 0 {
		s.counterB = 70
	}
	s.counterB--
	s.frame = 2
	if int8(s.counterB) < 24 {
		v := s.counterB >> 3
		if v == 2 {
			v = 0
		}
		s.frame = v + 3
		if s.counterB == 9 || s.counterB == 14 {
			e.fireNativeAimed(enemy, 28, 4)
		}
	}
	if s.damage != 0 {
		damage := s.damage
		s.damage = 0
		if e.consumeGroundArmor(enemy, damage, false) {
			s.flags |= 4
			s.frame = 6
			return
		}
		s.flash = 6
	}
	if s.flash != 0 {
		s.flash--
		s.frame = 5
		if s.flash&1 == 0 {
			s.frame = 2
		}
	}
}

func (e *Engine) groundHeavyLauncher(enemy *Enemy, clock int) {
	s := &enemy.ground
	s.flags |= 4
	if enemy.Y < int(s.fire)-16 {
		s.hidden = true
		return
	}
	if enemy.Y == int(s.fire)-16 {
		e.groundSound(30)
	}
	if s.frame < 5 {
		s.counterA = 16
		if clock&14 == 0 {
			s.frame++
		}
		return
	}
	s.flags &^= 4
	if s.frame >= 13 {
		s.flags |= 4
		if s.frame < 20 && clock&6 == 0 {
			s.frame++
		}
		return
	}
	if s.damage != 0 {
		damage := s.damage
		s.damage = 0
		if e.consumeGroundArmor(enemy, damage, false) {
			s.frame = 13
			return
		}
		if s.frame < 8 {
			s.frame = 8
			return
		}
	}
	if s.frame >= 8 {
		if clock&2 != 0 {
			s.frame++
			if s.frame >= 13 {
				s.frame = 7
				s.counterA = 0
			}
		}
		return
	}
	if s.counterA == 0 {
		s.frame = 7
		if e.nextRandom()&31 != 0 {
			return
		}
		s.counterA = 24
	}
	s.counterA--
	s.frame = 5
	if s.counterA >= 8 && s.counterA < 16 {
		s.frame = 6
		if s.counterA == 13 {
			e.fireNativeAimed(enemy, 4, 8)
		} else if s.counterA == 12 {
			e.fireNativeAimed(enemy, 35, 16)
		}
	}
}

func (e *Engine) groundFanBattery(enemy *Enemy, clock int) {
	s := &enemy.ground
	if s.frame < 12 {
		if s.frame < 3 {
			if clock&6 == 0 {
				s.frame++
			}
			return
		}
		if s.counterA == 0 {
			s.counterA = 50
		}
		s.counterA--
		s.frame = 0
		if s.counterA < 25 {
			s.frame = 1
			dx := 0
			switch s.counterA {
			case 20:
				dx = -50
			case 15:
				dx = -25
			case 10:
				dx = 15
			case 5:
				dx = 50
			}
			if dx != 0 {
				e.fireNativeVector(enemy.X+28, enemy.Y+11, dx, 50)
			}
		}
		if s.damage != 0 {
			damage := s.damage
			s.damage = 0
			if e.consumeGroundArmor(enemy, damage, false) {
				s.frame = 3
				s.flags |= 4
				return
			}
			s.flash = 6
		}
		if s.flash != 0 {
			s.flash--
			if s.flash&1 != 0 {
				s.frame = 2
			}
		}
		return
	}
	if s.frame >= 15 {
		if s.frame < 17 && clock&14 == 0 {
			s.frame++
		}
		return
	}
	s.flags &^= 4
	s.counterA--
	if s.counterA == 0 {
		s.counterA = 50
		e.fireNativeAimed(enemy, 28, 14)
	}
	if s.damage != 0 {
		damage := s.damage
		s.damage = 0
		if e.consumeGroundArmor(enemy, damage, true) {
			s.frame = 15
			s.flags |= 4
			return
		}
		s.flash = 6
	}
	s.frame = 12
	if byte(clock)&31 < 24 {
		s.frame++
	}
	if s.flash != 0 {
		s.flash--
		if s.flash&1 != 0 {
			s.frame = 14
		}
	}
}

func (e *Engine) groundCaveCells(enemy *Enemy, clock int) {
	s := &enemy.ground
	if s.frame > 5 {
		if int8(s.armor) < 0 {
			return
		}
		if s.damage == 0 {
			s.hidden = true
			return
		}
		damage := s.damage
		s.damage = 0
		if e.consumeGroundArmor(enemy, damage, false) {
			s.flags |= 4
		}
		e.groundImpact(enemy)
		return
	}
	if s.frame == 5 {
		if s.counterA == 0 {
			return
		}
		s.counterA--
		if e.nextRandom()&6 != 0 {
			return
		}
		e.groundImpactAt(enemy.X-64+int(e.nextRandom()&127), enemy.Y-32+int(e.nextRandom()&63))
		return
	}
	wave := byte(byte(clock) << 1)
	if int8(wave) < 0 {
		wave = byte(-int8(wave))
	}
	s.frame = 3
	s.flags &^= 4
	if wave < 24 {
		s.frame = wave >> 3
		s.flags |= 4
	} else if enemy.Y >= 8 && enemy.Y < 168 && (wave+4)&7 == 0 {
		e.fireNativeVector(enemy.X+12, enemy.Y+20, int(int8(e.nextRandom())), int(int8(e.nextRandom())))
	}
	living := 0
	for _, other := range e.Enemies {
		if !other.Definition.Ground || other.Definition.Kind != 5 || other.Y <= enemy.Y-64 || other.Y > enemy.Y+64 {
			continue
		}
		armor := byte(other.Health)
		if other.ground.initialized {
			armor = other.ground.armor
		}
		if int8(armor) >= 0 {
			living++
		}
	}
	if living != 1 {
		s.flags |= 4
		return
	}
	if s.damage != 0 {
		damage := s.damage
		s.damage = 0
		if e.consumeGroundArmor(enemy, damage, false) {
			s.flags |= 4
			s.frame = 5
			s.counterA = 75
			return
		}
		s.flash = 4
		e.groundImpact(enemy)
	}
	if s.flash != 0 {
		s.flash--
		if s.flash&1 != 0 {
			s.frame = 4
		}
	}
}

func (e *Engine) groundImpact(enemy *Enemy) {
	e.groundImpactAt(enemy.X-16+int(e.nextRandom()&31), enemy.Y-16+int(e.nextRandom()&31))
}
func (e *Engine) groundImpactAt(x, y int) {
	// The original temporary impact list has two eight-count entries. These
	// use the recovered explosion pixels; their state never removes scenery.
	e.Explosions = append(e.Explosions, Explosion{X: x, Y: y, Duration: 32})
	_ = e.nextRandom() & 15
	e.groundSound(31)
}

func (e *Engine) groundRotatingTurret(enemy *Enemy, mode int) {
	s := &enemy.ground
	switch mode {
	case 1:
		s.counterA++
		if int8(s.counterA) >= 70 {
			s.counterA = 0
		}
		value := s.counterA
		if int8(value) >= 35 {
			value = 70 - value
		}
		s.frame = 0
		if value >= 20 {
			s.frame++
		}
		if value >= 25 {
			s.frame++
		}
		if value == 35 {
			e.fireNativeAimed(enemy, 20, 18)
		}
	case 2:
		if s.counterA == 0 {
			s.counterA = e.nextRandom()&63 | 23
		}
		s.counterA--
		if s.counterA != 0 && s.counterA != 32 {
			return
		}
		opposite := s.counterA == 0
		s.counterA = 0
		queued := e.queueGroundFlying(13, enemy.X+16, enemy.Y+93, 0x10000)
		if opposite && queued {
			e.groundSpawns[len(e.groundSpawns)-1].VX = -1
		}
	case 3:
		// The original mode-three type-$20 has no live behaviour before rendering.
	default:
		if s.counterB >= 2 {
			s.counterB--
		}
		if s.counterA != 0 {
			s.counterA--
			return
		}
		target := -1
		distance := int(^uint(0) >> 1)
		for index, p := range e.Players {
			if !p.Active || p.Lives <= 0 || p.Dying > 0 {
				continue
			}
			candidate := abs(p.X+12-enemy.X-19) + abs(p.Y-enemy.Y)
			if candidate <= distance {
				target, distance = index, candidate
			}
		}
		if target < 0 {
			return
		}
		p := e.Players[target]
		dx := int(int16(p.X - 6 - enemy.X))
		if dx == 0 {
			dx = 1
		}
		direction := 0
		if dx < 0 {
			direction = 4
		}
		dy := int(int16(enemy.Y + 6 - p.Y))
		quotient := int(int16(dy * 64 / (dx | 1)))
		angle := 0
		if quotient < 154 {
			angle = 1
		}
		if quotient < 26 {
			angle = 2
		}
		if quotient < -26 {
			angle = 3
		}
		if quotient < -154 {
			angle = 4
		}
		angle = (angle + direction) & 7
		delta := int(int8(e.groundTableByte(uint32(int(0x5e68) + angle - int(s.frame)))))
		if delta != 0 {
			s.frame = byte((int(s.frame) + delta) & 7)
			s.template.NormalFrame = s.frame
			s.counterA = 10
			if s.counterB < 2 {
				s.counterB = 1
			}
			return
		}
		if s.counterB >= 2 {
			return
		}
		if s.counterB != 0 {
			s.counterB--
			return
		}
		s.counterB = byte(e.Options.EnemyFireDelay)
		offsetsX := []int{19, 26, 30, 27, 20, 13, 9, 14}
		offsetsY := []int{6, 10, 15, 23, 28, 23, 15, 10}
		frame := int(s.frame) & 7
		e.fireNativeAimed(enemy, offsetsX[frame], offsetsY[frame])
	}
}

func (e *Engine) moveNativeGroundBeam(enemy *Enemy) bool {
	s := &enemy.ground
	if !s.initialized {
		s.initialized = true
		s.armor = byte(enemy.Health)
		s.frame = byte(enemy.Frame)
		s.scroll = e.Scroll - 1
		s.owner = -1
		enemy.Definition.Height = 0
	}
	step := e.Scroll - s.scroll
	if step < 0 || step > 1 {
		step = 1
	}
	s.scroll = e.Scroll
	enemy.Y += step
	enemy.fixedY = enemy.Y << 16
	if enemy.Y >= 208 {
		enemy.Health = -1
		return true
	}
	clock := e.nativeClock()
	if s.frame >= 3 {
		if clock&2 == 0 {
			s.frame++
			if s.frame >= 11 {
				enemy.Health = -1
				return true
			}
		}
		enemy.Frame = int(s.frame)
		enemy.Definition.Sprite = fmt.Sprintf("flying_10_%d_stage_%d", int(s.frame)-3, e.Stage)
		return true
	}
	if s.counterA < 32 {
		s.counterA++
	}
	enemy.Definition.Height = int(s.counterA)
	s.frame = 0
	if e.Data != nil && e.Stage < len(e.Data.Stages) && len(e.Data.Stages[e.Stage].TileBank) != 0 {
		if !e.groundTerrainSolid(enemy.X+e.CameraX+2, enemy.Y-1) {
			enemy.Y--
			enemy.fixedY = enemy.Y << 16
			if clock&2 == 0 {
				s.frame++
			}
		}
	}
	if s.damage != 0 {
		damage := s.damage
		s.damage = 0
		s.flash = 4
		if e.consumeGroundArmor(enemy, damage, false) {
			s.frame = 3
			enemy.Frame = 3
			enemy.Definition.Sprite = fmt.Sprintf("flying_10_0_stage_%d", e.Stage)
			return true
		}
	}
	if s.flash != 0 {
		s.flash--
		if s.flash&1 != 0 {
			s.frame = 2
		}
	}
	if s.counterA >= 20 {
		if s.counterB == 0 {
			s.counterB = 50
			e.fireNativeAimed(enemy, 12, 10)
		}
		s.counterB--
	}
	enemy.Frame = int(s.frame)
	enemy.Definition.Sprite = fmt.Sprintf("flying_7_%d_stage_%d", s.frame, e.Stage)
	return true
}

// groundTerrainSolid tests the same five individual bitmap bytes as LAB_885E.
// A plane that is all zero or all one does not stop the growing beam.
func (e *Engine) groundTerrainSolid(worldX, viewY int) bool {
	stage := e.Data.Stages[e.Stage]
	sourceY := stage.Height - e.Scroll + viewY
	if worldX < 0 || worldX >= stage.Width*16 || sourceY < 0 || sourceY >= stage.Height {
		return false
	}
	row, column := sourceY/16, worldX/16
	index := row*stage.Width + column
	if index < 0 || index >= len(stage.Tiles) {
		return false
	}
	word := stage.Tiles[index]
	for plane := 0; plane < 5; plane++ {
		off := int(word)*2 + plane*32 + (sourceY%16)*2 + (worldX%16)/8
		if off < 0 || off >= len(stage.TileBank) {
			continue
		}
		value := stage.TileBank[off]
		if value != 0 && value != 255 {
			return true
		}
	}
	return false
}

// GroundCollisionBounds reproduces the common $6EFE-$6F1C draw-tail hit box:
// x is advanced by record+20 and then record+22 supplies its collision width.
// The original y bounds use the sprite's full height from the ground header.
func (e *Engine) GroundCollisionBounds(enemy Enemy) (x, y, width, height int) {
	if enemy.Definition.Ground && !enemy.Definition.FlyingPool && enemy.ground.initialized {
		return enemy.X + enemy.ground.template.CollisionOffset, enemy.Y, enemy.ground.template.CollisionWidth, enemy.Definition.Height
	}
	return enemy.X, enemy.Y, enemy.Definition.Width, enemy.Definition.Height
}
