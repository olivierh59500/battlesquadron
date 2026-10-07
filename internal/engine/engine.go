package engine

import (
	"fmt"
	"math"
	"slices"
)

// Mode identifies the native title, gameplay, game-over and ending screens.
type Mode uint8

const (
	Title Mode = iota
	Playing
	GameOver
	Ending
)

// Player preserves the state of one ship, including both original fire timers.
type Player struct {
	X, Y, Tilt, Lives, Weapon, Level, Nova int
	Score, Invulnerable, Dying, Respawn    int
	Cooldown, Repeat                       int
	Active                                 bool
	WreckBonus                             int
	novaHeld                               bool
	lastLifeScore                          int
	entryKeepsShip                         bool
}

// Enemy is one decoded object in the original eighteen-entry scenery pool.
type Enemy struct {
	ID, X, Y, VX, VY, Frame, Health, Age, FireTimer int
	PoolSlot                                        int
	Definition                                      Definition
	Script                                          []Motion
	scriptIndex, scriptTicks, fixedX, fixedY        int
	scriptLoop                                      int
	repeatScript                                    bool
	ground                                          nativeGroundState
	native                                          nativeAdaptiveState
	tracker                                         nativeTrackerState
	special                                         nativeSpecialState
}

// Bullet is one original primary projectile or one hostile fixed-point shot.
type Bullet struct {
	Nova                                       bool
	X, Y, VX, VY, Width, Height, Damage, Delay int
	Graphic                                    uint8
	Player, Slot, Age                          int
	fixedX, fixedY                             int
	originX, originY                           int
}

// Pickup is a weapon or Nova charge released by a decoded original object.
type Pickup struct {
	X, Y, Weapon, Nova, Age int
	Frame                   int
	Graphic                 uint8
	Sprite                  string
	PoolSlot                int
	SlotHeld                bool
	native                  nativePickupState
}

// Explosion is a timed animation; the renderer selects its original frames.
type Explosion struct {
	X, Y, Age, Duration int
	Frame               int
	Native, Formation   bool
	WeaponCarrier       bool
	SpriteKind          int
	PoolSlot            int
	nativeCount         byte
	Player              bool
}

// Event gives the presentation an audio or transition notification for this tick.
type Event struct {
	Kind   string
	Player int
	Value  int
}

// Engine owns deterministic game state and never calls Ebitengine or Android.
type Engine struct {
	Data                 *Data
	Options              Options
	Mode                 Mode
	Frame, Stage, Scroll int
	CameraX              int
	finalBattle          bool
	initialSurfaceEvents []Spawn
	Players              [2]Player
	Enemies              []Enemy
	PlayerShots          []Bullet
	EnemyShots           []Bullet
	Pickups              []Pickup
	Explosions           []Explosion
	Events               []Event
	NovaFrames           int
	Campaign             Campaign
	NovaOwner            int
	NovaRays             []NovaRay
	novaIndex            int
	npcPhase             bool
	groundSpawns         []Spawn
	latchedInputs        [2]Input
	nextEvent, nextID    int
	randomCursor         int
	modeFrames           int
	bossSpawned          bool
}

