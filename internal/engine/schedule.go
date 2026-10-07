package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// NativeSchedules decodes all four original scroll-triggered flying-object
// streams. The compact schedules are data records, never executable code.
// Stage one's stream is embedded at the start of LODST1; the other three are
// resident loader data. LODS0S is a graphics bank, not a replacement wave list.
func NativeSchedules(loader []byte, base uint32, overlays map[string][]byte) ([4][]Spawn, error) {
	var result [4][]Spawn
	table, err := nativeStageTable(loader, base)
	if err != nil {
		return result, err
	}
	definitions, err := nativeFlyingTable(loader, base)
	if err != nil {
		return result, err
	}
	vectors, err := nativeDirectionTable(loader, base)
	if err != nil {
		return result, err
	}
	resident := memory{loader, base}
	for stage := range result {
		bankName := []string{"lods0f", "lodst1", "lodst2", "lodst3"}[stage]
		bank := overlays[bankName]
		if len(bank) == 0 {
			bank = overlays[bankName+".bin"]
		}
		bankBase := []uint32{0x2e508, 0x2e89a, 0x2e4c0, 0x2e840}[stage]
		dataMemory := scheduleMemory{resident, memory{bank, bankBase}}
		descriptor, err := resident.at(table+uint32(stage)*140, 12)
		if err != nil {
			return result, err
		}
		pointer := binary.BigEndian.Uint32(descriptor)
		source := resident
		if stage == 1 {
			data := overlays["lodst1"]
			if len(data) == 0 {
				data = overlays["lodst1.bin"]
			}
			if len(data) == 0 {
				return result, fmt.Errorf("LODST1 is required for its original flying schedule")
			}
			source = memory{data, pointer}
		}
		previous := -1
		terminated := false
		for record := 0; record < 4096; record++ {
			first, err := source.at(pointer, 2)
			if err != nil {
				return result, err
			}
			progress := int(binary.BigEndian.Uint16(first))
			if progress == 65535 {
				terminated = true
				break
			}
			if progress < previous {
				return result, fmt.Errorf("stage%d original schedule decreases at $%x", stage, pointer)
			}
			previous = progress
			row, err := source.at(pointer, 12)
			if err != nil {
				return result, err
			}
			pointer += 12
			event := Spawn{Progress: progress, X: signedWord(row[2:]), Y: signedWord(row[4:])}
			if event.X == -1 {
				event.ClearObjects = true
				result[stage] = append(result[stage], event)
				continue
			}
			kind := row[6]
			if kind > 13 {
				return result, fmt.Errorf("stage%d unknown original flying kind%d", stage, kind)
			}
			event.Definition, err = decodeFlyingDefinition(loader, base, definitions+uint32(kind)*32, kind)
			if err != nil {
				return result, err
			}
			if event.X >= 800 {
				event.X -= 1000
				event.AbsoluteX = kind != 1 && kind != 6 && kind != 8
			}
			if kind == 1 || kind == 6 || kind == 8 {
				event.RandomMode = kind
			}
			script := binary.BigEndian.Uint32(row[8:])
			// The cracked loader rewrites these eight stage-one references after the
			// overlay is installed. Apply that verified relocation to the data only.
			if stage == 1 && script >= 0xf068 && script <= 0xf0e6 && (script-0xf068)%18 == 0 {
				script = 0xe88a + (script - 0xf068)
			}
			if kind == 0 {
				event.Script, event.ScriptLoop, event.RepeatScript, err = decodeDirectionScript(dataMemory, vectors, script)
				if err != nil {
					return result, fmt.Errorf("stage%d schedule $%x: %w", stage, pointer-12, err)
				}
			} else if (kind == 3 || kind == 13) && script != 0 {
				event.Script, err = decodeMaskScript(dataMemory, script)
				if err != nil {
					return result, fmt.Errorf("stage%d schedule $%x: %w", stage, pointer-12, err)
				}
			} else if stage == 1 && kind == 9 {
				event.Script, err = decodeBossCircle(dataMemory, 0x2ef20)
				if err != nil {
					return result, err
				}
				event.RepeatScript = true
			}
			result[stage] = append(result[stage], event)
		}
		if !terminated {
			return result, fmt.Errorf("stage%d original flying schedule lacks its terminator", stage)
		}
	}
	return result, nil
}

