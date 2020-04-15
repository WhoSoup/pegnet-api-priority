package main

import (
	"image/color"
)

var base []color.RGBA

func init() {
	base = []color.RGBA{
		color.RGBA{R: 236, G: 19, B: 19, A: 0},
		color.RGBA{R: 19, G: 65, B: 236, A: 0},
		color.RGBA{R: 19, G: 236, B: 19, A: 0},
		color.RGBA{R: 217, G: 236, B: 19, A: 0},
		color.RGBA{R: 19, G: 236, B: 228, A: 0},
		color.RGBA{R: 212, G: 45, B: 198, A: 0},
	}
}

type Colors struct {
	i int
}

func (c *Colors) Next() color.RGBA {
	col := base[c.i%len(base)]

	level := c.i / len(base)
	if level > 0 {
		col.R = uint8(float64(col.R) * (1 - 3*float64(level)/10))
	}

	c.i++
	return col
}
