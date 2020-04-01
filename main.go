package main

import (
	"fmt"
	"log"
	"os/user"
	"strings"

	"github.com/FactomProject/factom"

	"github.com/pegnet/pegnet/polling"
	"github.com/zpatrick/go-config"
)

var dataSource *polling.DataSources

// loads the config in the same manner pegnet does
func loadPegnetConfig() *config.Config {
	// load pegnet config
	u, err := user.Current()
	if err != nil {
		panic("Failed to read current user's name")
	}

	configFile := fmt.Sprintf("%s/.pegnet/defaultconfig.ini", u.HomeDir)
	iniFile := config.NewINIFile(configFile)
	return config.NewConfig([]config.Provider{iniFile})
}

func main() {
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
	pc := new(PriorityCompare)
	pc.ds = dataSource

	if err := pc.Run(); err != nil {
		log.Fatalf("App shut down with error: %v", err)
	}
}
