package engine

import (
	"reflect"
	"testing"
)

func TestForecastBranchCannotMutateLiveSimulation(t *testing.T) {
	e := newFixture(t)
	e.Data.Stages = []Stage{{Height: 8192, Events: []Spawn{{Progress: 20, Definition: Definition{Health: 3}}}}}
	e.Campaign.Gates = []Gate{{Phase: 1, Progress: 400}}
	e.Spawn(Spawn{X: 130, Y: 50, Definition: Definition{Health: 3, Width: 32, Height: 32}})
	before := e.Clone()
	branch := e.Clone()
	for range 80 {
		branch.Tick([2]Input{{X: 1, Fire: true, Nova: true}})
	}
	branch.Data.Stages[0].Height = 1
	branch.Data.Stages[0].Events = nil
	branch.Campaign.Gates[0].Progress = 1
	if !reflect.DeepEqual(e, before) {
		t.Fatal("forecast input changed the live state or its stage descriptors")
	}
	if branch.Frame == e.Frame {
		t.Fatal("forecast branch did not execute independently")
	}
}

func TestForecastSameInputsProduceSameResults(t *testing.T) {
	e := newFixture(t)
	e.Data.Stages = []Stage{{Height: 8192}}
	branch := e.Clone()
	for frame := 0; frame < 200; frame++ {
		input := [2]Input{{X: (frame/20)%3 - 1, Fire: true, Nova: frame == 81}}
		e.Tick(input)
		branch.Tick(input)
	}
	if !reflect.DeepEqual(e, branch) {
		t.Fatal("equivalent native inputs diverged after cloning")
	}
}
