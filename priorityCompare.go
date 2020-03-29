package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pegnet/pegnet/modules/opr"

	"github.com/FactomProject/factom"
	"github.com/pegnet/pegnet/common"

	"github.com/pegnet/pegnet/polling"
)

var chain = "a642a8674f46696cc47fdb6b65f9c87b2a19c5ea8123b3d2f0c13b6f33a9d5ef"

type PriorityCompare struct {
	ds *polling.DataSources
}

func (pc *PriorityCompare) getEntries(height int64) ([]*factom.Entry, error) {
	dblock, _, err := factom.GetDBlockByHeight(height)
	if err != nil {
		return nil, err
	}

	for _, eblocks := range dblock.DBEntries {
		if eblocks.ChainID == chain {
			return factom.GetAllEBlockEntries(eblocks.KeyMR)
		}
	}

	return nil, fmt.Errorf("unable to find opr eblock for this height")
}

type closeness struct {
	source string
	diff   uint64
	prct   float64
}

func (c closeness) String() string {
	return fmt.Sprintf("[Source: %s, Diff: %d, %.2f%%]", c.source, c.diff, c.prct*100)
}

func valString(assets []opr.AssetUint) string {
	var sb strings.Builder
	for _, ass := range assets {
		sb.WriteString(fmt.Sprintf("#%s-%d", ass.Name, ass.Value))
	}
	return sb.String()
}

func (pc *PriorityCompare) Results(prefix string, comp *compete) {
	var sources []string
	for _, ass := range pc.ds.PriorityList {
		sources = append(sources, ass.DataSource.Name())
	}

	sort.Slice(sources, func(i, j int) bool {
		if sources[i] == "FixedUSD" {
			return true
		}
		return comp.less(sources[i], sources[j])
	})

	barrier := true
	var out strings.Builder
	for i, s := range sources {
		if barrier && !comp.hasWinner(s) && s != "FixedUSD" {
			barrier = false
			out.WriteString("|| ")
		}
		out.WriteString(fmt.Sprintf("%s=%d ", s, i))
	}
	log.Printf("%s %s", prefix, out.String())
}

func (pc *PriorityCompare) BandCheck(entries []*factom.Entry, price map[string]map[string]float64) {
	count := 0
	for _, e := range entries {
		tmp, err := opr.ParseV2Content(e.Content)
		if err != nil {
			log.Println(err)
		}
		o := &opr.V4Content{V2Content: *tmp}

		inside := true
		for _, asset := range o.GetOrderedAssetsFloat() {
			best := pc.ds.AssetSources[asset.Name][0]
			val := price[asset.Name][best]

			if math.Abs(val-asset.Value)/val > 1 {
				inside = false
			}
		}

		if inside {
			count++
		}

	}

	log.Printf("Current priority order is within 1%% of %d/%d OPRs (%.2f%%)", count, len(entries), float64(count)/float64(len(entries))*100)
}

func (pc *PriorityCompare) Compare(entries []*factom.Entry, price map[string]map[string]float64) {
	comp := new(compete)
	miner := make(map[string]bool)

	log.Print("====================================")
	for _, e := range entries {
		tmp, err := opr.ParseV2Content(e.Content)
		if err != nil {
			log.Println(err)
		}
		o := &opr.V4Content{V2Content: *tmp}

		if miner[o.GetID()] {
			continue
		}
		miner[o.GetID()] = true

		compminer := new(compete)

		for _, ass := range o.GetOrderedAssetsUint() {
			if ass.Name == "USD" {
				continue
			}
			sources, ok := price[ass.Name]
			if !ok {
				log.Printf("asset doesn't have a source for some reason: %s", ass.Name)
				continue
			}

			c := new(closest)
			for source, val := range sources {
				uval := opr.FloatToUint64(val)
				c.add(source, uval, ass.Value)
			}

			winner, _ := c.best()

			for _, name := range pc.ds.AssetSources[ass.Name] {
				comp.add(name, len(pc.ds.AssetSources[ass.Name])-1, winner == name)
				compminer.add(name, len(pc.ds.AssetSources[ass.Name])-1, winner == name)
			}
		}

		pc.Results(fmt.Sprintf("OPR [miner = %s]", o.GetID()), compminer)
	}

	log.Print("====================================")
	pc.Results("Total", comp)
	pc.BandCheck(entries, price)
}

