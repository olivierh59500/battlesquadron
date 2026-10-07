// Package engine implements Battle Squadron's game state without graphics APIs.
// Numeric rules are decoded from the local original loader wherever available.
package engine

import (
	"encoding/binary"
	"fmt"
)

const (
	FPS             = 50
	PlayerShotLimit = 12
	EnemyLimit      = 18
	EnemyShotLimit  = 12
)

// Input contains one player's independent logical joystick and fire buttons.
type Input struct {
	X, Y       int
	Fire, Nova bool
}

// Options contains the original difficulty knobs and a verification-only guard.
type Options struct {
	Lives                int
	StartWeapon          int
	EnemyProjectileCap   int
	EnemyProjectileSpeed int
	EnemyFireDelay       int
	Invulnerable         bool
}

// DefaultOptions selects three ships and the original twelve-projectile pool.
func DefaultOptions() Options {
	return Options{Lives: 3, EnemyProjectileCap: 12, EnemyProjectileSpeed: 2, EnemyFireDelay: 50}
}

// Shot describes one original twelve-byte primary-projectile template.
type Shot struct {
	X, Y, VX, VY int
	Height       int
	Graphic      uint8
	Delay        int
	Damage       int
}

// Weapon preserves an original weapon-level header and its projectile records.
type Weapon struct {
	Address                 uint32
	Cooldown, Width, Height int
	BankSize, Shift         int
	Slots                   [12]int8
	Shots                   []Shot
}

// Definition describes an original scenery or flying-object record.
type Definition struct {
	Address                          uint32
	GraphicAddress                   uint32
	Kind, Graphic                    uint8
	Frame                            int
	Frames, PlaneStride, FrameStride int
	Width, Height, Health            int
	Score, FireDelay                 int
	Sprite                           string
	Ground, Boss, Collectable        bool
	Weapon, Nova                     int
	NativeKind                       int
	FlyingPool                       bool
	TrackingFrames                   int
}

// Spawn is an original scheduled object, measured in stage scroll pixels.
type Spawn struct {
	Progress     int
	X, Y         int
	VX, VY       int
	Definition   Definition
	Script       []Motion
	ClearObjects bool
	RandomMode   uint8
	ScriptLoop   int
	RepeatScript bool
}

// Motion is one decoded movement-script command, with 16.16 fixed velocities.
type Motion struct {
	Duration, VX, VY int
	Frame            int
}

// Stage carries original map words and any decoded timed-object schedule.
// Tile storage follows disk order: the first row is the top of the level.
type Stage struct {
	ID, Mode, Height, Width int
	Tiles                   []uint16
	Events                  []Spawn
	Boss                    *Spawn
	Next                    int
}

// MapRule associates an original terrain word with its scenery-object template.
type MapRule struct {
	Mode                     int
	Tile                     uint16
	Definition               Definition
	MinProgress, MaxProgress int
	FollowingTile            uint16
	RandomBelow              int
}

// Data contains only decoded originals; the engine does not load image files.
type Data struct {
	Nova        *NovaConfig
	Loader      []byte
	LoaderBase  uint32
	WeaponTable uint32
	Weapons     [4][6]Weapon
	Stages      []Stage
	MapRules    []MapRule
	Random      []byte
	Options     Options
}

// DecodeWeapons finds and validates all twenty-four weapon-level definitions.
// The table is discovered structurally because cracked loaders move addresses.
func DecodeWeapons(loader []byte, base, table uint32) ([4][6]Weapon, uint32, error) {
	var empty [4][6]Weapon
	if table != 0 {
		weapons, err := readWeaponTable(loader, base, table)
		return weapons, table, err
	}
	for offset := 0; offset+4*24 <= len(loader); offset += 2 {
		pointer := binary.BigEndian.Uint32(loader[offset:])
		if pointer < base || uint64(pointer-base)+24 > uint64(len(loader)) {
			continue
		}
		if binary.BigEndian.Uint16(loader[pointer-base+2:]) != 0 {
			continue
		}
		address := base + uint32(offset)
		if weapons, err := readWeaponTable(loader, base, address); err == nil {
			return weapons, address, nil
		}
	}
	return empty, 0, fmt.Errorf("the original twenty-four-entry weapon table was not found")
}

func readWeaponTable(loader []byte, base, table uint32) ([4][6]Weapon, error) {
	var result [4][6]Weapon
	reader := memory{loader, base}
	pointers, err := reader.at(table, 24*4)
	if err != nil {
		return result, err
	}
	for index := 0; index < 24; index++ {
		address := binary.BigEndian.Uint32(pointers[index*4:])
		header, err := reader.at(address, 24)
		if err != nil {
			return result, err
		}
		weaponID := int(binary.BigEndian.Uint16(header[2:]))
		if weaponID != index/6 {
			return result, fmt.Errorf("weapon template at $%x belongs to %d, expected %d", address, weaponID, index/6)
		}
		weapon := Weapon{
			Address: address, Cooldown: int(binary.BigEndian.Uint16(header)),
			BankSize: int(binary.BigEndian.Uint16(header[4:])), Shift: int(binary.BigEndian.Uint16(header[6:])),
			Width: int(binary.BigEndian.Uint16(header[20:])), Height: int(binary.BigEndian.Uint16(header[22:])),
		}
		if weapon.Cooldown < 1 || weapon.Cooldown > 32 || weapon.Width < 1 || weapon.Width > 64 || weapon.Height < 1 || weapon.Height > 64 || weapon.BankSize > 12 || weapon.Shift > 12 {
			return result, fmt.Errorf("invalid original weapon header at $%x", address)
		}
		count := 0
		for slot := range weapon.Slots {
			weapon.Slots[slot] = int8(header[8+slot])
			if weapon.Slots[slot] < -1 || weapon.Slots[slot] > 1 {
				return result, fmt.Errorf("invalid projectile slot at $%x", address)
			}
			if weapon.Slots[slot] == 0 {
				count++
			}
		}
		if count == 0 || count > 12 {
			return result, fmt.Errorf("weapon at $%x has no projectile records", address)
		}
		records, err := reader.at(address+24, count*12)
		if err != nil {
			return result, err
		}
		for shotIndex := 0; shotIndex < count; shotIndex++ {
			record := records[shotIndex*12:]
			shot := Shot{
				X: signedWord(record), Y: signedWord(record[2:]), VX: signedWord(record[4:]), VY: signedWord(record[6:]),
				Height: int(record[8]), Graphic: record[9], Delay: int(record[10]), Damage: int(int8(record[11])),
			}
			if shot.Height == 0 || shot.Delay == 0 || shot.Graphic < 0x40 || shot.Graphic > 0x57 || shot.VX < -16 || shot.VX > 16 || shot.VY < -16 || shot.VY > 16 {
				return result, fmt.Errorf("invalid projectile template at $%x", address+24+uint32(shotIndex*12))
			}
			weapon.Shots = append(weapon.Shots, shot)
		}
		result[index/6][index%6] = weapon
	}
	return result, nil
}

type memory struct {
	data []byte
	base uint32
}

func (m memory) at(address uint32, count int) ([]byte, error) {
	if address < m.base || count < 0 || uint64(address-m.base)+uint64(count) > uint64(len(m.data)) {
		return nil, fmt.Errorf("original address $%x with size %d is outside the loader", address, count)
	}
	return m.data[address-m.base : address-m.base+uint32(count)], nil
}

func signedWord(data []byte) int { return int(int16(binary.BigEndian.Uint16(data))) }
