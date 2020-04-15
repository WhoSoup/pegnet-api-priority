package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	ds     *polling.DataSources
	Blocks map[int32]*Block
	mtx    sync.RWMutex
}

type Block struct {
	Height     int32
	APIResults map[string]AssetList
	OPRs       map[string][]AssetList
}

func newblock(height int32) *Block {
	b := new(Block)
	b.Height = height
	b.APIResults = make(map[string]AssetList)
	b.OPRs = make(map[string][]AssetList)
	return b
}

type AssetList map[string]float64

func (pc *PriorityCompare) BlockList() []int32 {
	pc.mtx.RLock()
	defer pc.mtx.RUnlock()

	blocks := make([]int32, 0)
	for b := range pc.Blocks {
		blocks = append(blocks, b)
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i] > blocks[j]
	})
	return blocks
}

type Info struct {
	Height  int32
	HasOPRs bool
	HasAPIs bool
	Data    map[string]MoreInfo
}

type MoreInfo struct {
	Miners AssetList
	APIs   AssetList
}

func (pc *PriorityCompare) GetBlock(height int32) *Info {
	pc.mtx.RLock()
	defer pc.mtx.RUnlock()

	if b, ok := pc.Blocks[height]; ok {

		info := new(Info)
		info.Height = height
		info.Data = make(map[string]MoreInfo)

		for _, a := range opr.V4Assets {
			if a == "USD" {
				continue
			}
			info.Data[a] = MoreInfo{
				Miners: make(AssetList),
				APIs:   make(AssetList),
			}
		}

		for name, assets := range b.APIResults {
			for source, value := range assets {
				if _, ok := info.Data[name]; ok {
					info.Data[name].APIs[source] = value
					info.HasAPIs = true
				}
			}
		}

		for miner, oprs := range b.OPRs {
			for i, assets := range oprs {
				id := miner
				if i > 0 {
					id += fmt.Sprintf(" (%d)", i+1)
				}

				for name, value := range assets {
					if _, ok := info.Data[name]; ok {
						info.Data[name].Miners[id] = value
						info.HasOPRs = true
					}
				}
			}
		}

		return info
	}
	return nil
}

