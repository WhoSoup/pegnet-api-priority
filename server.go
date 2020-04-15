package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"text/tabwriter"
	"text/template"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func server(port int) {
	t := &Template{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}

	e := echo.New()
	e.Renderer = t
	e.Use(middleware.Recover())
	e.GET("/", index)
	e.GET("/block/:height/", block)
	e.GET("/image/:height/:asset", image)
	e.GET("/raw/:height/:asset", raw)

	log.Printf("Stat server starting at http://localhost:%d", port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}

func index(c echo.Context) error {
	return c.Render(http.StatusOK, "index.html", struct {
		Blocks []int32
	}{
		pc.BlockList(),
	})
}

func raw(c echo.Context) error {
	rh := c.Param("height")
	height, err := strconv.Atoi(rh)
	if err != nil {
		return err
	}

	asset := c.Param("asset")
	block := pc.GetBlock(int32(height))
	if block.Height == 0 {
		return fmt.Errorf("Block not found")
	}

	info, ok := block.Data[asset]
	if !ok {
		return fmt.Errorf("invalid asset")
	}

	buf := bytes.NewBuffer(nil)

	tw := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Source\tPrice\t\n")
	fmt.Fprintf(tw, "------\t-----\t\n")
	for source, price := range info.APIs {
		fmt.Fprintf(tw, "%s\t%.8f\t\n", source, price)
	}

	tw.Flush()
	fmt.Fprintln(tw)

	tw = tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Miner\tPrice\t\n")
	fmt.Fprintf(tw, "------\t-----\t\n")
	for source, price := range info.Miners {
		fmt.Fprintf(tw, "%s\t%.8f\t\n", source, price)
	}

	tw.Flush()
	return c.Blob(http.StatusOK, "text/plain", buf.Bytes())
}

func orderkeys(m map[string]plotter.XYs) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

func image(c echo.Context) error {
	rh := c.Param("height")
	height, err := strconv.Atoi(rh)
	if err != nil {
		return err
	}

	asset := c.Param("asset")
	block := pc.GetBlock(int32(height))
	if block.Height == 0 {
		return fmt.Errorf("Block not found")
	}

	info, ok := block.Data[asset]
	if !ok {
		return fmt.Errorf("invalid asset")
	}

	apidata := make(map[string]plotter.XYs)
	max := 0.0
	min := float64(math.MaxFloat64)

	for source, price := range info.APIs {
		apidata[source] = plotter.XYs{plotter.XY{X: price, Y: 0}}
		if price > max {
			max = price
		}
		if price < min {
			min = price
		}
	}

	minerdata := make(map[string]plotter.XYs)

	track := 1.0
	for id, price := range info.Miners {
		minerdata[id] = append(minerdata[id], plotter.XY{X: price, Y: track})
		track++

		if price > max {
			max = price
		}
		if price < min {
			min = price
		}
	}

	p, err := plot.New()
	if err != nil {
		panic(err)
	}
	p.Title.Text = fmt.Sprintf("%s submissions in block %d", asset, height)
	p.X.Label.Text = fmt.Sprintf("Price of %s", asset)
	p.HideY()
	// Draw a grid behind the data
	p.Add(plotter.NewGrid())
	p.Legend.Top = true
	p.X.Max = max * 1.002

	cols := new(Colors)

	for _, source := range orderkeys(apidata) {
		sdata := apidata[source]
		s, err := plotter.NewScatter(sdata)
		if err != nil {
			panic(err)
		}
		s.GlyphStyle.Color = cols.Next()
		s.GlyphStyle.Shape = draw.BoxGlyph{}
		s.GlyphStyle.Radius = vg.Points(6)

		p.Add(s)
		p.Legend.Add(source, s)
	}

	for _, miner := range orderkeys(minerdata) {
		sdata := minerdata[miner]
		s, err := plotter.NewScatter(sdata)
		if err != nil {
			panic(err)
		}
		s.GlyphStyle.Color = cols.Next()
		s.GlyphStyle.Shape = draw.PyramidGlyph{}
		s.GlyphStyle.Radius = vg.Points(6)

		p.Add(s)
		p.Legend.Add(miner, s)
	}

	// Save the plot to a PNG file.
	writer, err := p.WriterTo(800, 4*vg.Inch, "png")
	if err != nil {
		panic(err)
	}

	buf := bytes.NewBuffer(nil)
	writer.WriteTo(buf)
	return c.Blob(http.StatusOK, "image/png", buf.Bytes())
}

func block(c echo.Context) error {
	rh := c.Param("height")
	height, err := strconv.Atoi(rh)
	if err != nil {
		return err
	}

	block := pc.GetBlock(int32(height))
	if block.Height == 0 {
		return fmt.Errorf("Block not found")
	}

	return c.Render(http.StatusOK, "block.html", block)
}
