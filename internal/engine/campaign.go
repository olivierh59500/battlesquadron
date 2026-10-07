package engine

import (
	"encoding/binary"
	"fmt"
	"slices"
)

// Gate preserves one original surface entrance check and its scenery template.
// WorldX is measured before the viewport's horizontal camera displacement.
type Gate struct {
	Phase, Progress, WorldX int
	ReturnScroll            int
	CodeAddress             uint32
	Definition              Definition
}

// NativeY is the original ground object's upper edge after its entrance check.
func (g Gate) NativeY(progress int) int { return progress - g.Progress - g.Definition.Height }

// DecodeSurfaceGates follows the loader's three CMPI.W/BTST/MOVE.W/MOVEQ
// checks to the shared portal template. It decodes operands and never executes
// guest code, so relocated cracker and original loader tables remain supported.
func DecodeSurfaceGates(loader []byte, base uint32) ([]Gate, error) {
	word := func(off int) uint16 { return binary.BigEndian.Uint16(loader[off:]) }
	returns, err := decodeSurfaceReturns(loader)
	if err != nil {
		return nil, err
	}
	for start := 0; start+76 <= len(loader); start += 2 {
		if word(start) != 0x0c6d || word(start+2) != 0x0f10 {
			continue
		}
		displacement := word(start + 4)
		var gates []Gate
		valid := true
		for index := 0; index < 3; index++ {
			off := start + index*24
			if word(off) != 0x0c6d || word(off+4) != displacement || word(off+8) != 0x082d || word(off+16) != 0x303c || word(off+20)&0xff00 != 0x7c00 {
				valid = false
				break
			}
			phase := int(word(off+20) & 0xff)
			counter := int(word(off + 18))
			progress := int(word(off + 2))
			if phase != index+1 || word(off+10) != uint16(phase) || counter > 23 || progress <= 0 {
				valid = false
				break
			}
			gates = append(gates, Gate{Phase: phase, Progress: progress, WorldX: (23 - counter) * 16, ReturnScroll: returns[index], CodeAddress: base + uint32(off)})
		}
		if !valid || word(start+70) != 0x247c {
			continue
		}
		address := binary.BigEndian.Uint32(loader[start+72:])
		definition, err := DecodeDefinition(loader, base, address)
		if err != nil {
			return nil, fmt.Errorf("original portal template: %w", err)
		}
		if definition.Kind != 0x27 || definition.Width != 32 || definition.Height != 32 {
			return nil, fmt.Errorf("unexpected original portal template at $%x", address)
		}
		// The portal controller selects only states zero and one, independent of
		// the generic scenery template's later animation/death bookkeeping bytes.
		definition.Frame, definition.Frames, definition.FireDelay = 0, 2, 0
		for index := range gates {
			gates[index].Definition = definition
		}
		if gates[0].Progress >= gates[1].Progress || gates[1].Progress >= gates[2].Progress {
			return nil, fmt.Errorf("unordered original surface gate thresholds")
		}
		return gates, nil
	}
	return nil, fmt.Errorf("the original three surface gate checks were not found")
}

// decodeSurfaceReturns reads the three original phase checks and fixed map rows.
// The transition fills a 256-pixel display ring before returning control, so the
// playable progress differs from both the map reload row and portal entry point.
func decodeSurfaceReturns(loader []byte) ([3]int, error) {
	word := func(off int) uint16 { return binary.BigEndian.Uint16(loader[off:]) }
	for start := 0; start+78 <= len(loader); start += 2 {
		valid := true
		var scrolls [3]int
		for index := range scrolls {
			off := start + index*26
			if word(off) != 0x0c6d || word(off+2) != uint16(index+1) || word(off+8) != 0x08ed || word(off+10) != uint16(index+1) || word(off+14) != 0x323c || word(off+18) != 0x2b7c || word(off+24) != 0xf4b0 {
				valid = false
				break
			}
			scrolls[index] = int(word(off+16))*16 + 256
		}
		if valid {
			return scrolls, nil
		}
	}
	return [3]int{}, fmt.Errorf("the original three surface return rows were not found")
}

// Campaign tracks original surface/cave routing, portal holds and cave-three's
// terrain-clock delay before returning to the surface. Fade pixels remain a
// separate presentation boundary.
type Campaign struct {
	Gates         []Gate
	SurfaceScroll int
	ClearedMask   uint8
	PortalHold    int
	Entrance      Gate
	ActiveCave    int
	EndHold       int
	WreckAwarded  [2]int
	endingCave    bool
}

// NewCampaign starts a fresh native campaign with a private entrance list.
func NewCampaign(gates []Gate) Campaign { return Campaign{Gates: slices.Clone(gates)} }

// GateAtScroll selects a visible, uncleared entrance while on the surface.
// The supplied revision releases its entrance phase at a top edge of 204.
func (c *Campaign) GateAtScroll(progress int) (Gate, bool) {
	if c.ActiveCave != 0 {
		return Gate{}, false
	}
	for _, gate := range c.Gates {
		if c.ClearedMask&(1<<uint(gate.Phase)) != 0 || progress < gate.Progress {
			continue
		}
		if gate.NativeY(progress) >= 204 {
			continue
		}
		return gate, true
	}
	return Gate{}, false
}

