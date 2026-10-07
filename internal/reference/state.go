package reference

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

// State retains selected original scalars and capture-phase provenance.
// RAM is private diagnostic data and is omitted from generated JSON reports.
type State struct {
	RAM                            []byte `json:"-"`
	CycleUnit                      uint32
	StoredCycles                   uint64
	PC                             uint32
	Clock, Stage, Progress, Camera int
	Cleared                        uint8
	Players                        [2]engine.Player
}

// DecodeState reads UAE chunk padding, compressed chip RAM and CPU provenance.
// FS-UAE save_cpu writes 15 registers after its eight-byte header: PC is at$44,
// while$4C is USP. This reader deliberately does not interpret guest instructions.
func DecodeState(data []byte) (State, error) {
	var state State
	for off := 0; off+12 <= len(data); {
		id := string(data[off : off+4])
		size := int(binary.BigEndian.Uint32(data[off+4:]))
		flags := binary.BigEndian.Uint32(data[off+8:])
		if size < 12 || size > len(data)-off {
			break
		}
		payload := data[off+12 : off+size]
		if flags&1 != 0 {
			if len(payload) < 4 {
				return state, fmt.Errorf("compressed UAE chunk has no length")
			}
			expected := int(binary.BigEndian.Uint32(payload))
			reader, err := zlib.NewReader(bytes.NewReader(payload[4:]))
			if err != nil {
				return state, err
			}
			decoded, err := io.ReadAll(io.LimitReader(reader, 16<<20))
			reader.Close()
			if err != nil || len(decoded) != expected {
				return state, fmt.Errorf("compressed UAE chunk has an invalid payload")
			}
			payload = decoded
		}
		switch id {
		case "CRAM":
			state.RAM = payload
		case "CYCS":
			if len(payload) >= 16 {
				state.CycleUnit = binary.BigEndian.Uint32(payload[4:])
				state.StoredCycles = binary.BigEndian.Uint64(payload[8:])
			}
		case "CPU ":
			if len(payload) >= 0x48 {
				state.PC = binary.BigEndian.Uint32(payload[0x44:])
			}
		}
		off += size + 4 - size%4
	}
	if len(state.RAM) < 0x9ba0 || state.CycleUnit == 0 {
		return state, fmt.Errorf("state lacks original chip RAM or cycle provenance")
	}
	word := func(address int) int { return int(binary.BigEndian.Uint16(state.RAM[address:])) }
	state.Clock, state.Stage, state.Progress, state.Camera = word(0x1058), word(0x9b9c), word(0x9b86), word(0x9b84)-256
	state.Cleared = state.RAM[0x6f5d]
	for index := range state.Players {
		p := 0x4da2 + index*266
		score, err := strconv.Atoi(string(state.RAM[p+106 : p+114]))
		if err != nil {
			return state, fmt.Errorf("original score is not eight ASCII digits")
		}
		marker := state.RAM[p+38]
		active, lives := marker < 175, int(state.RAM[p+56])
		if active && (marker == 0 || marker == 100 || state.RAM[p+41] != 0) {
			lives++
		}
		state.Players[index] = engine.Player{Active: active, X: word(p) - 256, Y: word(p+2) - 256, Tilt: word(p+10) / 2, Lives: lives,
			Weapon: word(p + 58), Level: word(p + 60), Nova: word(p + 66), Score: score, Invulnerable: word(p + 52), Cooldown: word(p + 46), Repeat: int(state.RAM[p+57]), Respawn: int(state.RAM[p+48]), Dying: int(state.RAM[p+49])}
	}
	return state, nil
}