func (pc *PriorityCompare) Compare2(entries []*factom.Entry, price map[string]map[string]float64) {
	//comp := new(compete)
	miner := make(map[string]bool)

	//sourceCount := make(map[string]int)

	beats := make(map[string]int)

	for _, e := range entries {
		tmp, err := opr.ParseV2Content(e.Content)
		if err != nil {
			log.Println(err)
		}
		o := &opr.V4Content{V2Content: *tmp}

		if miner[o.GetID()] {
			continue
		}
		miner[o.GetID()] = true

		//compminer := new(compete)

		//log.Printf("OPR [miner = %s]", o.GetID())
		for _, ass := range o.GetOrderedAssetsUint() {
			if ass.Name == "USD" {
				continue
			}
			sources, ok := price[ass.Name]
			if !ok {
				log.Printf("asset doesn't have a source for some reason: %s", ass.Name)
				continue
			}

			var close []closeness
			for source, val := range sources {
				uval := opr.FloatToUint64(val)
				diff := absDiff(uval, ass.Value)
				close = append(close, closeness{source: source, diff: diff, prct: float64(diff) / float64(ass.Value)})
			}

			sort.Slice(close, func(i, j int) bool {
				return close[i].diff < close[j].diff
			})

			//log.Printf("Closest source for %s (%f): %s", ass.Name, opr.Uint64ToFloat(ass.Value), close[0].String())

			s := ""
			for _, possible := range pc.ds.AssetSources[ass.Name] {
				if close[0].source == possible {
					s += fmt.Sprintf(" (%s)", possible)
				} else {
					beats[fmt.Sprintf("%s-%s", close[0].source, possible)]++
					s += fmt.Sprintf(" %s", possible)
				}
			}
			//log.Printf("Possible sources for %s: %s", ass.Name, s)
		}
	}

	var sources []string
	for _, ass := range pc.ds.PriorityList {
		sources = append(sources, ass.DataSource.Name())
	}

	beatTotal := make(map[string]int)
	beatCount := make(map[string]int)

	for _, possible := range pc.ds.AssetSources {
		for _, A := range possible {
			for _, B := range possible {
				if A == B {
					continue
				}
				beatTotal[fmt.Sprintf("%s", A)]++
				beatTotal[fmt.Sprintf("%s", B)]++
				a, oka := beats[fmt.Sprintf("%s-%s", A, B)]
				b, okb := beats[fmt.Sprintf("%s-%s", B, A)]

				if oka && !okb || oka && a > b {
					beatCount[fmt.Sprintf("%s", A)]++
				}
				if okb && !oka || okb && b > a {
					beatCount[fmt.Sprintf("%s", B)]++
				}
			}
		}
	}

	sort.Slice(sources, func(i, j int) bool {
		a := sources[i]
		b := sources[j]
		if a == "FixedUSD" {
			return true
		}
		winrateA := float64(beatCount[fmt.Sprintf("%s", a)]) / float64(beatTotal[fmt.Sprintf("%s", a)])
		winrateB := float64(beatCount[fmt.Sprintf("%s", b)]) / float64(beatTotal[fmt.Sprintf("%s", b)])
		return winrateA > winrateB
	})

	log.Print(sources)
	s := ""
	for i, s := range sources {
		s += fmt.Sprintf("%s=%d ", s, i+1)
	}
	log.Print(s)

	log.Printf("%v", beats)
}

func (pc *PriorityCompare) Debug() {
	var price map[string]map[string]float64
	json.Unmarshal([]byte(DEBUGdata), &price)

	var entries []*factom.Entry
	json.Unmarshal([]byte(DEBUGopr), &entries)

	pc.Compare(entries, price)
	os.Exit(0)
}

func (pc *PriorityCompare) Run() error {
	minute, err := factom.GetCurrentMinute()
	if err != nil {
		return err
	}
	log.Printf("Initializing App at height %d, minute %d, waiting for next block...", minute.DirectoryBlockHeight+1, minute.Minute)

	monitor := common.GetMonitor()

	listener := monitor.NewListener()

	if minute.Minute == 1 { // don't process a partial minute
		<-listener
	}

	var lastPrices map[string]map[string]float64
	for event := range listener {

		if event.Minute != 1 {
			log.Printf("%d minutes until next check", (11-event.Minute)%10)
			continue
		}

		start := time.Now()
		log.Printf("Querying Datasources...")

		cacheWrap := make(map[string]polling.IDataSource)

		for _, source := range pc.ds.DataSources {
			cacheWrap[source.Name()] = polling.NewCachedDataSource(source)
		}

		pa := make(map[string]map[string]float64)
		for _, asset := range common.AllAssets {
			pa[asset] = make(map[string]float64)
			for _, sourceName := range dataSource.AssetSources[asset] {
				price, err := cacheWrap[sourceName].FetchPegPrice(asset)
				if err != nil {
					log.Printf("error for (%s, %s): %v\n", sourceName, asset, err)
					continue
				}

				pa[asset][sourceName] = price.Value
			}
		}
		log.Printf("Datasources fetched in %s", time.Since(start))

		if lastPrices != nil {
			start = time.Now()
			log.Println("Downloading entries...")
			entries, err := pc.getEntries(int64(event.Dbht - 1))
			log.Println("Entries downloaded in", time.Since(start))
			if err != nil {
				log.Printf("Unable to download entries: %v", err)
			} else {
				pc.Compare(entries, lastPrices)
				pc.BandCheck(entries, lastPrices)
			}
		} else {
			log.Println("Don't have last block's prices to compare entries to")
		}

		lastPrices = pa
	}

	return nil
}
