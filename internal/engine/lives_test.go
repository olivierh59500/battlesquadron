package engine

import "testing"

func TestOriginalExtraLifeDigitTransitions(t *testing.T) {
	for _, c := range []struct {
		previous, current int
		award             bool
	}{
		{99999, 100010, true}, {199999, 200005, false}, {299999, 300001, true}, {599999, 600020, true},
		{90000, 310000, false}, {999999, 1000000, true}, {1999999, 2000020, true},
		{9999999, 10000000, true}, {99999999, 0, false}, {100000, 100500, false},
	} {
		if got := OriginalExtraLife(c.previous, c.current); got != c.award {
			t.Errorf("source digits %d->%d award=%t;want%t", c.previous, c.current, got, c.award)
		}
	}
}

func TestOriginalExtraLifeBankAndSpareCap(t *testing.T) {
	e := newFixture(t)
	e.Frame = 1
	e.Players[0].lastLifeScore = 90000
	e.Players[0].Score = 100000
	e.awardOriginalLife()
	if e.Players[0].Lives != 4 {
		t.Fatal("source threshold did not grant a ship")
	}
	e.Players[0].Lives = 5
	e.Players[0].lastLifeScore = 290000
	e.Players[0].Score = 300000
	e.awardOriginalLife()
	if e.Players[0].Lives != 5 || e.Players[0].lastLifeScore != 300000 {
		t.Fatal("spare cap or sampled digits changed")
	}
}
