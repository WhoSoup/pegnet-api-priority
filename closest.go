package main

import "sort"

// calculates which source has a value closest to the reference value from the opr
type closest struct {
	sources []source
}

func (c *closest) add(name string, price float64, reference float64) {
	c.sources = append(c.sources, source{name: name, diff: absDiff(price, reference)})
}

func (c *closest) best() (string, float64) {
	if len(c.sources) == 0 {
		return "no source found", 0
	}
	sort.Slice(c.sources, func(i, j int) bool {
		return c.sources[i].diff < c.sources[j].diff
	})

	return c.sources[0].name, c.sources[0].diff
}

func (c *closest) isBest(name string) bool {
	if len(c.sources) == 0 {
		return false
	}
	_, val := c.best()
	for _, src := range c.sources {
		if src.diff != val {
			break
		}
		if src.name == name {
			return true
		}
	}
	return false
}

type source struct {
	name string
	diff float64
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
