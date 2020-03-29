package main

// keeps track of data sources competing against each other
// each source is a "player"
// for every currency, there are X sources with that data
// the source with a price closest to the one in the OPR is a winner
// the other sources are losers
//
// the less(string,string) function can be used to sort the external
// list of sources in the order of which source won the largest percentage
// of matchups
type compete struct {
	players map[string]*player
}

func (c *compete) player(name string) *player {
	if c.players == nil {
		c.players = make(map[string]*player)
	}

	if p, ok := c.players[name]; ok {
		return p
	}
	p := new(player)
	c.players[name] = p
	return p
}

func (c *compete) add(player string, matchups int, winner bool) {
	p := c.player(player)
	p.matchups += matchups
	if winner {
		p.beats += matchups
	}
}

func (c *compete) hasWinner(player string) bool {
	return c.player(player).beats > 0
}

func (c *compete) less(a, b string) bool {
	return c.player(a).score() > c.player(b).score()
}

// keeping count for a single player
type player struct {
	beats    int
	matchups int
}

func (p *player) score() float64 {
	return float64(p.beats) / float64(p.matchups)
}
