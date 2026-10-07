package replay

import (
	"fmt"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

type cursorRun struct {
	fields uint32
	masks  [2]byte
}

// Cursor plays ordinary joystick inputs without forecasts or engine mutations.
// It owns a compact copy of the runs, so the caller may discard its Recording.
type Cursor struct {
	runs      []cursorRun
	index     int
	remaining uint32
	position  uint64
	fields    uint64
	inputs    [2]engine.Input
}

// NewCursor validates and compacts a recording once, outside the frame loop.
func NewCursor(recording Recording) (*Cursor, error) {
	cursor := &Cursor{fields: recording.Fields, runs: make([]cursorRun, len(recording.Runs))}
	var fields uint64
	for index, run := range recording.Runs {
		if run.Fields == 0 {
			return nil, fmt.Errorf("replay contains an empty input run")
		}
		fields += uint64(run.Fields)
		cursor.runs[index] = cursorRun{fields: run.Fields, masks: [2]byte{Mask(run.Inputs[0]), Mask(run.Inputs[1])}}
	}
	if fields != recording.Fields {
		return nil, fmt.Errorf("replay field count mismatch")
	}
	return cursor, nil
}

// Next returns one PAL field's input in constant time, without allocations.
// A false result means the complete recording has been consumed.
func (c *Cursor) Next() ([2]engine.Input, bool) {
	if c.remaining == 0 {
		if c.index == len(c.runs) {
			return [2]engine.Input{}, false
		}
		run := c.runs[c.index]
		c.index++
		c.remaining = run.fields
		c.inputs = [2]engine.Input{FromMask(run.masks[0]), FromMask(run.masks[1])}
	}
	c.remaining--
	c.position++
	return c.inputs, true
}

// Reset restarts playback without modifying the stored inputs or allocating.
func (c *Cursor) Reset() {
	c.index, c.remaining, c.position = 0, 0, 0
	c.inputs = [2]engine.Input{}
}

// Fields returns the immutable recording length in original PAL fields.
func (c *Cursor) Fields() uint64 { return c.fields }

// Position returns how many inputs have been consumed since the latest Reset.
func (c *Cursor) Position() uint64 { return c.position }
