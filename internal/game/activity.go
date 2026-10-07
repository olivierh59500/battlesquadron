package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// activityMonitor observes physical input before menu/game handlers consume it.
// Buffers and raw-pad neutral baselines are reused during ordinary rendering.
type activityMonitor struct {
	keys          []ebiten.Key
	chars         []rune
	touches       []ebiten.TouchID
	pads          []ebiten.GamepadID
	buttons       []ebiten.GamepadButton
	rawNeutral    map[ebiten.GamepadID][]float64
	cursorX       int
	cursorY       int
	cursorSampled bool
}

func (m *activityMonitor) poll() playerActivity {
	m.keys = inpututil.AppendPressedKeys(m.keys[:0])
	m.chars = ebiten.AppendInputChars(m.chars[:0])
	m.touches = ebiten.AppendTouchIDs(m.touches[:0])
	held := len(m.keys) != 0 || len(m.touches) != 0
	for button := ebiten.MouseButtonLeft; button <= ebiten.MouseButtonMax; button++ {
		held = held || ebiten.IsMouseButtonPressed(button)
	}
	x, y := ebiten.CursorPosition()
	moved := m.cursorSampled && (x != m.cursorX || y != m.cursorY)
	m.cursorX, m.cursorY, m.cursorSampled = x, y, true
	wheelX, wheelY := ebiten.Wheel()
	active := held || moved || wheelX != 0 || wheelY != 0 || len(m.chars) != 0
	m.pads = ebiten.AppendGamepadIDs(m.pads[:0])
	for _, id := range m.pads {
		m.buttons = inpututil.AppendPressedGamepadButtons(id, m.buttons[:0])
		padHeld := len(m.buttons) != 0
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			for _, axis := range []ebiten.StandardGamepadAxis{ebiten.StandardGamepadAxisLeftStickHorizontal, ebiten.StandardGamepadAxisLeftStickVertical, ebiten.StandardGamepadAxisRightStickHorizontal, ebiten.StandardGamepadAxisRightStickVertical} {
				padHeld = padHeld || math.Abs(ebiten.StandardGamepadAxisValue(id, axis)) > 0.3
			}
			for button := ebiten.StandardGamepadButton(0); button <= ebiten.StandardGamepadButtonMax; button++ {
				padHeld = padHeld || ebiten.StandardGamepadButtonValue(id, button) > 0.3
			}
		} else {
			if m.rawNeutral == nil {
				m.rawNeutral = make(map[ebiten.GamepadID][]float64)
			}
			axes := ebiten.GamepadAxisNum(id)
			neutral, known := m.rawNeutral[id]
			if !known || len(neutral) != axes {
				neutral = make([]float64, axes)
				for axis := range neutral {
					neutral[axis] = ebiten.GamepadAxisValue(id, axis)
				}
				m.rawNeutral[id] = neutral
				active = true
			}
			for axis, baseline := range neutral {
				padHeld = padHeld || math.Abs(ebiten.GamepadAxisValue(id, axis)-baseline) > 0.3
			}
		}
		held, active = held || padHeld, active || padHeld
	}
	for id := range m.rawNeutral {
		connected := false
		for _, current := range m.pads {
			connected = connected || id == current
		}
		if !connected {
			delete(m.rawNeutral, id)
			active = true
		}
	}
	return playerActivity{active: active, held: held}
}