// New validates the supplied original data and starts on the title screen.
func New(data *Data) (*Engine, error) {
	if data == nil {
		return nil, fmt.Errorf("original game data is required")
	}
	copyData := *data
	copyData.Stages = slices.Clone(data.Stages)
	if len(copyData.Loader) != 0 {
		var err error
		copyData.Weapons, copyData.WeaponTable, err = DecodeWeapons(copyData.Loader, copyData.LoaderBase, copyData.WeaponTable)
		if err != nil {
			return nil, err
		}
		if len(copyData.MapRules) == 0 {
			copyData.MapRules = DecodeMapRules(copyData.Loader, copyData.LoaderBase)
		}
	}
	for weapon := range copyData.Weapons {
		for level := range copyData.Weapons[weapon] {
			if len(copyData.Weapons[weapon][level].Shots) == 0 {
				return nil, fmt.Errorf("original weapon %d level %d is missing", weapon, level)
			}
		}
	}
	for index := range copyData.Stages {
		stage := &copyData.Stages[index]
		stage.Events = slices.Clone(stage.Events)
		slices.SortStableFunc(stage.Events, func(a, b Spawn) int { return a.Progress - b.Progress })
		if stage.Width == 0 {
			stage.Width = 24
		}
		if stage.Height == 0 && len(stage.Tiles) != 0 {
			stage.Height = len(stage.Tiles) / stage.Width * 16
		}
	}
	options := data.Options
	if options.Lives == 0 {
		options = DefaultOptions()
	}
	options.Lives = max(3, min(5, options.Lives))
	options.StartWeapon = max(0, min(3, options.StartWeapon))
	options.EnemyProjectileCap = max(1, min(12, options.EnemyProjectileCap))
	options.EnemyProjectileSpeed = max(1, min(5, options.EnemyProjectileSpeed))
	options.EnemyFireDelay = max(8, options.EnemyFireDelay)
	var campaign Campaign
	if len(copyData.Loader) != 0 {
		gates, err := DecodeSurfaceGates(copyData.Loader, copyData.LoaderBase)
		if err != nil {
			return nil, err
		}
		campaign = NewCampaign(gates)
		for index, address := range []uint32{0x4da2, 0x4eac} {
			word, err := (memory{copyData.Loader, copyData.LoaderBase}).at(address+54, 2)
			if err != nil {
				return nil, err
			}
			copyData.PlayerSpawnX[index] = int(word[0])<<8 | int(word[1])
			copyData.PlayerSpawnX[index] -= 256
		}
	}
	if copyData.PlayerSpawnX == ([2]int{}) {
		copyData.PlayerSpawnX = [2]int{112, 160}
	}
	return &Engine{Data: &copyData, Options: options, Mode: Title, CameraX: 48, Campaign: campaign}, nil
}

// Start resets both pools while retaining the selected game options.
func (e *Engine) Start(players int) {
	if e.finalBattle && len(e.Data.Stages) != 0 {
		e.Data.Stages[0].Events = e.initialSurfaceEvents
	}
	e.finalBattle = false
	e.Campaign = NewCampaign(e.Campaign.Gates)
	e.NovaRays, e.novaIndex, e.NovaOwner = nil, 0, 0
	e.Mode, e.Frame, e.Scroll, e.Stage, e.modeFrames = Playing, 0, 160, 0, 0
	e.nextEvent, e.nextID, e.randomCursor, e.NovaFrames = 0, 0, 0, 0
	e.bossSpawned = false
	e.Enemies, e.PlayerShots, e.EnemyShots, e.Pickups, e.Explosions, e.Events = nil, nil, nil, nil, nil, nil
	for index := range e.Players {
		// The fresh-game initializer differs from the later death respawn.
		e.Players[index] = Player{X: e.Data.PlayerSpawnX[index], Y: 208, Tilt: 3, Lives: e.Options.Lives,
			Weapon: max(0, min(3, e.Options.StartWeapon)), Nova: 3, Active: index < players,
			Invulnerable: 360, Respawn: 130}
	}
}

// ReturnToTitle abandons the current session and releases the gameplay pools.
func (e *Engine) ReturnToTitle() {
	e.NovaRays, e.NovaFrames = nil, 0
	e.Mode, e.modeFrames = Title, 0
	e.Enemies, e.PlayerShots, e.EnemyShots, e.Pickups, e.Explosions = nil, nil, nil, nil, nil
}

