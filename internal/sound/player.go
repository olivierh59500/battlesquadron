// Package sound natively replays the original Ron Klaren sequences and Paula samples.
package sound

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

const paulaClock = 3546895

// Assets contains the unmodified, depacked sound overlays from the supplied disk.
type Assets struct{ Game, Title, Common, Special []byte }

type layout struct {
	base, global, channel, instruments, arpeggios, periods, effects uint32
	timer                                                           uint64
	tracks                                                          uint16
	gameplay, clearInstrumentVolume, clearSlideEnvelope, end85      bool
}

var gameLayout = layout{0x246f0, 0x251f8, 0x252a4, 0x25504, 0x2545c, 0x25166, 0x2539c, 0x3101, 10, true, true, true, false}
var titleLayout = layout{0x3d800, 0x3df16, 0x3df3c, 0x3e118, 0x3e088, 0x3de84, 0x3e034, 0x2501, 1, false, true, false, true}
var commonLayout = layout{0x3d800, 0x3dee8, 0x3defe, 0x3e0ce, 0x3e04a, 0x3de56, 0x3dff6, 0x2601, 1, false, false, false, false}

// Player is an endless stereo signed-16-bit PCM stream. It translates the
// original note, envelope, vibrato, arpeggio and waveform rules directly in Go;
// it never executes any 68000 instructions. Public controls are safe to call
// while one audio consumer owns Read.
type Player struct {
	mu                              sync.Mutex
	assets                          Assets
	data                            []byte
	layout                          layout
	channels                        [4]channel
	saved                           [4]channel
	voices                          [4]voice
	sampleRate                      int
	tickPhase                       uint64
	idle, muted, effectsMuted, fade bool
	master, savedMaster, fadeCount  byte
	pending                         uint16
	restore, busy                   bool
	musicEnabled, effectsEnabled    bool
	tail                            [4]byte
	tailPosition                    int
	err                             error
}

type channel struct {
	instrument, song, list, pattern                                                                           uint32
	arpeggio                                                                                                  [12]byte
	transpose, repeat, period, target, envelopeStage, vibratoPosition, arpeggioPosition                       uint16
	note, duration, slideNote, envelopeReload, envelopeCount, vibratoDelay, waveformCount, slideSpeed, volume byte
	retrigger, attack, muted                                                                                  bool
	attackCount                                                                                               byte
}

type voice struct {
	pointer                uint32
	length, period, volume uint16
	enabled                bool
	activePointer          uint32
	activeLength           int
	position               int
	phase                  uint64
}

var _ io.Reader = (*Player)(nil)

// New starts the original gameplay music using only LODGAM.
func New(game []byte, sampleRate int) (*Player, error) {
	return NewWithAssets(Assets{Game: game}, sampleRate)
}

// NewWithAssets also makes the original title and common-overlay tunes available.
func NewWithAssets(assets Assets, sampleRate int) (*Player, error) {
	if sampleRate < 8000 || sampleRate > 192000 {
		return nil, fmt.Errorf("sound sample rate must be between 8000 and 192000 Hz")
	}
	if len(assets.Game) != 38108 {
		return nil, fmt.Errorf("LODGAM overlay has %d bytes; expected 38108", len(assets.Game))
	}
	if len(assets.Title) != 0 && len(assets.Title) != 22470 {
		return nil, fmt.Errorf("LODMUS overlay has %d bytes; expected 22470", len(assets.Title))
	}
	if len(assets.Common) != 0 && len(assets.Common) != 20218 {
		return nil, fmt.Errorf("LODCOM overlay has %d bytes; expected 20218", len(assets.Common))
	}
	if len(assets.Special) != 0 && len(assets.Special) != 12186 {
		return nil, fmt.Errorf("LODSPE overlay has %d bytes; expected 12186", len(assets.Special))
	}
	p := &Player{assets: Assets{Game: append([]byte(nil), assets.Game...), Title: append([]byte(nil), assets.Title...), Common: append([]byte(nil), assets.Common...), Special: append([]byte(nil), assets.Special...)}, sampleRate: sampleRate, tailPosition: 4, musicEnabled: true, effectsEnabled: true}
	p.install(gameLayout, p.assets.Game)
	if p.err != nil {
		return nil, p.err
	}
	return p, nil
}

