package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pegnet/pegnet/modules/opr"

	"github.com/FactomProject/factom"
	"github.com/pegnet/pegnet/common"

	"github.com/pegnet/pegnet/polling"
)

// opr chain id
var chain = "a642a8674f46696cc47fdb6b65f9c87b2a19c5ea8123b3d2f0c13b6f33a9d5ef"

type PriorityCompare struct {
	ds *polling.DataSources
}

// download entries by height
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

// print the results of a `compete` run
func (pc *PriorityCompare) Results(prefix string, comp *compete) {
	var sources []string
	for _, ass := range pc.ds.PriorityList {
		sources = append(sources, ass.DataSource.Name())
	}

	sort.Slice(sources, func(i, j int) bool {
		if sources[i] == "FixedUSD" { // hardcode to be 0
			return true
		}
		return comp.less(sources[i], sources[j])
	})

	var once sync.Once
	var out strings.Builder
	for i, s := range sources {
		// print || before sources without any winners
		if !comp.hasWinner(s) && s != "FixedUSD" {
			once.Do(func() {
				out.WriteString("\nUnused: ")
			})
		}
		out.WriteString(fmt.Sprintf("%s=%d ", s, i))
	}
	fmt.Printf("%s\n  Used: %s\n", prefix, out.String())
}

// BandCheck compares if configured sources are within a 1% band of oprs
func (pc *PriorityCompare) BandCheck(entries []*factom.Entry, price map[string]map[string]polling.PegItem) {
	count := 0
	for _, e := range entries {
		tmp, err := opr.ParseV2Content(e.Content)
		if err != nil {
			log.Println(err)
			continue
		}
		o := &opr.V4Content{V2Content: *tmp}

		if len(tmp.Assets) != len(opr.V4Assets) {
			continue
		}

		inside := true
		for _, asset := range o.GetOrderedAssetsFloat() {
			best := pc.ds.AssetSources[asset.Name][0]
			val := price[asset.Name][best]

			pct := val.Value * 0.01
			if asset.Value < val.Value-pct || asset.Value > val.Value+pct {
				inside = false
			}
		}

		if inside {
			count++
		}

	}

	log.Printf("Current priority order is within 1%% of %d/%d OPRs (%.2f%%)", count, len(entries), float64(count)/float64(len(entries))*100)
}

// Compares entries with price data
func (pc *PriorityCompare) Compare(height int32, entries []*factom.Entry, price map[string]map[string]polling.PegItem) {
	comp := new(compete) // global rank
	miner := make(map[string]bool)

	var once sync.Once
	for i, e := range entries {
		tmp, err := opr.ParseV2Content(e.Content)
		if err != nil {
			log.Println(err)
			continue
		}
		// do some rudimentary error checks
		if len(tmp.Assets) != len(opr.V4Assets) {
			continue
		}
		if tmp.Height != height {
			continue
		}

		o := &opr.V4Content{V2Content: *tmp}

		// only do once per miner
		if miner[o.GetID()] {
			continue
		}
		miner[o.GetID()] = true

		once.Do(func() {
			fmt.Printf("=============== Report for Height %d ===============\n", o.GetHeight())
		})
		if i > 0 {
			fmt.Println()
		}

		compminer := new(compete) // per miner rank
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
				uval := opr.FloatToUint64(val.Value)
				c.add(source, uval, ass.Value)
			}

			for _, name := range pc.ds.AssetSources[ass.Name] {
				best := c.isBest(name)
				comp.add(name, len(pc.ds.AssetSources[ass.Name])-1, best)
				compminer.add(name, len(pc.ds.AssetSources[ass.Name])-1, best)
			}
		}

		pc.Results(fmt.Sprintf("OPR [miner = %s]", o.GetID()), compminer)
	}

	fmt.Print("========================================================\n")
	pc.Results("Total", comp)
	fmt.Print("========================================================\n")
}

func (pc *PriorityCompare) Run() error {
	// get current minute info just for display purposes
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

	var lastPrices map[string]map[string]polling.PegItem
	for event := range listener {
		if event.Minute != 1 {
			log.Printf("%d minutes until next entry check", (11-event.Minute)%10)
			continue
		}

		pa := make(map[string]map[string]polling.PegItem)
		start := time.Now()
		log.Printf("Querying Datasources to use for block %d", event.Dbht+1)

		cacheWrap := make(map[string]polling.IDataSource)

		for _, source := range pc.ds.DataSources {
			cacheWrap[source.Name()] = polling.NewCachedDataSource(source)
		}

		for _, asset := range common.AllAssets {
			pa[asset] = make(map[string]polling.PegItem)
			for _, sourceName := range dataSource.AssetSources[asset] {
				price, err := cacheWrap[sourceName].FetchPegPrice(asset)
				if err != nil {
					log.Printf("error for (%s, %s): %v\n", sourceName, asset, err)
					continue
				}

				pa[asset][sourceName] = price
			}
		}
		log.Printf("Datasources fetched in %s", time.Since(start))

		if lastPrices != nil {
			start := time.Now()
			log.Println("Downloading entries...")
			entries, err := pc.getEntries(int64(event.Dbht - 1))
			log.Println("Entries downloaded in", time.Since(start))
			if err != nil {
				log.Printf("Unable to download entries: %v", err)
			} else {
				pc.Compare(event.Dbht-1, entries, lastPrices)
				pc.BandCheck(entries, lastPrices)
			}
		} else {
			log.Println("Don't have last block's prices to compare entries to, need to wait for next block")
		}

		lastPrices = pa
	}

	return nil
}