// Tick advances exactly one original PAL gameplay update.
func (e *Engine) Tick(inputs [2]Input) {
	e.Events = e.Events[:0]
	if e.Mode != Playing {
		if e.Mode == Ending && e.modeFrames < 100 {
			e.ApplyOriginalEndingBonusTick()
		}
		e.modeFrames++
		return
	}
	e.Frame++
	firstHalf := e.Frame&1 != 0
	if firstHalf {
		e.latchedInputs = inputs
		e.npcPhase = true
		e.updateCamera()
		e.updateEnemies()
		e.updatePickups()
		e.npcPhase = false
	}
	if e.NovaFrames > 0 && e.Data.Nova == nil {
		e.NovaFrames--
	}
	for index := range e.Players {
		e.updatePlayer(index, e.latchedInputs[index])
	}
	e.updatePlayerShots()
	e.updateEnemyShots()
	e.updateExplosions()
	e.updateNova()
	if firstHalf {
		e.collisions()
		e.awardOriginalLife()
		e.npcPhase = true
		e.advanceStage()
		e.npcPhase = false
	}
	living := false
	for _, player := range e.Players {
		living = living || player.Active && (player.Lives > 0 || player.Dying > 0)
	}
	if !living {
		e.Mode, e.modeFrames = GameOver, 0
		e.Events = append(e.Events, Event{Kind: "game-over"})
	}
}

func (e *Engine) updatePlayer(index int, input Input) {
	p := &e.Players[index]
	if !p.Active || p.Lives == 0 {
		return
	}
	if p.Invulnerable > 0 {
		p.Invulnerable--
	}
	if p.Dying > 0 {
		p.Dying--
		if p.Dying == 0 {
			p.Lives--
			if p.Lives > 0 {
				p.entryKeepsShip = false
				// Original $530A/$5344 resets Nova and halves the weapon level.
				p.Level /= 2
				p.Nova, p.Invulnerable, p.Respawn = 3, 300, 145
				p.X, p.Y, p.Tilt = e.Data.PlayerSpawnX[index], 256, 3
				p.Cooldown, p.Repeat = 0, 0
			}
		}
		return
	}
	if p.Respawn > 0 {
		p.Respawn--
		// Actual $5026 selects Up during the last 45 fields, including field zero.
		if p.Respawn < 45 && p.Y > 2 {
			p.Y -= 2
		}
		return
	}
	// Original movement tests each axis independently; diagonals retain full speed.
	x, y := clampDirection(input.X), clampDirection(input.Y)
	p.X, p.Y = max(0, min(256, p.X+x*2)), max(2, min(176, p.Y+y*2))
	if e.Frame%4 == 0 {
		if x != 0 {
			p.Tilt = max(0, min(6, p.Tilt+x))
		} else if p.Tilt < 3 {
			p.Tilt++
		} else if p.Tilt > 3 {
			p.Tilt--
		}
	}
	if e.Frame&1 == 0 {
		return
	}
	if input.Nova && !p.novaHeld && p.Nova > 0 && e.NovaFrames == 0 {
		p.Nova--
		e.NovaFrames = 50
		if e.Data.Nova != nil {
			e.NovaFrames = 255
			e.NovaOwner = index
			e.novaIndex = 0
		}
		e.EnemyShots = e.EnemyShots[:0]
		if e.Data.Nova == nil {
			for enemyIndex := range e.Enemies {
				e.damageEnemy(enemyIndex, 2, index)
			}
		}
		e.Events = append(e.Events, Event{Kind: "nova", Player: index})
	}
	p.novaHeld = input.Nova
	// The timer branch returns before the auto-repeat branch in the original.
	if p.Cooldown > 0 {
		p.Cooldown--
		if p.Repeat > 0 {
			p.Repeat--
		}
		if !input.Fire {
			p.Repeat = 0
		}
		return
	}
	if !input.Fire {
		p.Repeat = 0
		return
	}
	if p.Repeat > 0 {
		p.Repeat--
		return
	}
	if e.NovaFrames == 0 || e.Data.Nova != nil && e.NovaOwner != index {
		e.fire(index)
	}
}