// UpdatePortal requires every active, nondead player's position to lie inside
// the original 32-pixel watch box for ten consecutive ticks. Left, right and
// top edges are inclusive; the bottom edge is strictly excluded. Respawning
// ships still participate, as their original life-state byte remains live.
// The tenth tick returns true once; leaving the box rearms the detector.
func (c *Campaign) UpdatePortal(players [2]Player, x, y int, active bool) bool {
	if !active || c.ActiveCave != 0 {
		c.PortalHold = 0
		return false
	}
	living := 0
	for _, player := range players {
		if !player.Active || player.Lives <= 0 || player.Dying > 0 {
			continue
		}
		living++
		if player.X < x-16 || player.X > x+16 || player.Y < y-16 || player.Y >= y+16 {
			c.PortalHold = 0
			return false
		}
	}
	if living == 0 {
		c.PortalHold = 0
		return false
	}
	if c.PortalHold >= 10 {
		return false
	}
	c.PortalHold++
	return c.PortalHold == 10
}

// EnterCave remembers the surface state and returns the original cave phase.
func (c *Campaign) EnterCave(phase, scroll int) (int, error) {
	if c.ActiveCave != 0 {
		return 0, fmt.Errorf("a cave is already active")
	}
	if phase < 1 || phase > 3 || c.ClearedMask&(1<<uint(phase)) != 0 {
		return 0, fmt.Errorf("cave phase %d is unavailable", phase)
	}
	var entrance Gate
	for _, gate := range c.Gates {
		if gate.Phase == phase {
			entrance = gate
			break
		}
	}
	if entrance.Phase == 0 {
		return 0, fmt.Errorf("cave phase %d has no original entrance", phase)
	}
	c.SurfaceScroll, c.ActiveCave, c.Entrance, c.PortalHold = scroll, phase, entrance, 0
	c.EndHold, c.endingCave = 0, false
	return phase, nil
}

// CaveReturnReady preserves the original 300 terrain updates after cave three's
// map ends. It does not change the player's 50 Hz movement or timer cadence.
func (c *Campaign) CaveReturnReady() bool {
	if c.ActiveCave != 3 {
		return true
	}
	if !c.endingCave {
		c.EndHold, c.endingCave = 300, true
		return false
	}
	if c.EndHold > 0 {
		c.EndHold--
	}
	return c.EndHold == 0
}

// ExitCave marks only the completed cave and returns its fixed playable surface
// progress after the original transition's display-ring prefill.
func (c *Campaign) ExitCave() (int, error) {
	if c.ActiveCave < 1 || c.ActiveCave > 3 {
		return 0, fmt.Errorf("no cave is active")
	}
	c.ClearedMask |= 1 << uint(c.ActiveCave)
	scroll := c.Entrance.ReturnScroll
	if scroll == 0 {
		// Data-only fixtures without a recovered loader retain their entry point.
		scroll = c.SurfaceScroll
	}
	c.ActiveCave, c.PortalHold, c.Entrance = 0, 0, Gate{}
	c.EndHold, c.endingCave = 0, false
	return scroll, nil
}

// Completed reports the original three cave-clear bits, without ending a game.
func (c *Campaign) Completed() bool { return c.ClearedMask&0x0e == 0x0e }

// originalScoreIncrement decodes the eight decimal digits preceding an original
// score-adder operand. They are data bytes, not ASCII or packed BCD words.
func (e *Engine) originalScoreIncrement(end uint32) int {
	if e.Data == nil || end < 8 {
		return 0
	}
	digits, err := (memory{e.Data.Loader, e.Data.LoaderBase}).at(end-8, 8)
	if err != nil {
		return 0
	}
	value := 0
	for _, digit := range digits {
		if digit > 9 {
			return 0
		}
		value = value*10 + int(digit)
	}
	return value
}

// tickWreckBonus translates the original even-clock payout after its 160-update
// text introduction. Score wraps at eight decimal digits, as the source adder
// drops the final carry. Disabled ships lose their unclaimed wreck bonuses.
func (e *Engine) tickWreckBonus(clock int) {
	if clock < 160 || clock&1 != 0 {
		return
	}
	increment := e.originalScoreIncrement(0x7322)
	for index := range e.Players {
		p := &e.Players[index]
		if !p.Active || p.Lives == 0 && p.Dying == 0 {
			p.WreckBonus = 0
			continue
		}
		if p.WreckBonus > 0 {
			p.WreckBonus--
			e.Campaign.WreckAwarded[index]++
			p.Score = (p.Score + increment) % 100000000
		}
	}
}

// settleWreckBonus folds only the currently instantaneous transition's payout.
// Its final balances match the source; the animated countdown remains unproved.
func (e *Engine) settleWreckBonus() {
	e.Campaign.WreckAwarded = [2]int{}
	for clock := 160; clock < 360; clock += 2 {
		e.tickWreckBonus(clock)
	}
}

// ApplyOriginalEndingBonusTick grants one original 1000-point increment to each
// enabled ship. The caller invokes it for the ending's first 100 PAL updates.
func (e *Engine) ApplyOriginalEndingBonusTick() {
	increment := e.originalScoreIncrement(0x401a)
	for index := range e.Players {
		p := &e.Players[index]
		if p.Active && (p.Lives > 0 || p.Dying > 0) {
			p.Score = (p.Score + increment) % 100000000
		}
	}
}