func decodeBossCircle(source scheduleReader, address uint32) ([]Motion, error) {
	data, err := source.at(address, 400*4)
	if err != nil {
		return nil, err
	}
	motions := make([]Motion, 400)
	for i := range motions {
		motions[i] = Motion{Duration: 1, VX: signedWord(data[i*4:]) * 16, VY: signedWord(data[i*4+2:]) * 16}
	}
	return motions, nil
}

// nativeStageTable follows the original immediate table assignment used by the
// new-game initializer. Validate all four descriptor pointers before accepting it.
func nativeStageTable(loader []byte, base uint32) (uint32, error) {
	for at := 0; at+8 <= len(loader); at += 2 {
		if !bytes.Equal(loader[at:at+2], []byte{0x2b, 0x7c}) || !bytes.Equal(loader[at+6:at+8], []byte{0x1b, 0x98}) {
			continue
		}
		pointer := binary.BigEndian.Uint32(loader[at+2:])
		records, err := (memory{loader, base}).at(pointer, 4*140)
		if err != nil {
			continue
		}
		valid := true
		for index := 0; index < 4; index++ {
			p := binary.BigEndian.Uint32(records[index*140:])
			if p == 0 || p&1 != 0 {
				valid = false
			}
		}
		if valid {
			return pointer, nil
		}
	}
	return 0, fmt.Errorf("the original four-stage schedule descriptor table was not found")
}

func nativeFlyingTable(loader []byte, base uint32) (uint32, error) {
	// MULU #$20,D3 followed by MOVEA.L #table,A2 and ADDA.W D3,A2.
	prefix := []byte{0xc6, 0xfc, 0x00, 0x20, 0x24, 0x7c}
	for at := 0; at+12 <= len(loader); at += 2 {
		if !bytes.Equal(loader[at:at+6], prefix) {
			continue
		}
		pointer := binary.BigEndian.Uint32(loader[at+6:])
		if _, err := (memory{loader, base}).at(pointer, 14*32); err == nil {
			return pointer, nil
		}
	}
	return 0, fmt.Errorf("the original fourteen-entry flying template table was not found")
}

func nativeDirectionTable(loader []byte, base uint32) (uint32, error) {
	// The direction handler loads this 32-entry signed X/Y table after masking
	// the direction byte to five bits. Accept its demonstrated trailing signature.
	prefix := []byte{0x22, 0x7c}
	for at := 0; at+8 <= len(loader); at += 2 {
		if !bytes.Equal(loader[at:at+2], prefix) || !bytes.Equal(loader[at+6:at+8], []byte{0xd2, 0xc3}) {
			continue
		}
		pointer := binary.BigEndian.Uint32(loader[at+2:])
		table, err := (memory{loader, base}).at(pointer, 128)
		if err != nil {
			continue
		}
		if signedWord(table[8*4:]) == 4096 && signedWord(table[24*4:]) == -4096 {
			return pointer, nil
		}
	}
	return 0, fmt.Errorf("the original thirty-two-direction motion table was not found")
}

func decodeFlyingDefinition(loader []byte, base, address uint32, kind byte) (Definition, error) {
	record, err := (memory{loader, base}).at(address, 32)
	if err != nil {
		return Definition{}, err
	}
	height, widthWords := int(binary.BigEndian.Uint16(record)), int(binary.BigEndian.Uint16(record[2:]))
	if height < 1 || height > 128 || widthWords < 2 || widthWords > 8 {
		return Definition{}, fmt.Errorf("invalid original flying geometry at $%x", address)
	}
	stride := (widthWords - 1) * 2 * height
	frames := 16
	killSound := 28
	if kind == 2 {
		killSound = 29
	}
	if kind == 4 || kind == 8 {
		frames = 32
	}
	return Definition{Address: address, GraphicAddress: binary.BigEndian.Uint32(record[20:]), Kind: kind, Graphic: kind,
		Width: (widthWords - 1) * 16, Height: height, Health: int(record[12]), Score: DecodeBCDScore(binary.BigEndian.Uint16(record[24:])),
		HitSound: int(record[13]), KillSound: killSound,
		PlaneStride: stride, FrameStride: stride * 6, Frames: frames, Frame: 0, Sprite: fmt.Sprintf("flying_%d_0", kind), NativeKind: int(kind), FlyingPool: true, Boss: kind == 2 || kind == 9, Collectable: kind == 5, TrackingFrames: int(record[27])}, nil
}

