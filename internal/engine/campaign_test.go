package engine

import (
	"os"
	"testing"
)

func TestOriginalSurfaceGateOperands(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if err != nil {
		t.Skip("the original loader is extracted locally and excluded from Git")
	}
	gates, err := DecodeSurfaceGates(loader, 0x100)
	if err != nil {
		t.Fatal(err)
	}
	if len(gates) != 3 {
		t.Fatalf("got%d surfacegates", len(gates))
	}
	progress := []int{0x0f10, 0x1490, 0x1dd0}
	x := []int{192, 224, 176}
	for i, gate := range gates {
		if gate.Phase != i+1 || gate.Progress != progress[i] || gate.WorldX != x[i] || gate.Definition.Address != 0x2f40 || gate.Definition.Kind != 39 || gate.Definition.Frames != 2 {
			t.Errorf("gate%d: %+v", i, gate)
		}
	}
}

func TestPortalNeedsTenConsecutiveTicksAndAllLivePlayers(t *testing.T) {
	c := NewCampaign(nil)
	players := [2]Player{{Active: true, Lives: 3, X: 100, Y: 100}, {Active: true, Lives: 3, X: 100, Y: 100}}
	for i := 0; i < 9; i++ {
		if c.UpdatePortal(players, 100, 100, true) {
			t.Fatalf("enteredafter%d ticks", i+1)
		}
	}
	players[1].X = 117
	if c.UpdatePortal(players, 100, 100, true) || c.PortalHold != 0 {
		t.Fatal("outsidepartner didnotresetthehold")
	}
	players[1].X = 100
	for i := 0; i < 9; i++ {
		if c.UpdatePortal(players, 100, 100, true) {
			t.Fatal("didnotrequirefreshconsecutiveticks")
		}
	}
	if !c.UpdatePortal(players, 100, 100, true) {
		t.Fatal("tenthvalidtickdidnotenter")
	}
	if c.UpdatePortal(players, 100, 100, true) {
		t.Fatal("heldportaltriggeredtwice")
	}
	c.UpdatePortal(players, 100, 100, false)
	if c.PortalHold != 0 {
		t.Fatal("hiddenportaldidnotreset")
	}
}

func TestPortalStrictBottomAndDeadPlayerEligibility(t *testing.T) {
	for _, edge := range []struct {
		name   string
		x, y   int
		inside bool
	}{{"left", 84, 100, true}, {"right", 116, 100, true}, {"top", 100, 84, true}, {"bottom", 100, 116, false}, {"above", 100, 83, false}} {
		t.Run(edge.name, func(t *testing.T) {
			c := NewCampaign(nil)
			p := [2]Player{{Active: true, Lives: 3, X: edge.x, Y: edge.y}}
			for i := 0; i < 10; i++ {
				entered := c.UpdatePortal(p, 100, 100, true)
				if entered != (edge.inside && i == 9) {
					t.Fatalf("edge%s,tick%d entered%v", edge.name, i, entered)
				}
			}
		})
	}
	c := NewCampaign(nil)
	players := [2]Player{{Active: true, Lives: 3, X: 100, Y: 100}, {Active: true, Lives: 3, Dying: 4, X: 200, Y: 200}}
	for i := 0; i < 10; i++ {
		entered := c.UpdatePortal(players, 100, 100, true)
		if entered != (i == 9) {
			t.Fatal("deadpartnerblockedlivingplayer")
		}
	}
	c.UpdatePortal([2]Player{}, 100, 100, true)
	if c.PortalHold != 0 {
		t.Fatal("noliveplayersdidnotresetthehold")
	}
}

func TestCampaignReturnsToSurfaceAndAccumulatesAllThreeClearBits(t *testing.T) {
	gates := []Gate{{Phase: 1, Progress: 3856, WorldX: 192, Definition: Definition{Height: 32}}, {Phase: 2, Progress: 5264, WorldX: 224, Definition: Definition{Height: 32}}, {Phase: 3, Progress: 7632, WorldX: 176, Definition: Definition{Height: 32}}}
	c := NewCampaign(gates)
	for index, gate := range gates {
		scroll := gate.Progress + 100
		if selected, ok := c.GateAtScroll(scroll); !ok || selected.Phase != gate.Phase {
			t.Fatalf("phase%d gateunavailable", gate.Phase)
		}
		stage, err := c.EnterCave(gate.Phase, scroll)
		if err != nil || stage != gate.Phase {
			t.Fatalf("entry:%d,%v", stage, err)
		}
		if _, ok := c.GateAtScroll(scroll); ok {
			t.Fatal("surfaceportalvisibleinsideacave")
		}
		if _, err := c.EnterCave(gate.Phase, scroll); err == nil {
			t.Fatal("nestedcaveentryaccepted")
		}
		returned, err := c.ExitCave()
		if err != nil || returned != scroll {
			t.Fatalf("returnscroll%d,%v", returned, err)
		}
		if c.ClearedMask&(1<<uint(gate.Phase)) == 0 {
			t.Fatal("completedcavebitmissing")
		}
		if _, ok := c.GateAtScroll(scroll); ok {
			t.Fatal("completedentrancewasofferedagain")
		}
		if c.Completed() != (index == 2) {
			t.Fatal("campaigncompletiondidnotrequireallthreecaves")
		}
	}
	if _, err := c.ExitCave(); err == nil {
		t.Fatal("surfaceexitaccepted")
	}
	if _, err := c.EnterCave(4, 9000); err == nil {
		t.Fatal("invalidcaveaccepted")
	}
}

func TestMalformedGateCodeFails(t *testing.T) {
	for _, loader := range [][]byte{nil, make([]byte, 128), {0x0c, 0x6d, 0x0f, 0x10}} {
		if _, err := DecodeSurfaceGates(loader, 0x100); err == nil {
			t.Fatal("missingchecksaccepted")
		}
	}
}
