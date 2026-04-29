package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	. "github.com/kungfusheep/glyph"
	"github.com/kungfusheep/riffkey"
)

type service struct {
	Name    string
	CPU     string
	Memory  string
	Latency string
	RPS     string
	CPUVal  float64
	MemVal  float64
	LatVal  float64
	Status  string
}

func main() {
	services := []service{
		{Name: "api-gateway"},
		{Name: "auth-service"},
		{Name: "order-engine"},
		{Name: "inventory-db"},
		{Name: "payment-proc"},
		{Name: "search-index"},
		{Name: "notification"},
		{Name: "cache-layer"},
	}

	app := NewApp()
	app.SetView(
		VBox.Border(BorderRounded).Title(" match demo ")(
			HBox.Gap(2)(
				Text("SERVICE").Bold().Width(16),
				Text("CPU").Bold().Width(10).Align(AlignRight),
				Text("MEM").Bold().Width(10).Align(AlignRight),
				Text("LATENCY").Bold().Width(12).Align(AlignRight),
				Text("RPS").Bold().Width(10).Align(AlignRight),
				Text("STATUS").Bold().Width(12).Align(AlignRight),
			),
			HRule(),
			ForEach(&services, func(s *service) Component {
				return HBox.Gap(2)(
					Text(&s.Name).Width(16),

					Match(&s.CPUVal,
						Gt(90.0, Text(&s.CPU).Width(10).Align(AlignRight).Bold().FG(Red)),
						Gt(70.0, Text(&s.CPU).Width(10).Align(AlignRight).FG(BrightYellow)),
					).Default(Text(&s.CPU).Width(10).Align(AlignRight).FG(Green)),

					Match(&s.MemVal,
						Gt(85.0, Text(&s.Memory).Width(10).Align(AlignRight).Bold().FG(Red)),
						Gt(60.0, Text(&s.Memory).Width(10).Align(AlignRight).FG(BrightYellow)),
					).Default(Text(&s.Memory).Width(10).Align(AlignRight).FG(Green)),

					Match(&s.LatVal,
						Where(func(v float64) bool { return v > 500 },
							Text(&s.Latency).Width(12).Align(AlignRight).Bold().FG(Red)),
						Where(func(v float64) bool { return v > 200 },
							Text(&s.Latency).Width(12).Align(AlignRight).FG(BrightYellow)),
					).Default(Text(&s.Latency).Width(12).Align(AlignRight).FG(Green)),

					Text(&s.RPS).Width(10).Align(AlignRight).FG(BrightBlack),

					Match(&s.Status,
						Where(func(s string) bool { return strings.HasPrefix(s, "crit") },
							Text(&s.Status).Width(12).Align(AlignRight).Bold().FG(Red)),
						Where(func(s string) bool { return strings.HasPrefix(s, "warn") },
							Text(&s.Status).Width(12).Align(AlignRight).FG(BrightYellow)),
					).Default(Text(&s.Status).Width(12).Align(AlignRight).FG(Green)),
				)
			}),
		),
	).Router().NoCounts()

	app.Handle("q", func(_ riffkey.Match) { app.Stop() })

	go simulate(&services, app)

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func simulate(services *[]service, app *App) {
	tick := 0
	for range time.NewTicker(200 * time.Millisecond).C {
		for i := range *services {
			s := &(*services)[i]
			base := float64(i) * 0.7
			phase := float64(tick) * 0.05

			s.CPUVal = clamp(30+40*math.Sin(phase+base)+rand.Float64()*15, 0, 100)
			s.MemVal = clamp(40+30*math.Cos(phase*0.7+base)+rand.Float64()*10, 0, 100)
			s.LatVal = clamp(50+300*math.Sin(phase*0.3+base)+rand.Float64()*50, 5, 2000)
			rps := clamp(500+400*math.Cos(phase*0.4+base)+rand.Float64()*100, 10, 5000)

			s.CPU = fmt.Sprintf("%.1f%%", s.CPUVal)
			s.Memory = fmt.Sprintf("%.1f%%", s.MemVal)
			s.Latency = fmt.Sprintf("%.0fms", s.LatVal)
			s.RPS = fmt.Sprintf("%.0f/s", rps)

			switch {
			case s.CPUVal > 90 || s.LatVal > 500:
				s.Status = "critical"
			case s.CPUVal > 70 || s.LatVal > 200:
				s.Status = "warning"
			default:
				s.Status = "healthy"
			}
		}
		tick++
		app.RequestRender()
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
