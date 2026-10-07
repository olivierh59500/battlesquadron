package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// DecodeMapRules locates original CMPI.W tile checks and their object pointers.
// It reads data operands; no original instruction is executed by the engine.
func DecodeMapRules(loader []byte, base uint32) []MapRule {
	groups := [][]uint16{
		{0x6180, 0x5640, 0x0280, 0x8020, 0x5d20, 0x5d70, 0x5910, 0x59b0},
		{0x5c80, 0x00a0, 0x0230, 0x0d70, 0x92e0},
		{0x7260, 0x9420, 0x0cd0},
		{0x8c00, 0x9420, 0x92e0, 0x3a70, 0x4bf0, 0x3ed0, 0x3340, 0x4e70, 0x4290, 0x3520},
	}
	var rules []MapRule
	cursor := 0
	for mode, tiles := range groups {
		for _, tile := range tiles {
			pattern := []byte{0x0c, 0x41, byte(tile >> 8), byte(tile)}
			found := bytes.Index(loader[cursor:], pattern)
			if found < 0 {
				continue
			}
			offset := cursor + found + 4
			cursor = offset
			end := min(len(loader), offset+44)
			operand := bytes.Index(loader[offset:end], []byte{0x24, 0x7c})
			if operand < 0 || offset+operand+6 > len(loader) {
				continue
			}
			address := binary.BigEndian.Uint32(loader[offset+operand+2:])
			definition, err := DecodeDefinition(loader, base, address)
			if err != nil {
				continue
			}
			rule := MapRule{Mode: mode, Tile: tile, Definition: definition}
			if mode == 0 && tile == 0x5640 {
				rule.FollowingTile = 0x5cd0
			}
			if mode == 0 && tile == 0x0280 {
				rule.MinProgress, rule.MaxProgress = 0x03e8, 0x1f40
			}
			if mode == 1 && tile == 0x92e0 {
				rule.MaxProgress = 0x19c8
			}
			if mode == 3 && tile == 0x92e0 {
				rule.MaxProgress, rule.RandomBelow = 0x0dde, 0x40
			}
			rules = append(rules, rule)
		}
	}
	return rules
}

// DecodeDefinition reads one original forty-eight-byte scenery-object template.
func DecodeDefinition(loader []byte, base, address uint32) (Definition, error) {
	record, err := (memory{loader, base}).at(address, 48)
	if err != nil {
		return Definition{}, err
	}
	widthWords, height := int(binary.BigEndian.Uint16(record[8:])), int(binary.BigEndian.Uint16(record[6:]))
	if widthWords < 1 || widthWords > 8 || height < 1 || height > 128 {
		return Definition{}, fmt.Errorf("invalid original object geometry at $%x", address)
	}
	kind, frame := record[17], int(record[25])
	definition := Definition{
		Address: address, GraphicAddress: binary.BigEndian.Uint32(record[12:]),
		Kind: kind, Graphic: kind, Frame: frame,
		Width: widthWords * 16, Height: height, Health: int(record[28]), Score: DecodeBCDScore(binary.BigEndian.Uint16(record[44:])),
		PlaneStride: widthWords * 2 * height, FrameStride: widthWords * 2 * height * 5,
		Frames: max(1, int(record[33])-frame+1), Ground: true,
		Sprite: fmt.Sprintf("object_%d_%d", kind, frame),
	}
	// A nonzero random firing mask supplies the original initial shot countdown.
	if record[43] != 0 && kind != 0x27 {
		definition.FireDelay = int(record[43]) + 1
	}
	return definition, nil
}

