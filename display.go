package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/FactomProject/factom"
	"github.com/pegnet/pegnet/modules/opr"
	"github.com/pegnet/pegnet/polling"
)

func contains(a []string, b string) bool {
	for _, v := range a {
		if v == b {
			return true
		}
	}
	return false
}

func (pc *PriorityCompare) Display() {
	datab, _ := ioutil.ReadFile("data-238486.txt")
	data := string(datab)

	lines := strings.Split(data, "\n")

	price := make(map[string]map[string]polling.PegItem)
	var entries []*factom.Entry

	json.Unmarshal([]byte(lines[0]), &entries)
	json.Unmarshal([]byte(lines[1]), &price)

	tmp, err := opr.ParseV2Content(entries[0].Content)
	if err != nil {
		log.Fatal(err)
	}
	o := &opr.V4Content{V2Content: *tmp}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	p := func(start, end int) bool {

		assets := o.GetOrderedAssetsFloat()
		if start >= len(assets) {
			return false
		}
		for i := start; i < len(assets) && i < end; i++ {
			fmt.Fprintf(tw, "\t%s", assets[i].Name)
		}
		fmt.Fprintln(tw, "\t")
		for i := start; i < len(assets) && i < end; i++ {
			fmt.Fprintf(tw, "\t%s", strings.Repeat("=", len(assets[i].Name)))
		}
		fmt.Fprintln(tw, "\t")

		for _, pr := range pc.ds.PriorityList {
			fmt.Fprintf(tw, "%s\t", pr.DataSource.Name())

			for i := start; i < len(assets) && i < end; i++ {

				if v, ok := price[assets[i].Name][pr.DataSource.Name()]; ok {
					c := new(closest)
					for _, prr := range pc.ds.PriorityList {
						c.add(prr.DataSource.Name(), opr.FloatToUint64(price[assets[i].Name][prr.DataSource.Name()].Value), opr.FloatToUint64(assets[i].Value))
					}

					if c.isBest(pr.DataSource.Name()) {
						fmt.Fprintf(tw, "(%.8f)\t", v.Value)
					} else {
						fmt.Fprintf(tw, "%.8f\t", v.Value)
					}
				} else {
					fmt.Fprintf(tw, "\t")
				}
			}
			fmt.Fprintln(tw, "")
		}

		for i := start; i < len(assets) && i < end; i++ {
			fmt.Fprintf(tw, "\t%s", strings.Repeat("=", len(assets[i].Name)))
		}
		fmt.Fprintln(tw, "\t")

		fmt.Fprintf(tw, "%s", o.GetID())
		for i := start; i < len(assets) && i < end; i++ {
			fmt.Fprintf(tw, "\t%.8f", assets[i].Value)
		}
		fmt.Fprintln(tw, "\t")

		tw.Flush()
		fmt.Println()
		return true
	}

	i := 0
	for p(i, i+8) {
		i += 8
	}
}
