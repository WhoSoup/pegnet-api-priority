package main

import "sort"

// calculates which source has a value closest to the reference value from the opr
type closest struct {
	sources []source
}

func (c *closest) add(name string, price uint64, reference uint64) {
	c.sources = append(c.sources, source{name: name, diff: absDiff(price, reference)})
}

func (c *closest) best() (string, uint64) {
	if len(c.sources) == 0 {
		return "no source found", 0
	}
	sort.Slice(c.sources, func(i, j int) bool {
		return c.sources[i].diff < c.sources[j].diff
	})

	return c.sources[0].name, c.sources[0].diff
}

type source struct {
	name string
	diff uint64
}

func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}