func (e *Engine) fire(index int) {
	p := &e.Players[index]
	weapon := e.Data.Weapons[p.Weapon][p.Level]
	var slots [PlayerShotLimit]Bullet
	var occupied [PlayerShotLimit]bool
	primary, previous := 0, 0
	for _, bullet := range e.PlayerShots {
		if bullet.Player == index {
			if bullet.Slot < 0 || bullet.Slot >= PlayerShotLimit {
				continue
			}
			slots[bullet.Slot], occupied[bullet.Slot] = bullet, true
			if weapon.Slots[bullet.Slot] == 0 {
				primary++
			} else if weapon.Slots[bullet.Slot] > 0 {
				previous++
			}
		}
	}
	// Original $3FF8/$4014 preserves the previous bank until it leaves the screen.
	if primary > 0 && previous > 0 {
		return
	}
	if primary > 0 && weapon.BankSize > 0 && weapon.BankSize*2 <= PlayerShotLimit {
		copy(slots[:weapon.BankSize], slots[weapon.BankSize:weapon.BankSize*2])
		copy(occupied[:weapon.BankSize], occupied[weapon.BankSize:weapon.BankSize*2])
		if weapon.Shift > 0 {
			copy(slots[weapon.Shift:], slots[:PlayerShotLimit-weapon.Shift])
			copy(occupied[weapon.Shift:], occupied[:PlayerShotLimit-weapon.Shift])
		}
	}
	shotIndex := 0
	for slot, role := range weapon.Slots {
		if role != 0 || shotIndex >= len(weapon.Shots) {
			continue
		}
		template := weapon.Shots[shotIndex]
		shotIndex++
		slots[slot] = Bullet{
			X: p.X + template.X, Y: p.Y + template.Y, VX: template.VX, VY: template.VY,
			Width: weapon.Width, Height: template.Height, Damage: template.Damage, Delay: template.Delay,
			Graphic: template.Graphic, Player: index, Slot: slot, originX: template.X, originY: template.Y,
		}
		occupied[slot] = true
	}
	kept := e.PlayerShots[:0]
	for _, bullet := range e.PlayerShots {
		if bullet.Player != index {
			kept = append(kept, bullet)
		}
	}
	e.PlayerShots = kept
	for slot, bullet := range slots {
		if occupied[slot] {
			bullet.Slot = slot
			e.PlayerShots = append(e.PlayerShots, bullet)
		}
	}
	p.Cooldown, p.Repeat = weapon.Cooldown, 15
	e.Events = append(e.Events, Event{Kind: "shot", Player: index, Value: p.Weapon})
}

func (e *Engine) updatePlayerShots() {
	kept := e.PlayerShots[:0]
	for _, bullet := range e.PlayerShots {
		bullet.Age++
		if bullet.Delay > 0 {
			bullet.Delay--
			if bullet.Delay == 0 {
				// Staggered projectiles attach to the current ship until launch.
				player := e.Players[bullet.Player]
				bullet.X, bullet.Y = player.X+bullet.originX, player.Y+bullet.originY
			}
		} else {
			bullet.X += bullet.VX
			bullet.Y += bullet.VY
		}
		if bullet.X > -16 && bullet.X < 304 && bullet.Y > -13 && bullet.Y < 256 && bullet.Age < 300 {
			kept = append(kept, bullet)
		}
	}
	e.PlayerShots = kept
}