// UseMenu switches to the original LODMUS title tune or back to gameplay.
func (p *Player) UseMenu(menu bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if menu && len(p.assets.Title) != 0 {
		p.install(titleLayout, p.assets.Title)
	} else {
		p.install(gameLayout, p.assets.Game)
	}
}

// UseCommon switches to the original LODCOM tune used by the common overlay.
func (p *Player) UseCommon() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.assets.Common) != 0 {
		p.install(commonLayout, p.assets.Common)
	}
}

// PlayTrack selects the original one-based song number. Invalid numbers are ignored.
func (p *Player) PlayTrack(track uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if track == 0 || track > p.layout.tracks {
		return
	}
	p.fade = false
	if p.savedMaster != 0 {
		p.master = p.savedMaster
		p.savedMaster = 0
	}
	if p.muted || p.busy {
		return
	}
	p.pending = track
	p.idle = true
	if p.layout.gameplay && track != 1 {
		p.busy = true
	}
}

// PlayEffect uses the original effect code: bits 4-5 select the Paula channel,
// and the low nibble selects one of sixteen unmodified sample descriptors.
func (p *Player) PlayEffect(code uint16) { p.mu.Lock(); defer p.mu.Unlock(); p.effect(code) }

// SetMusicEnabled pauses the original sequence while sample effects continue.
func (p *Player) SetMusicEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.musicEnabled = enabled
	p.muted = !enabled
	if !enabled {
		for i := range p.voices {
			p.voices[i].volume = 0
		}
	}
}

// SetEffectsEnabled enables or disables subsequent original sample events.
func (p *Player) SetEffectsEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.effectsEnabled = enabled
	p.effectsMuted = !enabled
}

// Fade follows the driver's original master-volume fade.
func (p *Player) Fade() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.savedMaster == 0 {
		p.savedMaster = p.master
	}
	p.fade = true
}

// Err reports a malformed original data pointer or stream command.
func (p *Player) Err() error { p.mu.Lock(); defer p.mu.Unlock(); return p.err }

// Read supplies little-endian stereo PCM, including reads that split a PCM frame.
func (p *Player) Read(out []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for n < len(out) {
		if p.err != nil {
			return n, p.err
		}
		if p.tailPosition == 4 {
			p.render(p.tail[:])
			p.tailPosition = 0
			if p.err != nil {
				return n, p.err
			}
		}
		copied := copy(out[n:], p.tail[p.tailPosition:])
		p.tailPosition += copied
		n += copied
	}
	return n, nil
}

func (p *Player) install(l layout, data []byte) {
	p.layout = l
	p.data = append(p.data[:0], data...)
	p.channels = [4]channel{}
	p.saved = [4]channel{}
	p.voices = [4]voice{}
	p.idle = p.byte(l.global) != 0
	p.master = p.byte(l.global + 1)
	p.savedMaster = 0
	p.fade = false
	p.muted = !p.musicEnabled
	p.effectsMuted = !p.effectsEnabled
	p.pending = 0
	p.restore = false
	p.busy = false
	p.fadeCount = 0
	p.err = nil
	for i := range p.channels {
		p.initialize(i, p.long(l.channel+uint32(i)*62+8))
		p.enable(i, true)
	}
	p.tickPhase = l.timer * uint64(p.sampleRate) * 5
	p.tailPosition = 4
}

func (p *Player) initialize(i int, song uint32) {
	old := p.channels[i]
	c := channel{instrument: p.layout.instruments, song: song, list: song, period: old.period, target: old.target, retrigger: old.retrigger, muted: old.muted}
	if p.layout.gameplay {
		c.retrigger = true
		c.muted = false
	}
	c.pattern = p.long(song)
	c.transpose = p.word(song + 6)
	c.repeat = p.word(song+10) - 1
	p.channels[i] = c
	p.sample(i, c.instrument)
	if p.layout.gameplay {
		p.voices[i].volume = 0
	}
}

