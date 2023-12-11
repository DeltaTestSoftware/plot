package main

import "dt/plot"

func main() {
	plot.Plot(func(p *plot.Plotter) {
		p.New().X([]float64{-0.5, -0.1, 2}).Y([]int{-1, 1, -1}).RGB(255, 0, 255)
		p.New().Y([]int{10, -10, 10, -10}).RGB(255, 0, 0)
	})
}
