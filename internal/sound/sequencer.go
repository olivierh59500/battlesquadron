package sound

// tick translates the three original CIA timer handlers. Gameplay can borrow
// the channel bank for a short song and restore it when command $83 finishes.
func (p *Player) tick() {
	if !p.idle {
		if p.fade {
			if p.master == 0 {
				p.idle = true
				p.master = p.savedMaster
				p.savedMaster = 0
				p.fade = false
				for i := range p.voices {
					p.voices[i].volume = 0
				}
				return
			}
			if p.layout.gameplay {
				p.fadeCount--
				if int8(p.fadeCount) < 0 {
					p.fadeCount = 2
					p.master--
				}
			} else {
				p.fadeCount ^= 1
				if p.fadeCount == 0 {
					p.master--
				}
			}
		}
		for i := range p.channels {
			p.update(i)
		}
		return
	}
	if p.pending != 0 {
		if p.layout.gameplay {
			p.saved = p.channels
		}
		offset := uint32(6)
		if p.layout.gameplay {
			offset = 12
		}
		songs := p.layout.global + offset + uint32(p.pending-1)*16
		for i := range p.channels {
			p.enable(i, false)
			p.initialize(i, p.long(songs+uint32(i)*4))
		}
		p.idle = false
		p.pending = 0
		return
	}
	if p.restore && p.layout.gameplay {
		p.channels = p.saved
		for i := range p.channels {
			c := &p.channels[i]
			p.enable(i, false)
			p.sample(i, c.instrument)
			p.voices[i].period = c.period
			p.voices[i].volume = uint16(c.volume)
		}
		p.restore = false
		p.busy = false
		p.idle = false
	}
}

// update follows LODGAM $24856, LODMUS $3D8D2 and LODCOM $3D8B4. Their
// shared data format differs in three command side effects and attack handling.
func (p *Player) update(i int) {
	c := &p.channels[i]
	v := &p.voices[i]
	if !c.muted {
		if c.attack {
			if c.attackCount != 0 {
				c.attackCount--
			} else {
				c.attack = false
				if !p.muted {
					if p.layout.gameplay {
						c.retrigger = p.byte(c.instrument+8) != 0
					}
					p.sample(i, c.instrument)
				}
			}
		}
		p.enable(i, true)
		if c.retrigger {
			c.retrigger = false
			v.length = 1
		}
	}
	if p.muted {
		return
	}
	if c.duration != 0 {
		c.duration--
		if c.attack {
			return
		}
		p.waveform(c)
		if c.slideSpeed != 0 {
			if int16(c.target-c.period) < 0 {
				c.period -= uint16(c.slideSpeed)
				if int16(c.target-c.period) >= 0 {
					c.period = c.target
				}
			} else {
				c.period += uint16(c.slideSpeed)
				if int16(c.target-c.period) < 0 {
					c.period = c.target
				}
			}
		} else {
			note := uint16(c.arpeggio[c.arpeggioPosition]+c.note) + c.transpose
			c.period = p.pitch(note)
			c.arpeggioPosition--
			if int16(c.arpeggioPosition) < 0 {
				c.arpeggioPosition += 12
			}
		}
		if !c.muted {
			v.period = c.period
		}
		if speed := p.byte(c.instrument + 11); speed != 0 {
			if c.vibratoDelay != 0 {
				c.vibratoDelay--
			} else {
				descriptor := p.long(c.instrument + 4)
				value := int16(int8(p.byte(p.long(descriptor) + uint32(c.vibratoPosition))))
				modulation := value * int16(p.byte(c.instrument+12))
				if !c.muted {
					v.period = uint16(modulation) + c.period
				}
				c.vibratoPosition -= uint16(speed)
				if int16(c.vibratoPosition) < 0 {
					c.vibratoPosition = p.word(descriptor+4) * 2
				}
			}
		}
		p.envelope(i)
		return
	}
	c.slideSpeed = 0
	if delay := p.byte(c.instrument + 13); delay != 0 {
		c.vibratoDelay = (delay << 2) - 1
	}
	p.sequence(i)
}

// waveform retains the driver's byte subtraction and in-place sample mutation.
// These are original synthesizer waveforms, never replacement sound assets.
func (p *Player) waveform(c *channel) {
	instrument := c.instrument
	speed := p.byte(instrument + 9)
	if speed == 0 {
		return
	}
	if c.waveformCount != 0 {
		c.waveformCount--
		return
	}
	c.waveformCount = speed - 1
	descriptor := p.long(instrument)
	length := p.word(descriptor + 6)
	length = length&0xff00 | uint16(byte(length)-p.byte(instrument+10))
	wave := uint32(int64(p.long(descriptor)) + int64(int16(length)))
	position := p.byte(instrument + 24)
	p.put(wave+uint32(position), p.byte(instrument+22))
	flip := false
	if int8(p.byte(instrument+23)) >= 0 {
		position++
		flip = position == p.byte(instrument+10)*2
	} else {
		position--
		flip = position == 0
	}
	if flip {
		p.put(instrument+23, ^p.byte(instrument+23))
		p.put(instrument+22, ^p.byte(instrument+22))
	}
	p.put(instrument+24, position)
}

