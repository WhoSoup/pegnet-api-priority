package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os/user"
	"strings"

	"github.com/FactomProject/factom"
	"github.com/pegnet/pegnet/polling"
	"github.com/zpatrick/go-config"
)

var dataSource *polling.DataSources
var pc *PriorityCompare

var debugOut string
var debugIn string

// loads the config in the same manner pegnet does
func loadPegnetConfig() *config.Config {
	// load pegnet config
	u, err := user.Current()
	if err != nil {
		log.Fatal("Failed to read current user's name")
	}

	configFile := fmt.Sprintf("%s/.pegnet/defaultconfig.ini", u.HomeDir)
	iniFile := config.NewINIFile(configFile)
	return config.NewConfig([]config.Provider{iniFile})
}

func main() {
	serverStart := flag.Bool("server", true, "enable to start a server for extended statistics")
	serverPort := flag.Int("port", 8080, "the port for the web server (if enabled)")

	flag.StringVar(&debugOut, "debug-save", "", "the filename to dump data to")
	flag.StringVar(&debugIn, "debug-read", "", "the filename to read data from. in this mode, apis will not be queried")
	flag.Parse()

	conf := loadPegnetConfig()
	dataSource = polling.NewDataSources(conf)

	// print out the currently enabled data sources by priority
	var sourceOrder []string
	for _, ds := range dataSource.PriorityList {
		sourceOrder = append(sourceOrder, fmt.Sprintf("%s (%d)", ds.DataSource.Name(), ds.Priority))
	}
	log.Printf("%d DataSources Loaded: %s", len(dataSource.DataSources), strings.Join(sourceOrder, ", "))

	// initiate factom client
	factomd, err := conf.String("Miner.FactomdLocation")
	if err != nil {
		log.Fatal(err)
	}
	factom.SetFactomdServer(factomd)

	// initiate app
	pc = new(PriorityCompare)
	pc.ds = dataSource
	pc.Blocks = make(map[int32]*Block)

	if *serverStart {
		go server(*serverPort)
	}

	if debugIn != "" {
		b, err := ioutil.ReadFile(debugIn)
		if err != nil {
			panic(err)
		}

		if err := json.Unmarshal(b, &pc.Blocks); err != nil {
			panic(err)
		}

		select {}
	}

	if err := pc.Run(); err != nil {
		log.Fatalf("App shut down with error: %v", err)
	}
}
