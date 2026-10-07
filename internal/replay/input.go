// Package replay stores only physical joystick inputs for deterministic checks.
package replay

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

const magic = "BSINPUT1"

// Run is a repeated pair of the original logical joysticks.
type Run struct {
	Fields uint32
	Inputs [2]engine.Input
}

// Recording contains source provenance and run-length encoded ordinary inputs.
type Recording struct {
	Source [32]byte
	Runs   []Run
	Fields uint64
}

func (r *Recording) Append(inputs [2]engine.Input) {
	r.Fields++
	if len(r.Runs) > 0 {
		last := &r.Runs[len(r.Runs)-1]
		if last.Inputs == inputs && last.Fields < ^uint32(0) {
			last.Fields++
			return
		}
	}
	r.Runs = append(r.Runs, Run{1, inputs})
}

func Encode(w io.Writer, r Recording) error {
	if _, err := io.WriteString(w, magic); err != nil {
		return err
	}
	if _, err := w.Write(r.Source[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, r.Fields); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(r.Runs))); err != nil {
		return err
	}
	for _, run := range r.Runs {
		if run.Fields == 0 {
			return fmt.Errorf("replay contains an empty input run")
		}
		if err := binary.Write(w, binary.LittleEndian, run.Fields); err != nil {
			return err
		}
		if _, err := w.Write([]byte{Mask(run.Inputs[0]), Mask(run.Inputs[1])}); err != nil {
			return err
		}
	}
	return nil
}

func Decode(reader io.Reader) (Recording, error) {
	var r Recording
	var header [8]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return r, err
	}
	if string(header[:]) != magic {
		return r, fmt.Errorf("unknown input recording")
	}
	if _, err := io.ReadFull(reader, r.Source[:]); err != nil {
		return r, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &r.Fields); err != nil {
		return r, err
	}
	var count uint32
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return r, err
	}
	if count > 10000000 || r.Fields > 100000000 {
		return r, fmt.Errorf("replay exceeds verification limits")
	}
	var sum uint64
	for i := uint32(0); i < count; i++ {
		var run Run
		if err := binary.Read(reader, binary.LittleEndian, &run.Fields); err != nil {
			return r, err
		}
		var masks [2]byte
		if _, err := io.ReadFull(reader, masks[:]); err != nil {
			return r, err
		}
		if run.Fields == 0 || masks[0]&0xc0 != 0 || masks[1]&0xc0 != 0 {
			return r, fmt.Errorf("invalid input run")
		}
		run.Inputs = [2]engine.Input{FromMask(masks[0]), FromMask(masks[1])}
		sum += uint64(run.Fields)
		r.Runs = append(r.Runs, run)
	}
	if sum != r.Fields {
		return r, fmt.Errorf("replay field count mismatch")
	}
	return r, nil
}

// Mask uses the original clockwise up/right/down/left/fire/Nova bit order.
func Mask(input engine.Input) byte {
	var mask byte
	if input.Y < 0 {
		mask |= 1
	}
	if input.X > 0 {
		mask |= 2
	}
	if input.Y > 0 {
		mask |= 4
	}
	if input.X < 0 {
		mask |= 8
	}
	if input.Fire {
		mask |= 16
	}
	if input.Nova {
		mask |= 32
	}
	return mask
}

func FromMask(mask byte) engine.Input {
	var input engine.Input
	if mask&1 != 0 {
		input.Y--
	}
	if mask&2 != 0 {
		input.X++
	}
	if mask&4 != 0 {
		input.Y++
	}
	if mask&8 != 0 {
		input.X--
	}
	input.Fire = mask&16 != 0
	input.Nova = mask&32 != 0
	return input
}