// Spawn creates a decoded object while enforcing the original fixed pool size.
func (e *Engine) Spawn(spawn Spawn) bool {
	if spawn.AbsoluteX {
		spawn.X -= e.CameraX
	}
	if spawn.ClearObjects {
		kept := e.Enemies[:0]
		for _, enemy := range e.Enemies {
			if !enemy.Definition.FlyingPool {
				kept = append(kept, enemy)
			}
		}
		e.Enemies = kept
		explosions := e.Explosions[:0]
		for _, explosion := range e.Explosions {
			if !explosion.Native {
				explosions = append(explosions, explosion)
			}
		}
		e.Explosions = explosions
		pickups := e.Pickups[:0]
		for _, pickup := range e.Pickups {
			if !pickup.SlotHeld {
				pickups = append(pickups, pickup)
			}
		}
		e.Pickups = pickups
		return true
	}
	switch spawn.RandomMode {
	case 6:
		spawn.X = int(e.nextRandom()&0x7f) + 0x40
	case 1, 8:
		spawn.X = int(e.nextRandom()) + int(e.nextRandom()&0x7f) - 0x40
	}
	if spawn.Definition.Collectable {
		slot, held := 0, false
		if spawn.Definition.FlyingPool {
			slot = e.availableFlyingSlot()
			if slot < 0 {
				return false
			}
			held = true
		}
		e.Pickups = append(e.Pickups, Pickup{X: spawn.X, Y: spawn.Y, Weapon: spawn.Definition.Weapon,
			Nova: spawn.Definition.Nova, Graphic: spawn.Definition.Graphic, Sprite: spawn.Definition.Sprite, PoolSlot: slot, SlotHeld: held})
		return true
	}
	count, limit := 0, EnemyLimit
	if spawn.Definition.FlyingPool {
		limit = EnemyShotLimit
	}
	for _, enemy := range e.Enemies {
		if enemy.Definition.FlyingPool == spawn.Definition.FlyingPool {
			count++
		}
	}
	if !spawn.Definition.FlyingPool && count >= limit {
		return false
	}
	slot := 0
	if spawn.Definition.FlyingPool {
		slot = e.availableFlyingSlot()
		if slot < 0 {
			return false
		}
	}
	e.nextID++
	delay := spawn.Definition.FireDelay
	if delay == 0 {
		delay = e.Options.EnemyFireDelay
	}
	health := spawn.Definition.Health
	if spawn.Definition.FlyingPool {
		health = OriginalFlyingArmor(health, e.Players[0].Active && e.Players[1].Active)
	}
	e.Enemies = append(e.Enemies, Enemy{
		ID: e.nextID, PoolSlot: slot, X: spawn.X, Y: spawn.Y, VX: spawn.VX, VY: spawn.VY,
		Frame: spawn.Definition.Frame, Health: health, Definition: spawn.Definition,
		Script: spawn.Script, FireTimer: delay, fixedX: spawn.X << 16, fixedY: spawn.Y << 16,
		scriptLoop: spawn.ScriptLoop, repeatScript: spawn.RepeatScript,
	})
	// The source scenery allocator consumes its random byte before returning.
	if spawn.Definition.Ground && !spawn.Definition.FlyingPool {
		e.initializeGround(&e.Enemies[len(e.Enemies)-1])
	}
	return true
}

func (e *Engine) updateEnemies() {
	e.updateEnemyPass(true)
	for _, spawn := range e.takeNativeGroundSpawns() {
		e.Spawn(spawn)
	}
	e.updateEnemyPass(false)
}