// decodeDirectionScript expands four-byte original commands to integer motion
// segments. Turns happen after movement; the displayed frame uses the new angle.
// An FF00 command jumps to an absolute stream pointer, while FF01 removes it.
func decodeDirectionScript(source scheduleReader, vectors, pointer uint32) ([]Motion, int, bool, error) {
	if pointer == 0 {
		return nil, 0, false, fmt.Errorf("a directed flying formation has no original script")
	}
	motions := []Motion{}
	visited := map[uint32]int{}
	for commands := 0; commands < 4096; commands++ {
		if start, ok := visited[pointer]; ok {
			return motions, start, true, nil
		}
		visited[pointer] = len(motions)
		first, err := source.at(pointer, 2)
		if err != nil {
			return nil, 0, false, err
		}
		if first[0] == 255 {
			switch first[1] {
			case 0:
				jump, err := source.at(pointer+2, 4)
				if err != nil {
					return nil, 0, false, err
				}
				pointer = binary.BigEndian.Uint32(jump)
				continue
			case 1:
				return motions, 0, false, nil
			default:
				return nil, 0, false, fmt.Errorf("unknown directed script terminator%d at $%x", first[1], pointer)
			}
		}
		command, err := source.at(pointer, 4)
		if err != nil {
			return nil, 0, false, err
		}
		pointer += 4
		if command[0] == 0 {
			return nil, 0, false, fmt.Errorf("zero-duration directed command at $%x", pointer-4)
		}
		direction, delay := command[1], command[3]
		for tick := 0; tick < int(command[0]); tick++ {
			vector, err := source.at(vectors+uint32(direction&31)*4, 4)
			if err != nil {
				return nil, 0, false, err
			}
			factor := int(direction&224)>>2 + 8
			motion := Motion{Duration: 1, VX: signedWord(vector) * factor, VY: signedWord(vector[2:]) * factor}
			if delay != 0 {
				delay--
				if delay == 0 {
					direction += command[2]
					delay = command[3]
				}
			}
			motion.Frame = int(direction&31) >> 1
			// Keep command boundaries as loop targets; merge only inside this command.
			if tick > 0 && len(motions) > 0 {
				last := &motions[len(motions)-1]
				if last.VX == motion.VX && last.VY == motion.VY && last.Frame == motion.Frame {
					last.Duration++
					continue
				}
			}
			motions = append(motions, motion)
		}
	}
	return nil, 0, false, fmt.Errorf("the original directed movement script did not terminate")
}

func decodeMaskScript(source scheduleReader, pointer uint32) ([]Motion, error) {
	motions := []Motion{}
	for commands := 0; commands < 4096; commands++ {
		row, err := source.at(pointer, 4)
		if err != nil {
			return nil, err
		}
		pointer += 4
		if binary.BigEndian.Uint16(row) == 0 {
			return motions, nil
		}
		duration := int(int8(row[0]))
		velocity := int(int8(row[1]))
		motion := Motion{Frame: 0}
		if duration < 0 {
			motion.Duration = -duration
			motion.VX = velocity << 16
			motion.VY = 1 << 16
		} else {
			motion.Duration = duration
			motion.VY = (velocity + 1) << 16
		}
		if motion.Duration == 0 {
			return nil, fmt.Errorf("invalid mask script duration at $%x", pointer-4)
		}
		motions = append(motions, motion)
	}
	return nil, fmt.Errorf("the original mask movement script did not terminate")
}

type scheduleReader interface {
	at(uint32, int) ([]byte, error)
}
type scheduleMemory []memory

func (m scheduleMemory) at(address uint32, count int) ([]byte, error) {
	for _, bank := range m {
		if data, err := bank.at(address, count); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("original script address $%x with size %d is outside the resident and stage banks", address, count)
}
