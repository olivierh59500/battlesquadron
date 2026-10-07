// Package controls maps independent pointers onto an eight-way arcade joystick.
package controls

import "math"

// State contains the logical controls shared by desktop and Android.
type State struct {
	X, Y       float64
	Fire, Nova bool
}

// Point is a pointer position in the application's logical coordinate system.
type Point struct{ X, Y float64 }

// Controller owns individual touches so movement and fire can be held together.
type Controller struct {
	stickID, fireID, novaID int
	stick, center           Point
	active                  map[int]bool
	Width, Height           float64
}

// New creates a controller with no captured pointers.
func New(width, height float64) *Controller {
	c := &Controller{Width: width, Height: height}
	c.Cancel()
	return c
}

// Cancel releases all controls after a pause, focus loss, or Android lifecycle event.
func (c *Controller) Cancel() {
	c.stickID, c.fireID, c.novaID = -1, -1, -1
	c.active = make(map[int]bool)
	c.stick, c.center = Point{}, Point{}
}

// Update captures new touches and releases ended touches without transferring roles.
func (c *Controller) Update(points map[int]Point) State {
	for id := range c.active {
		if _, ok := points[id]; !ok {
			delete(c.active, id)
			if c.stickID == id {
				c.stickID = -1
			}
			if c.fireID == id {
				c.fireID = -1
			}
			if c.novaID == id {
				c.novaID = -1
			}
		}
	}
	// Sorting IDs makes simultaneous gestures deterministic across map iteration.
	ids := make([]int, 0, len(points))
	for id := range points {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	for _, id := range ids {
		p := points[id]
		if !c.active[id] {
			c.active[id] = true
			switch {
			case p.X < 80 && p.Y > 70 && c.stickID == -1:
				c.stickID, c.center = id, p
			case p.X > c.Width-80 && p.Y > c.Height*0.64 && c.fireID == -1:
				c.fireID = id
			case p.X > c.Width-80 && p.Y > c.Height*0.34 && p.Y <= c.Height*0.64 && c.novaID == -1:
				c.novaID = id
			}
		}
		if id == c.stickID {
			c.stick = p
		}
	}
	s := State{Fire: c.fireID != -1, Nova: c.novaID != -1}
	if c.stickID != -1 {
		dx, dy := c.stick.X-c.center.X, c.stick.Y-c.center.Y
		if math.Hypot(dx, dy) > 7 {
			angle := math.Atan2(dy, dx)
			sector := math.Round(angle/(math.Pi/4)) * math.Pi / 4
			s.X, s.Y = math.Round(math.Cos(sector)), math.Round(math.Sin(sector))
		}
	}
	return s
}

// Stick returns the captured gesture's origin and current position for rendering.
func (c *Controller) Stick() (Point, Point, bool) { return c.center, c.stick, c.stickID != -1 }
