// Package autoplay supplies an input-only expert player for native validation.
// Its forecasts are private engine copies; the observed game is never modified.
package autoplay

import (
	"math"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

const horizon = 48

const (
	branchFields = 4
	beamWidth    = 16
)

// Statistics describes planning work, independent of gameplay event accounting.
type Statistics struct {
	Decisions, ForecastFields, PlannedNovas int
}

// Controller repeatedly forecasts joystick alternatives, aims at original
// targets, intercepts moving capsules and follows every surface entrance.
type Controller struct {
	Statistics Statistics
	last       [2]engine.Input
	targetID   [2]int
}

// New starts a deterministic expert player without changing game options.
func New() *Controller { return &Controller{} }

type goal struct {
	x, y, weight int
	id           int
	kind         string
}

// Next returns ordinary joystick/fire/Nova inputs for the next PAL field.
// The second field uses the original engine's preceding input latch, so only
// the first field of each pair requires a new decision.
func (c *Controller) Next(e *engine.Engine) [2]engine.Input {
	if e.Mode != engine.Playing {
		return [2]engine.Input{}
	}
	if e.Frame&1 != 0 {
		inputs := c.last
		for index, p := range e.Players {
			if !p.Active || p.Lives == 0 {
				inputs[index] = engine.Input{}
			}
		}
		return inputs
	}
	c.Statistics.Decisions++
	var goals [2]goal
	for index, p := range e.Players {
		if p.Active && p.Lives > 0 && p.Dying == 0 && p.Respawn == 0 {
			goals[index] = c.chooseGoal(e, index)
			c.targetID[index] = goals[index].id
		}
	}
	var chosen [2]engine.Input
	for index, p := range e.Players {
		if !p.Active || p.Lives == 0 || p.Dying > 0 || p.Respawn > 0 {
			continue
		}
		chosen[index] = c.plan(e, chosen, goals, index)
		if chosen[index].Nova {
			c.Statistics.PlannedNovas++
		}
	}
	for index, p := range e.Players {
		if !p.Active || p.Lives == 0 {
			chosen[index] = engine.Input{}
			continue
		}
		// Releasing during cooldown cancels the source auto-repeat delay. This
		// is an ordinary fire-button pulse, also valid on the original joystick.
		chosen[index].Fire = p.Cooldown == 0
	}
	c.last = chosen
	return chosen
}

func (c *Controller) chooseGoal(e *engine.Engine, index int) goal {
	p := e.Players[index]
	// Each entrance gets enough preparation time to cross the screen at the
	// original two-pixel speed, including the camera's moving world offset.
	if e.Stage == 0 && !e.FinalBattle() {
		for _, gate := range e.Campaign.Gates {
			if e.Campaign.ClearedMask&(1<<uint(gate.Phase)) != 0 || e.Scroll < gate.Progress-90 || gate.NativeY(e.Scroll) >= 180 {
				continue
			}
			return goal{x: clamp(gate.WorldX-e.CameraX, 0, 256), y: clamp(gate.NativeY(e.Scroll)+2, 10, 160), weight: 90, kind: "portal"}
		}
	}
	best := goal{x: p.X, y: 160, weight: 3, kind: "position"}
	bestPriority := -math.MaxFloat64
	// Capsules oscillate horizontally. Forecast their exact original motion
	// and choose a reachable interception instead of chasing their current X.
	if len(e.Pickups) != 0 {
		forecast := e.Clone()
		for field := 0; field <= 80; field++ {
			if field&3 == 0 {
				for _, pickup := range forecast.Pickups {
					x, y := clamp(pickup.X-4, 0, 256), clamp(pickup.Y-3, 2, 176)
					distance := max(abs(x-p.X), abs(y-p.Y))
					if distance > field*2+16 {
						continue
					}
					priority := 2400.0 - float64(field*10+distance*2)
					if pickup.Nova > 0 {
						priority = 1500 + float64(8-p.Nova)*100 - float64(field*10+distance*2)
					} else if p.Level < 5 {
						priority += 5500 + float64(5-p.Level)*500
					}
					if priority > bestPriority {
						bestPriority = priority
						best = goal{x: x, y: y, weight: 24, kind: "pickup"}
						if pickup.Nova == 0 && p.Level < 5 {
							best.weight = 40
						}
					}
				}
			}
			forecast.Tick([2]engine.Input{})
			c.Statistics.ForecastFields++
		}
	}
	for _, enemy := range e.Enemies {
		if x, y, width, height, collectable := e.GroundWreckBounds(enemy); collectable {
			if p.WreckBonus >= 99 {
				continue
			}
			x, y = x+width/2, y+height/2
			priority := 1700.0 - float64(abs(x-p.X)+abs(y-p.Y))*3
			if priority > bestPriority && y <= 190 {
				bestPriority = priority
				best = goal{x: clamp(x, 0, 256), y: clamp(y, 2, 176), weight: 20, id: enemy.ID, kind: "wreck"}
			}
			continue
		}
		if !e.EnemyTouchable(enemy) || enemy.Y+enemy.Definition.Height < -20 || enemy.Y >= 166 {
			continue
		}
		priority := 400.0 + float64(enemy.Definition.Score)*0.05
		if enemy.Definition.Ground {
			priority += 120 + float64(max(0, enemy.Y))*2
		}
		if enemy.Definition.FlyingPool {
			switch enemy.Definition.NativeKind {
			case 6:
				priority += 1200
				if p.Level < 5 {
					priority += 4000
				}
			case 0:
				priority += 400
			case 9:
				priority += 700
				if enemy.PoolSlot == 9 {
					priority += 400
				}
			case 2:
				priority += 700
				if enemy.PoolSlot == 9 || enemy.PoolSlot == 10 {
					priority += 300
				}
			}
		}
		x := c.aimX(e, index, enemy)
		priority -= float64(abs(x-p.X)) * 2
		if enemy.ID == c.targetID[index] {
			priority += 60
		}
		if priority > bestPriority {
			bestPriority = priority
			best = goal{x: x, y: clamp(enemy.Y+enemy.Definition.Height+48, 140, 176), weight: 5, id: enemy.ID, kind: "attack"}
			if enemy.Definition.FlyingPool && enemy.Definition.NativeKind == 6 && p.Level < 5 {
				best.weight = 18
			}
		}
	}
	return best
}

func (c *Controller) aimX(e *engine.Engine, index int, enemy engine.Enemy) int {
	p := e.Players[index]
	weapon := e.Data.Weapons[p.Weapon][p.Level]
	targetX, targetY, targetWidth, targetHeight := e.EnemyCollisionBounds(enemy)
	best, distance := targetX+targetWidth/2-16, math.MaxInt
	for _, shot := range weapon.Shots {
		if shot.VY >= 0 {
			continue
		}
		time := max(1, (p.Y+shot.Y-targetY-targetHeight/2)/-shot.VY)
		lead := 0
		if !enemy.Definition.Ground && len(enemy.Script) != 0 {
			lead = (enemy.VX * time / 2) >> 16
		}
		x := clamp(targetX+targetWidth/2+lead-shot.X-shot.VX*time-weapon.Width/2, 0, 256)
		if d := abs(x - p.X); d < distance {
			best, distance = x, d
		}
	}
	return best
}

type branch struct {
	state *engine.Engine
	first engine.Input
	risk  float64
	value float64
}

func (c *Controller) plan(e *engine.Engine, chosen [2]engine.Input, goals [2]goal, index int) engine.Input {
	beam := []branch{{state: e}}
	for depth := 0; depth < horizon/branchFields; depth++ {
		next := make([]branch, 0, beamWidth)
		for _, previous := range beam {
			p := previous.state.Players[index]
			if previous.state.Mode != engine.Playing || p.Dying > 0 {
				insertBranch(&next, previous)
				continue
			}
			novaAlternatives := 1
			if depth == 0 && p.Invulnerable == 0 && p.Nova > 0 && e.NovaFrames == 0 {
				novaAlternatives = 2
			}
			for nova := 0; nova < novaAlternatives; nova++ {
				for y := -1; y <= 1; y++ {
					for x := -1; x <= 1; x++ {
						candidate := branch{state: previous.state.Clone(), first: previous.first, risk: previous.risk}
						input := engine.Input{X: x, Y: y, Nova: nova != 0}
						if depth == 0 {
							candidate.first = input
						}
						for field := 0; field < branchFields && candidate.state.Mode == engine.Playing; field++ {
							var inputs [2]engine.Input
							for player, ship := range candidate.state.Players {
								inputs[player] = engine.Input{X: direction(goals[player].x - ship.X), Y: direction(goals[player].y - ship.Y), Fire: ship.Cooldown == 0}
								if depth == 0 && player < index {
									inputs[player].X, inputs[player].Y, inputs[player].Nova = chosen[player].X, chosen[player].Y, chosen[player].Nova && field < 2
								}
							}
							inputs[index].X, inputs[index].Y = input.X, input.Y
							inputs[index].Nova = input.Nova && field < 2
							candidate.state.Tick(inputs)
							c.Statistics.ForecastFields++
							ship := candidate.state.Players[index]
							if ship.Invulnerable == 0 && candidate.state.NovaFrames == 0 {
								candidate.risk += danger(candidate.state, index) * 0.5
							}
							if ship.Dying > 0 || ship.Lives < e.Players[index].Lives {
								candidate.risk += 10000000 - float64(depth*branchFields+field)*10000
								break
							}
						}
						candidate.value = branchValue(e, candidate.state, goals[index], index) - candidate.risk
						insertBranch(&next, candidate)
					}
				}
			}
		}
		beam = next
	}
	allDying := true
	for _, candidate := range beam {
		if candidate.state.Players[index].Dying == 0 && candidate.state.Players[index].Lives >= e.Players[index].Lives {
			allDying = false
			break
		}
	}
	if allDying {
		// A narrow beam can lose a wide detour while it favors the current
		// firing lane. Only then test sustained ordinary escape directions.
		best := beam[0]
		for y := -1; y <= 1; y++ {
			for x := -1; x <= 1; x++ {
				candidate := c.escape(e, chosen, goals, index, engine.Input{X: x, Y: y})
				if candidate.value > best.value {
					best = candidate
				}
			}
		}
		return best.first
	}
	return beam[0].first
}

func (c *Controller) escape(e *engine.Engine, chosen [2]engine.Input, goals [2]goal, index int, movement engine.Input) branch {
	candidate := branch{state: e.Clone(), first: movement}
	for field := 0; field < horizon && candidate.state.Mode == engine.Playing; field++ {
		var inputs [2]engine.Input
		for player, ship := range candidate.state.Players {
			if !ship.Active || ship.Lives == 0 {
				continue
			}
			inputs[player] = engine.Input{X: direction(goals[player].x - ship.X), Y: direction(goals[player].y - ship.Y), Fire: ship.Cooldown == 0}
			if field < branchFields && player < index {
				inputs[player].X, inputs[player].Y, inputs[player].Nova = chosen[player].X, chosen[player].Y, chosen[player].Nova && field < 2
			}
		}
		inputs[index].X, inputs[index].Y = movement.X, movement.Y
		candidate.state.Tick(inputs)
		c.Statistics.ForecastFields++
		ship := candidate.state.Players[index]
		if ship.Invulnerable == 0 && candidate.state.NovaFrames == 0 {
			candidate.risk += danger(candidate.state, index) * 0.5
		}
		if ship.Dying > 0 || ship.Lives < e.Players[index].Lives {
			candidate.risk += 10000000 - float64(field)*10000
			break
		}
	}
	candidate.value = branchValue(e, candidate.state, goals[index], index) - candidate.risk
	return candidate
}

// insertBranch keeps distinct ship/camera/resource positions. The emergency
// escape forecast handles detours lost to a crowded ordinary firing-lane beam.
func insertBranch(beam *[]branch, candidate branch) {
	p := candidate.state.Players
	for index, old := range *beam {
		q := old.state.Players
		if p[0].X == q[0].X && p[0].Y == q[0].Y && p[1].X == q[1].X && p[1].Y == q[1].Y && candidate.state.CameraX == old.state.CameraX && p[0].Nova == q[0].Nova && p[1].Nova == q[1].Nova && candidate.state.Stage == old.state.Stage {
			if old.value >= candidate.value {
				return
			}
			*beam = append((*beam)[:index], (*beam)[index+1:]...)
			break
		}
	}
	position := len(*beam)
	for index, old := range *beam {
		if candidate.value > old.value {
			position = index
			break
		}
	}
	if position >= beamWidth {
		return
	}
	*beam = append(*beam, branch{})
	copy((*beam)[position+1:], (*beam)[position:])
	(*beam)[position] = candidate
	if len(*beam) > beamWidth {
		*beam = (*beam)[:beamWidth]
	}
}

func branchValue(e, forecast *engine.Engine, g goal, index int) float64 {
	initial := e.Players[index]
	value := 0.0
	p := forecast.Players[index]
	value -= float64(abs(g.x-p.X)+abs(g.y-p.Y)) * float64(g.weight)
	value += float64(p.Score-initial.Score) * 5
	// Earned upgrades must outweigh an ordinary wreck or a capped Nova's
	// 10000-point score bonus; stronger weapons improve the remaining run.
	value += float64(p.Level-initial.Level) * 100000
	if initial.Level < 5 {
		// A carrier's guaranteed capsule appears after its six death counts,
		// often beyond this horizon. Value actual damage progress toward that
		// future weapon instead of treating its ten-point kill as ordinary loot.
		for _, carrier := range e.Enemies {
			if !carrier.Definition.FlyingPool || carrier.Definition.NativeKind != 6 || carrier.Health < 0 {
				continue
			}
			remaining, known := carrier.Health, false
			for _, enemy := range forecast.Enemies {
				if enemy.ID == carrier.ID {
					remaining, known = enemy.Health, true
					break
				}
			}
			if !known {
				for _, explosion := range forecast.Explosions {
					if explosion.WeaponCarrier && explosion.PoolSlot == carrier.PoolSlot {
						remaining, known = -1, true
						break
					}
				}
			}
			if !known {
				for _, pickup := range forecast.Pickups {
					if pickup.SlotHeld && pickup.PoolSlot == carrier.PoolSlot && pickup.Nova == 0 {
						remaining, known = -1, true
						break
					}
				}
			}
			if known {
				value += float64(max(0, carrier.Health-remaining)) * 4000
			}
		}
	}
	if p.Nova >= initial.Nova {
		value += float64(p.Nova-initial.Nova) * 7000
	} else {
		// Reserve scarce charges for escape. Ordinary scenery scores must not
		// spend every early charge before dense cave waves and boss volleys.
		cost := 75000
		if initial.Nova < 3 {
			cost = 150000
		}
		value += float64(p.Nova-initial.Nova) * float64(cost)
	}
	value += float64(p.WreckBonus-initial.WreckBonus) * 12000
	if forecast.Stage != e.Stage || forecast.Campaign.ClearedMask != e.Campaign.ClearedMask {
		value += 200000
	}
	if forecast.Mode == engine.Ending {
		value += 100000000
	}
	if p.Invulnerable == 0 && forecast.NovaFrames == 0 && g.kind != "portal" {
		// Retain lateral escape room instead of retreating into a screen edge
		// while a converging volley is still just beyond the planning horizon.
		edge := max(0, 24-p.X, p.X-232)
		value -= float64(edge * edge * 12)
	}
	return value
}

func danger(e *engine.Engine, index int) float64 {
	p := e.Players[index]
	x, y := p.X+7, p.Y+7
	penalty := 0.0
	for _, shot := range e.EnemyShots {
		penalty += proximity(x, y, 18, 16, shot.X, shot.Y, shot.Width, shot.Height, 24)
	}
	for _, enemy := range e.Enemies {
		if e.EnemyContactDangerous(enemy) {
			penalty += proximity(x, y, 18, 16, enemy.X, enemy.Y, enemy.Definition.Width, enemy.Definition.Height, 40)
		}
	}
	return penalty
}

func proximity(x, y, width, height, targetX, targetY, targetWidth, targetHeight, margin int) float64 {
	dx := max(0, targetX-x-width, x-targetX-targetWidth)
	dy := max(0, targetY-y-height, y-targetY-targetHeight)
	distance := max(dx, dy)
	if distance >= margin {
		return 0
	}
	return float64((margin - distance) * (margin - distance))
}

func direction(value int) int {
	if value < -1 {
		return -1
	}
	if value > 1 {
		return 1
	}
	return 0
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func clamp(value, low, high int) int { return max(low, min(high, value)) }