func (p *Player) envelope(i int) {
	c := &p.channels[i]
	c.envelopeCount--
	if int8(c.envelopeCount) >= 0 {
		return
	}
	c.envelopeCount = c.envelopeReload
	level := p.byte(c.instrument + 14 + uint32(c.envelopeStage))
	rate := p.byte(c.instrument + 18 + uint32(c.envelopeStage))
	advance := false
	if int8(level-c.volume) < 0 {
		c.volume -= rate
		if int8(level-c.volume) >= 0 {
			c.volume = level
			advance = true
		}
	} else {
		c.volume += rate
		if int8(c.volume-p.master) >= 0 {
			c.volume = p.master
			advance = true
		} else if int8(level-c.volume) < 0 {
			c.volume = level
			advance = true
		}
	}
	if advance && c.envelopeStage != 3 {
		c.envelopeStage++
	}
	if !c.muted {
		p.voices[i].volume = uint16(c.volume & 63)
	}
}

func (p *Player) pitch(note uint16) uint16 {
	// The driver's indexed word addressing sign-extends its 16-bit displacement.
	displacement := int16(note * 2)
	return p.word(uint32(int64(p.layout.periods) + int64(displacement)))
}

// sequence processes commands in the original ordered pass. A duration-zero
// note retunes and continues reading in this same tick; it does not wait for
// another timer interrupt. Unknown negative bytes end the current pattern.
func (p *Player) sequence(i int) {
	c := &p.channels[i]
	v := &p.voices[i]
	cursor := c.pattern
	for guard := 0; guard < 1024 && p.err == nil; guard++ {
		if p.byte(cursor) == 0x80 {
			c.arpeggioPosition = 0
			entry := p.layout.arpeggios + uint32(p.byte(cursor+1))*12
			for j := range c.arpeggio {
				c.arpeggio[j] = p.byte(entry + uint32(j))
			}
			cursor += 2
		}
		if p.byte(cursor) == 0x81 {
			if p.layout.clearSlideEnvelope {
				c.envelopeStage = 0
			}
			c.slideNote = p.byte(cursor + 1)
			c.target = p.pitch(uint16(c.slideNote) + c.transpose)
			c.slideSpeed = p.byte(cursor + 2)
			c.duration = (p.byte(cursor+3) << 2) - 1
			cursor += 4
			c.pattern = cursor
			if !c.muted {
				v.period = c.period
			}
			return
		}
		if p.byte(cursor) == 0x82 {
			if p.layout.clearInstrumentVolume {
				c.volume = 0
			}
			c.envelopeStage = 0
			c.instrument = p.layout.instruments + uint32(p.byte(cursor+1))*32
			if !c.muted && !c.attack {
				p.enable(i, false)
				p.sample(i, c.instrument)
			}
			cursor += 2
		}
		if p.byte(cursor) == 0x83 {
			p.idle = true
			if p.layout.gameplay {
				p.pending = 0
				p.restore = true
			}
			return
		}
		if p.byte(cursor) == 0x84 {
			c.envelopeReload = p.byte(cursor + 1)
			cursor += 2
		}
		if p.layout.end85 && p.byte(cursor) == 0x85 {
			p.idle = true
			return
		}
		if int8(p.byte(cursor)) < 0 {
			if c.repeat != 0 {
				c.repeat--
			} else {
				c.list += 12
				if int8(p.byte(c.list)) < 0 {
					c.list = c.song
				}
				c.repeat = p.word(c.list+10) - 1
			}
			cursor = p.long(c.list)
			c.pattern = cursor
			c.transpose = p.word(c.list + 6)
			continue
		}
		c.note = p.byte(cursor)
		c.period = p.pitch(uint16(c.note) + c.transpose)
		duration := p.byte(cursor + 1)
		cursor += 2
		if duration == 0 {
			continue
		}
		c.duration = (duration << 2) - 1
		c.envelopeStage = 0
		c.pattern = cursor
		if c.muted || c.attack {
			return
		}
		p.enable(i, false)
		p.sample(i, c.instrument)
		if p.byte(c.instrument+8) != 0 {
			c.retrigger = true
		}
		v.period = c.period
		return
	}
	p.fail("original sound command stream at $%X did not reach a note", cursor)
}
