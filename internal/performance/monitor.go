// Package performance measures the actual presentation loop without changing game time.
package performance

import (
	"runtime"
	"slices"
	"time"
)

const sampleLimit = 18000

// Distribution reports measured CPU work or frame intervals in milliseconds.
type Distribution struct {
	Samples int     `json:"samples"`
	MeanMS  float64 `json:"mean_ms"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
	P99MS   float64 `json:"p99_ms"`
	MaxMS   float64 `json:"max_ms"`
}

// Report separates presentation cadence, CPU submission and process allocation.
// Draw timing does not include GPU completion; intervals include driver stalls.
type Report struct {
	Platform              string       `json:"platform"`
	TargetFPS             int          `json:"target_fps"`
	SimulationTPS         int          `json:"simulation_tps"`
	ElapsedSeconds        float64      `json:"elapsed_seconds"`
	Draws                 int          `json:"draws"`
	Updates               int          `json:"updates"`
	MeasuredFPS           float64      `json:"measured_fps"`
	MeasuredTPS           float64      `json:"measured_tps"`
	EngineFPS             float64      `json:"ebitengine_fps"`
	EngineTPS             float64      `json:"ebitengine_tps"`
	FrameIntervals        Distribution `json:"frame_intervals"`
	DrawCPU               Distribution `json:"draw_cpu"`
	UpdateCPU             Distribution `json:"update_cpu"`
	AllocationsPerDraw    float64      `json:"process_allocations_per_draw"`
	AllocatedBytesPerDraw float64      `json:"process_allocated_bytes_per_draw"`
	HeapBytes             uint64       `json:"heap_bytes"`
	GCCycles              uint32       `json:"gc_cycles"`
	GCPauseMS             float64      `json:"gc_pause_ms"`
	MaxEnemies            int          `json:"max_enemies"`
	MaxPlayerShots        int          `json:"max_player_shots"`
	MaxEnemyShots         int          `json:"max_enemy_shots"`
	MaxEffects            int          `json:"max_effects"`
}

// Monitor is owned by the sequential Update/Draw thread, like the game state.
// Its bounded buffers are allocated once, outside the measured interval.
type Monitor struct {
	ready, firstDraw, lastDraw time.Time
	intervals, draws, updates  []time.Duration
	drawCount, updateCount     int
	baseline                   runtime.MemStats
	maxEnemies, maxPlayerShots int
	maxEnemyShots, maxEffects  int
}

// New excludes asset/audio upload and scheduler stabilization during warmup.
func New(warmup time.Duration) *Monitor {
	return &Monitor{ready: time.Now().Add(warmup), intervals: make([]time.Duration, 0, sampleLimit), draws: make([]time.Duration, 0, sampleLimit), updates: make([]time.Duration, 0, sampleLimit)}
}

// RecordUpdate measures one unchanged PAL update.
func (m *Monitor) RecordUpdate(start time.Time, enemies, playerShots, enemyShots, effects int) {
	if m.firstDraw.IsZero() {
		return
	}
	m.updateCount++
	if len(m.updates) < sampleLimit {
		m.updates = append(m.updates, time.Since(start))
	}
	m.maxEnemies = max(m.maxEnemies, enemies)
	m.maxPlayerShots = max(m.maxPlayerShots, playerShots)
	m.maxEnemyShots = max(m.maxEnemyShots, enemyShots)
	m.maxEffects = max(m.maxEffects, effects)
}

// RecordDraw measures submitted Go drawing work and successive Draw starts.
func (m *Monitor) RecordDraw(start time.Time) {
	if start.Before(m.ready) {
		return
	}
	if m.firstDraw.IsZero() {
		m.firstDraw = start
		runtime.ReadMemStats(&m.baseline)
	} else if len(m.intervals) < sampleLimit {
		m.intervals = append(m.intervals, start.Sub(m.lastDraw))
	}
	m.lastDraw = start
	m.drawCount++
	if len(m.draws) < sampleLimit {
		m.draws = append(m.draws, time.Since(start))
	}
}

// Snapshot is intended for the end of a run, outside normal rendering.
func (m *Monitor) Snapshot(engineFPS, engineTPS float64) Report {
	r := Report{Platform: runtime.GOOS + "/" + runtime.GOARCH, TargetFPS: 60, SimulationTPS: 50, Draws: m.drawCount, Updates: m.updateCount, EngineFPS: engineFPS, EngineTPS: engineTPS,
		FrameIntervals: summarize(m.intervals), DrawCPU: summarize(m.draws), UpdateCPU: summarize(m.updates), MaxEnemies: m.maxEnemies, MaxPlayerShots: m.maxPlayerShots, MaxEnemyShots: m.maxEnemyShots, MaxEffects: m.maxEffects}
	if m.firstDraw.IsZero() {
		return r
	}
	r.ElapsedSeconds = m.lastDraw.Sub(m.firstDraw).Seconds()
	if r.ElapsedSeconds > 0 {
		r.MeasuredFPS = float64(m.drawCount-1) / r.ElapsedSeconds
		r.MeasuredTPS = float64(m.updateCount) / r.ElapsedSeconds
	}
	var current runtime.MemStats
	runtime.ReadMemStats(&current)
	r.HeapBytes, r.GCCycles = current.HeapAlloc, current.NumGC-m.baseline.NumGC
	r.GCPauseMS = float64(current.PauseTotalNs-m.baseline.PauseTotalNs) / 1e6
	if m.drawCount > 0 {
		r.AllocationsPerDraw = float64(current.Mallocs-m.baseline.Mallocs) / float64(m.drawCount)
		r.AllocatedBytesPerDraw = float64(current.TotalAlloc-m.baseline.TotalAlloc) / float64(m.drawCount)
	}
	return r
}

func summarize(samples []time.Duration) Distribution {
	if len(samples) == 0 {
		return Distribution{}
	}
	ordered := slices.Clone(samples)
	slices.Sort(ordered)
	var total time.Duration
	for _, sample := range ordered {
		total += sample
	}
	percentile := func(percent int) float64 {
		index := (len(ordered)*percent+99)/100 - 1
		return float64(ordered[index]) / float64(time.Millisecond)
	}
	return Distribution{Samples: len(ordered), MeanMS: float64(total) / float64(len(ordered)) / float64(time.Millisecond), P50MS: percentile(50), P95MS: percentile(95), P99MS: percentile(99), MaxMS: float64(ordered[len(ordered)-1]) / float64(time.Millisecond)}
}
