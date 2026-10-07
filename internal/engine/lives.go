package engine

// OriginalExtraLife reproduces the ASCII-digit comparisons in loader $1340.
// Crossing a tier alone is insufficient: its preceding sampled digit must be
// the one checked by the original, including its million-digit wraparound.
func OriginalExtraLife(previous, current int) bool {
	previous = (previous%100000000 + 100000000) % 100000000
	current = (current%100000000 + 100000000) % 100000000
	if current >= 1000000 {
		return current/1000000%10 != previous/1000000%10
	}
	before, after := previous/100000%10, current/100000%10
	return after == 1 && before == 0 || after == 3 && before == 2 || after == 6 && before == 5
}

// PlayerLifeSample exposes the original routine's read-only score sample and
// spare byte for development comparisons. Runtime ship counts include the live
// ship; original ordinary-entry stock counts the arriving ship instead.
func (e *Engine) PlayerLifeSample(index int) (previousScore, spare int) {
	if index < 0 || index >= len(e.Players) {
		return 0, 0
	}
	p := e.Players[index]
	spare = p.Lives
	if p.Active && p.Lives > 0 && (p.Respawn == 0 || p.entryKeepsShip) {
		spare--
	}
	return p.lastLifeScore, spare
}

// awardOriginalLife samples one player bank at the original ordinary-loop edge.
func (e *Engine) awardOriginalLife() {
	index := e.nativeClock() >> 2 & 1
	p := &e.Players[index]
	if !p.Active {
		return
	}
	award := OriginalExtraLife(p.lastLifeScore, p.Score)
	p.lastLifeScore = p.Score
	// The source caps its spare-ship byte at four. Ordinary initial/death
	// entry includes the arriving ship in that byte. Cave return's byte+$29
	// preserves the existing live ship separately throughout its entry animation.
	limit := 5
	if p.Respawn > 0 && !p.entryKeepsShip {
		limit = 4
	}
	if award && p.Lives < limit && (p.Lives > 0 || p.Dying > 0) {
		p.Lives++
		e.Events = append(e.Events, Event{Kind: "extra-life", Player: index})
	}
}
