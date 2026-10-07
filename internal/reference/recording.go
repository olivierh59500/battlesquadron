// Package reference reads development-only FS-UAE checkpoints and input records.
// It never executes original game code and is not imported by the application.
package reference

import (
	"encoding/binary"
	"fmt"
)

// Event preserves an FS-UAE input's exact frame and raster-line position.
type Event struct {
	Frame, Line int
	ID          uint16
	State       int8
}

// Frame contains the two source checksums and inputs recorded at that frame.
type Frame struct {
	Number                        int
	RandomChecksum, StateChecksum uint32
	Events                        []Event
}

// Recording follows the big-endian word format from FS-UAE 3.1.66 recording.c.
// Parsing its checksums does not establish that a replay has validated them.
type Recording struct{ Frames []Frame }

func DecodeRecording(data []byte) (Recording, error) {
	var record Recording
	if len(data)%4 != 0 {
		return record, fmt.Errorf("FS-UAE recording has an incomplete word")
	}
	line := 0
	for off := 0; off < len(data); off += 4 {
		word := binary.BigEndian.Uint32(data[off:])
		switch {
		case word&0x80000000 != 0:
			if off+12 > len(data) {
				return record, fmt.Errorf("FS-UAE frame has incomplete checksums")
			}
			random, state := binary.BigEndian.Uint32(data[off+4:]), binary.BigEndian.Uint32(data[off+8:])
			if random&0xf0000000 != 0x10000000 || state&0x08000000 == 0 {
				return record, fmt.Errorf("FS-UAE frame has invalid checksum markers")
			}
			number := int(word & 0x7fffffff)
			if len(record.Frames) > 0 && number != record.Frames[len(record.Frames)-1].Number+1 {
				return record, fmt.Errorf("FS-UAE frame sequence is discontinuous")
			}
			record.Frames = append(record.Frames, Frame{Number: number, RandomChecksum: random & 0xffffff, StateChecksum: state & 0xffffff})
			off += 8
			line = 0
		case word&0xc0000000 == 0x40000000:
			line = int(word & 0xffffff)
		case word&0xe0000000 == 0x20000000:
			if len(record.Frames) == 0 {
				return record, fmt.Errorf("FS-UAE input precedes its frame")
			}
			frame := &record.Frames[len(record.Frames)-1]
			frame.Events = append(frame.Events, Event{Frame: frame.Number, Line: line, ID: uint16(word), State: int8(word >> 16)})
		default:
			return record, fmt.Errorf("unknown FS-UAE recording word$%08x", word)
		}
	}
	return record, nil
}

func (r Recording) LastFrame() int {
	if len(r.Frames) == 0 {
		return 0
	}
	return r.Frames[len(r.Frames)-1].Number
}

// HeldRightWindow accepts only a constant, observed right input across a window.
// Other events or a release make this bounded diagnostic unsuitable for replay.
func (r Recording) HeldRightWindow(start, end int, right uint16) ([]Event, error) {
	held := false
	var events []Event
	for _, frame := range r.Frames {
		if frame.Number > end {
			break
		}
		for _, event := range frame.Events {
			if event.ID == right {
				held = event.State != 0
			}
			if frame.Number > start && (event.ID != right || !held) {
				return nil, fmt.Errorf("input changes inside the held-right comparison window")
			}
			events = append(events, event)
		}
		if frame.Number >= start && !held {
			return nil, fmt.Errorf("right input is not held at recording frame%d", frame.Number)
		}
	}
	if end > r.LastFrame() || start >= end {
		return nil, fmt.Errorf("comparison window is outside the recording")
	}
	return events, nil
}