// download entries by height
func (pc *PriorityCompare) getEntries(height int32) ([]*factom.Entry, error) {
	dblock, _, err := factom.GetDBlockByHeight(int64(height))
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
func (pc *PriorityCompare) BandCheck(height int32) {
	pc.mtx.RLock()
	defer pc.mtx.RUnlock()

	block, ok := pc.Blocks[height]
	if !ok {
		return
	}

	if len(block.APIResults) == 0 || len(block.OPRs) == 0 {
		log.Printf("Not enough data yet to perform BandCheck")
		return
	}

	count := 0
	total := 0
	for _, oprs := range block.OPRs {
		for _, opr := range oprs {
			total++

			inside := true
			for name, oprValue := range opr {
				best := pc.ds.AssetSources[name][0]
				sourceValue := block.APIResults[name][best]
				if oprValue < sourceValue*.99 || oprValue > sourceValue*1.01 {
					inside = false
				}
			}

			if inside {
				count++
			}

		}
	}

	log.Printf("Current priority order is within 1%% of %d of %d miners (%.2f%%)", count, total, float64(count)/float64(total)*100)
}

func (pc *PriorityCompare) saveEntries(height int32) {
	start := time.Now()
	log.Printf("Downloading entries in block %d", height)
	entries, err := pc.getEntries(height)
	if err != nil {
		log.Printf("Error downloading entries: %v", err)
		return
	}
	log.Println("Entries downloaded in", time.Since(start))

	miner := make(map[string]bool)

	pc.mtx.Lock()
	defer pc.mtx.Unlock()

	block, ok := pc.Blocks[height]
	if !ok {
		block = newblock(height)
		pc.Blocks[height] = block
	}

	for _, e := range entries {
		if len(e.ExtIDs) != 3 {
			continue
		}
		if len(e.ExtIDs[1]) != 8 {
			continue
		}
		if len(e.ExtIDs[2]) != 1 || e.ExtIDs[2][0] < 4 {
			continue
		}

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

		hash := sha256.Sum256(e.Content)
		id := fmt.Sprintf("%s-%x", o.GetID(), hash)

		if miner[id] {
			continue
		}
		miner[id] = true

		list := make(AssetList)
		for _, ass := range o.GetOrderedAssetsFloat() {
			list[ass.Name] = ass.Value
		}

		block.OPRs[o.GetID()] = append(block.OPRs[o.GetID()], list)
	}
}

// Compares entries with price data
func (pc *PriorityCompare) Compare(height int32) {
	pc.mtx.RLock()
	defer pc.mtx.RUnlock()

	block, ok := pc.Blocks[height]
	if !ok {
		return
	}

	if len(block.APIResults) == 0 || len(block.OPRs) == 0 {
		log.Printf("Not enough data yet to perform a comparison")
		return
	}

	comp := new(compete) // global rank

	var once sync.Once
	for miner, oprs := range block.OPRs {
		for i, opr := range oprs {
			once.Do(func() {
				fmt.Printf("=============== Report for Height %d ===============\n", block.Height)
			})

			compminer := new(compete) // per miner rank
			for name, value := range opr {
				if name == "USD" {
					continue
				}
				sources, ok := block.APIResults[name]
				if !ok {
					log.Printf("asset doesn't have a source for some reason: %s", name)
					continue
				}

				c := new(closest)
				for source, val := range sources {
					c.add(source, val, value)
				}

				for _, sourcename := range pc.ds.AssetSources[name] {
					best := c.isBest(sourcename)
					comp.add(sourcename, len(pc.ds.AssetSources[name])-1, best)
					compminer.add(sourcename, len(pc.ds.AssetSources[name])-1, best)
				}
			}

			if i > 0 {
				pc.Results(fmt.Sprintf("OPR [miner = %s (%d)]", miner, i+1), compminer)
			} else {
				pc.Results(fmt.Sprintf("OPR [miner = %s]", miner), compminer)
			}
		}
	}

	fmt.Print("========================================================\n")
	pc.Results("Total", comp)
	fmt.Print("========================================================\n")
}

func (pc *PriorityCompare) saveDatasources(height int32) {
	pc.mtx.Lock()
	defer pc.mtx.Unlock()

	block, ok := pc.Blocks[height]
	if !ok {
		block = newblock(height)
		pc.Blocks[height] = block
	}

	start := time.Now()
	log.Printf("Querying Datasources to use for block %d", height)

	cacheWrap := make(map[string]polling.IDataSource)

	for _, source := range pc.ds.DataSources {
		cacheWrap[source.Name()] = polling.NewCachedDataSource(source)
	}

	for _, asset := range common.AllAssets {
		block.APIResults[asset] = make(map[string]float64)
		for _, sourceName := range dataSource.AssetSources[asset] {
			price, err := cacheWrap[sourceName].FetchPegPrice(asset)
			if err != nil {
				log.Printf("error for (%s, %s): %v\n", sourceName, asset, err)
				continue
			}

			block.APIResults[asset][sourceName] = price.Value
		}
	}
	log.Printf("Datasources fetched in %s", time.Since(start))

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

	for event := range listener {
		if event.Minute != 1 {
			log.Printf("%d minutes until next entry check", (11-event.Minute)%10)
			continue
		}

		pc.saveDatasources(event.Dbht)
		pc.saveEntries(event.Dbht - 1)

		pc.Compare(event.Dbht - 1)
		pc.BandCheck(event.Dbht - 1)

		if debugOut != "" {
			b, err := json.Marshal(pc.Blocks)
			if err != nil {
				log.Print(err)
				continue
			}
			f, err := os.Create(debugOut)
			if err != nil {
				log.Printf("unable to write debug file: %v", err)
			} else {
				f.Write(b)
				f.Close()
			}
		}
	}

	return nil
}