func (e *Engine) updateEnemyPass(groundPass bool) {
	// The source scans flying records from slot zero, updating boss parents first.
	slices.SortStableFunc(e.Enemies, func(first, second Enemy) int {
		if first.Definition.FlyingPool != second.Definition.FlyingPool {
			if first.Definition.FlyingPool {
				return 1
			}
			return -1
		}
		return first.PoolSlot - second.PoolSlot
	})
	kept := make([]Enemy, 0, len(e.Enemies))
	for index, enemy := range e.Enemies {
		if enemy.Definition.FlyingPool == groundPass {
			kept = append(kept, enemy)
			continue
		}
		if enemy.Health < 0 {
			continue
		}
		enemy.Age++
		groundHandled := e.moveNativeGround(&enemy)
		if groundHandled {
			// Ground controllers retain their original mailbox and animation state.
		} else if e.moveNativeSpecial(&enemy) {
			// Multipart controllers own their original script and damage phases.
		} else if len(enemy.Script) != 0 {
			if enemy.scriptTicks == 0 {
				if enemy.scriptIndex == len(enemy.Script) {
					if enemy.repeatScript && enemy.scriptLoop >= 0 && enemy.scriptLoop < len(enemy.Script) {
						enemy.scriptIndex = enemy.scriptLoop
					} else {
						continue
					}
				}
				motion := enemy.Script[enemy.scriptIndex]
				enemy.scriptIndex++
				enemy.scriptTicks = max(1, motion.Duration)
				enemy.VX, enemy.VY, enemy.Frame = motion.VX, motion.VY, motion.Frame
			}
			enemy.fixedX += enemy.VX
			enemy.fixedY += enemy.VY
			enemy.X, enemy.Y = enemy.fixedX>>16, enemy.fixedY>>16
			enemy.scriptTicks--
		} else if !e.moveNativeEnemy(&enemy) {
			enemy.X += enemy.VX
			enemy.Y += enemy.VY
			if enemy.Definition.Ground {
				enemy.Y++
			}
		}
		if enemy.Health < 0 || enemy.Y > 256 || enemy.X < -96 || enemy.X > 384 {
			continue
		}
		// A decoded zero fire delay marks scenery that never emits hostile shots.
		if !groundHandled && enemy.Definition.FireDelay > 0 {
			enemy.FireTimer--
			if enemy.FireTimer <= 0 && enemy.Y >= 0 && enemy.Y < 208 {
				e.fireEnemy(enemy)
				enemy.FireTimer = max(8, enemy.Definition.FireDelay)
			}
		}
		e.Enemies[index] = enemy
		kept = append(kept, enemy)
	}
	e.Enemies = kept
}

func (e *Engine) fireEnemy(enemy Enemy) {
	if len(e.EnemyShots) >= e.Options.EnemyProjectileCap || e.NovaFrames > 0 {
		return
	}
	x, y := enemy.X+enemy.Definition.Width/2, enemy.Y+enemy.Definition.Height/2
	target := -1
	distance := math.MaxInt
	for index, player := range e.Players {
		if !player.Active || player.Lives == 0 || player.Dying > 0 {
			continue
		}
		candidate := abs(player.X+12-x) + abs(player.Y+16-y)
		if candidate < distance {
			target, distance = index, candidate
		}
	}
	if target < 0 {
		return
	}
	p := e.Players[target]
	dx, dy := p.X+12-x, p.Y+16-y
	// Preserve the original DIVU quantization and zero-distance downward shot.
	vx, vy := AimedVelocity(dx, dy, e.Options.EnemyProjectileSpeed)
	e.EnemyShots = append(e.EnemyShots, Bullet{X: x, Y: y, VX: vx, VY: vy, Width: 8, Height: 7,
		Graphic: 0x58, Player: -1, Damage: 1, fixedX: x << 16, fixedY: y << 16})
}

func (e *Engine) updateEnemyShots() {
	kept := e.EnemyShots[:0]
	for _, bullet := range e.EnemyShots {
		bullet.fixedX += bullet.VX
		bullet.fixedY += bullet.VY
		bullet.X, bullet.Y = bullet.fixedX>>16, bullet.fixedY>>16
		bullet.Age++
		if bullet.X >= -16 && bullet.X < 304 && bullet.Y >= -16 && bullet.Y < 240 {
			kept = append(kept, bullet)
		}
	}
	e.EnemyShots = kept
}

func (e *Engine) updatePickups() {
	kept := e.Pickups[:0]
	for _, pickup := range e.Pickups {
		pickup.Age++
		if e.updateNativePickup(&pickup) {
			kept = append(kept, pickup)
		}
	}
	e.Pickups = kept
}

func (e *Engine) updateExplosions() {
	e.advanceExplosions()
}

func clampDirection(value int) int { return max(-1, min(1, value)) }
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