func (e *Engine) advanceStage() {
	if len(e.Data.Stages) == 0 {
		return
	}
	stage := &e.Data.Stages[e.Stage]
	if e.Scroll < stage.Height {
		e.Scroll++
		if e.Scroll%16 == 0 {
			e.exposeMapRow(*stage)
		}
	}
	if len(e.Campaign.Gates) > 0 && e.Stage == 0 {
		gate, active := e.Campaign.GateAtScroll(e.Scroll)
		if e.Campaign.UpdatePortal(e.Players, gate.WorldX-e.CameraX, gate.NativeY(e.Scroll), active) {
			if cave, err := e.Campaign.EnterCave(gate.Phase, e.Scroll); err == nil {
				e.SelectStage(cave, 0)
				return
			}
		}
	}
	for e.nextEvent < len(stage.Events) && stage.Events[e.nextEvent].Progress <= e.Scroll {
		e.Spawn(stage.Events[e.nextEvent])
		e.nextEvent++
	}
	if e.Scroll < stage.Height || stage.Height == 0 {
		return
	}
	if stage.Boss != nil && !e.bossSpawned {
		e.Spawn(*stage.Boss)
		e.bossSpawned = true
		return
	}
	for _, enemy := range e.Enemies {
		if enemy.Definition.Boss && enemy.Health >= 0 {
			return
		}
	}
	if len(e.Campaign.Gates) > 0 {
		if e.Stage > 0 && e.Campaign.ActiveCave == e.Stage {
			if scroll, err := e.Campaign.ExitCave(); err == nil {
				e.SelectStage(0, scroll)
				return
			}
		}
		if e.Stage == 0 && !e.Campaign.Completed() {
			e.SelectStage(0, 160)
			return
		}
		if e.Stage == 0 && e.Campaign.Completed() {
			e.Mode, e.modeFrames = Ending, 0
			e.Events = append(e.Events, Event{Kind: "ending"})
			return
		}
	}
	if e.Stage+1 == len(e.Data.Stages) {
		e.Mode, e.modeFrames = Ending, 0
		e.Events = append(e.Events, Event{Kind: "ending"})
		return
	}
	e.Stage++
	e.Scroll, e.nextEvent, e.modeFrames = 0, 0, 0
	e.bossSpawned = false
	e.Enemies, e.PlayerShots, e.EnemyShots, e.Pickups = nil, nil, nil, nil
	e.Events = append(e.Events, Event{Kind: "stage", Value: e.Stage})
}

// SelectStage preserves ships while switching original map and wave banks.
// The animated warp compositor remains a separate presentation boundary.
func (e *Engine) SelectStage(stage, scroll int) {
	if stage < 0 || stage >= len(e.Data.Stages) {
		return
	}
	e.Stage, e.Scroll, e.nextEvent, e.bossSpawned = stage, scroll, 0, false
	e.Enemies, e.PlayerShots, e.EnemyShots, e.Pickups, e.Explosions = nil, nil, nil, nil, nil
	for e.nextEvent < len(e.Data.Stages[stage].Events) && e.Data.Stages[stage].Events[e.nextEvent].Progress <= scroll {
		e.nextEvent++
	}
	for index := range e.Players {
		e.Players[index].Invulnerable = max(100, e.Players[index].Invulnerable)
	}
	e.Events = append(e.Events, Event{Kind: "stage", Value: stage})
}

func (e *Engine) exposeMapRow(stage Stage) {
	if stage.Width <= 0 || len(stage.Tiles) < stage.Width {
		return
	}
	row := len(stage.Tiles)/stage.Width - 1 - e.Scroll/16
	if row < 0 {
		return
	}
	for column := 0; column < stage.Width; column++ {
		tile := stage.Tiles[row*stage.Width+column]
		for _, rule := range e.Data.MapRules {
			if rule.Mode != stage.Mode || rule.Tile != tile || e.Scroll < rule.MinProgress || rule.MaxProgress > 0 && e.Scroll >= rule.MaxProgress {
				continue
			}
			if rule.FollowingTile != 0 && (row+1)*stage.Width+column < len(stage.Tiles) && stage.Tiles[(row+1)*stage.Width+column] != rule.FollowingTile {
				continue
			}
			if rule.RandomBelow > 0 && int(e.nextRandom()) >= rule.RandomBelow {
				continue
			}
			e.Spawn(Spawn{X: column*16 - e.CameraX, Y: -rule.Definition.Height, Definition: rule.Definition})
		}
	}
}

func (e *Engine) nextRandom() byte {
	if len(e.Data.Random) == 0 {
		// A missing original random-byte source keeps optional map spawns closed.
		return 255
	}
	value := e.Data.Random[e.randomCursor%len(e.Data.Random)]
	e.randomCursor++
	return value
}
