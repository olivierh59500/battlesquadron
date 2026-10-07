package performance

import (
	"testing"
	"time"
)

func TestDistributionUsesNearestRankAndIncludesSlowFrames(t *testing.T) {
	samples := make([]time.Duration, 100)
	for index := range samples {
		samples[index] = time.Duration(index+1) * time.Millisecond
	}
	got := summarize(samples)
	if got.Samples != 100 || got.MeanMS != 50.5 || got.P50MS != 50 || got.P95MS != 95 || got.P99MS != 99 || got.MaxMS != 100 {
		t.Fatalf("unexpected distribution: %+v", got)
	}
	if samples[0] != time.Millisecond {
		t.Fatal("reporting mutated the recorded frame order")
	}
}

func TestWarmupIsExcludedAndDrawDoesNotAdvanceUpdates(t *testing.T) {
	m := New(time.Hour)
	start := time.Now()
	m.RecordDraw(start)
	m.RecordUpdate(start, 1, 2, 3, 4)
	if m.drawCount != 0 || m.updateCount != 0 {
		t.Fatal("warmup entered the measured interval")
	}
	m.ready = start
	m.RecordDraw(start)
	m.RecordDraw(start.Add(time.Second / 60))
	if m.drawCount != 2 || m.updateCount != 0 || len(m.intervals) != 1 {
		t.Fatal("rendering changed the independent simulation cadence")
	}
	m.RecordUpdate(start, 1, 2, 3, 4)
	if m.updateCount != 1 || m.maxEnemies != 1 || m.maxEffects != 4 {
		t.Fatal("update workload was not recorded")
	}
}