func (p *Player) effect(code uint16) {
	if !p.effectsEnabled || p.effectsMuted || p.err != nil {
		return
	}
	i := int(code >> 4 & 3)
	c := &p.channels[i]
	if c.muted {
		return
	}
	descriptor := p.layout.effects + uint32(code&15)*12
	c.retrigger = true
	c.attack = true
	c.attackCount = p.byte(descriptor + 11)
	p.enable(i, false)
	v := &p.voices[i]
	v.pointer = p.long(descriptor)
	v.length = p.word(descriptor + 4)
	v.period = p.word(descriptor + 6)
	v.volume = p.word(descriptor + 8)
}

func (p *Player) enable(i int, enabled bool) {
	v := &p.voices[i]
	if enabled && !v.enabled {
		v.activePointer = v.pointer &^ 1
		v.activeLength = lengthBytes(v.length)
		v.position = 0
		v.phase = 0
	}
	v.enabled = enabled
}

func lengthBytes(length uint16) int {
	if length == 0 {
		return 131072
	}
	return int(length) * 2
}
func (p *Player) sample(i int, instrument uint32) {
	descriptor := p.long(instrument)
	p.voices[i].pointer = p.long(descriptor)
	p.voices[i].length = p.word(descriptor + 4)
}

func (p *Player) render(out []byte) {
	threshold := p.layout.timer * uint64(p.sampleRate) * 5
	if p.tickPhase >= threshold {
		p.tickPhase -= threshold
		p.tick()
	}
	p.tickPhase += paulaClock
	var left, right int
	for i := range p.voices {
		v := &p.voices[i]
		if !v.enabled || v.period == 0 || v.activeLength == 0 {
			continue
		}
		// The clock remainder follows Paula's sample countdown even when a note
		// changes pitch. Location and length reload only at the block boundary.
		period := uint64(v.period) * uint64(p.sampleRate)
		address := v.activePointer + uint32(v.position)
		amplitude := 0
		// The menu's silent instrument points at a shared two-byte dummy block
		// outside this overlay. A zero-volume DMA word has no audible payload.
		if v.volume != 0 {
			amplitude = int(int8(p.byte(address))) * int(min(v.volume, 64)) * 2
		}
		if !p.musicEnabled && !p.channels[i].attack {
			amplitude = 0
		}
		if i == 0 || i == 3 {
			left += amplitude
		} else {
			right += amplitude
		}
		v.phase += paulaClock
		for v.phase >= period {
			v.phase -= period
			v.position++
			if v.position >= v.activeLength {
				v.position = 0
				v.activePointer = v.pointer &^ 1
				v.activeLength = lengthBytes(v.length)
			}
		}
	}
	binary.LittleEndian.PutUint16(out, uint16(int16(max(-32768, min(32767, left)))))
	binary.LittleEndian.PutUint16(out[2:], uint16(int16(max(-32768, min(32767, right)))))
}

func (p *Player) byte(address uint32) byte {
	at := int64(address) - int64(p.layout.base)
	if at < 0 || at >= int64(len(p.data)) {
		if !p.layout.gameplay && address >= 0x246f0 && uint64(address-0x246f0) < uint64(len(p.assets.Special)) {
			return p.assets.Special[address-0x246f0]
		}
		p.fail("original sound pointer $%X is outside its overlay", address)
		return 0
	}
	return p.data[at]
}
func (p *Player) word(address uint32) uint16 {
	return uint16(p.byte(address))<<8 | uint16(p.byte(address+1))
}
func (p *Player) long(address uint32) uint32 {
	return uint32(p.word(address))<<16 | uint32(p.word(address+2))
}
func (p *Player) put(address uint32, value byte) {
	at := int64(address) - int64(p.layout.base)
	if at < 0 || at >= int64(len(p.data)) {
		p.fail("original waveform pointer $%X is outside its overlay", address)
		return
	}
	p.data[at] = value
}
func (p *Player) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
}
