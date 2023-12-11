package plot

import (
	"fmt"
	"math"

	"github.com/gonutz/prototype/draw"
)

func Plot(plot func(p *Plotter)) error {
	p := &Plotter{}
	p.ResetRanges()
	return draw.RunWindow("Plot", 800, 600, func(window draw.Window) {
		if window.WasKeyPressed(draw.KeyEscape) {
			window.Close()
			return
		}

		if window.WasKeyPressed(draw.KeyF11) {
			p.fullscreen = !p.fullscreen
		}
		window.SetFullscreen(p.fullscreen)

		if window.WasKeyPressed(draw.KeyR) {
			p.ResetRanges()
		}

		p.Window = window
		p.reset()
		plot(p)
		p.drawGraphs()

		if p.doAtEnd != nil {
			p.doAtEnd()
		}
	})
}

type Plotter struct {
	draw.Window
	fullscreen bool
	dragging   bool
	dragX      int
	dragY      int
	minX       float64
	maxX       float64
	minY       float64
	maxY       float64
	graphs     []*Graph
	doAtEnd    func()
}

func (p *Plotter) SetFullscreen(f bool) {
	p.fullscreen = f
	p.dragging = false
}

func (p *Plotter) ResetRanges() {
	p.minX = math.Inf(1)
	p.maxX = math.Inf(-1)
	p.minY = math.Inf(1)
	p.maxY = math.Inf(-1)
	p.dragging = false
}

func (p *Plotter) reset() {
	p.graphs = p.graphs[:0]
	p.doAtEnd = nil
}

type Graph struct {
	x     []float64
	y     []float64
	color draw.Color
}

func (p *Plotter) New() *Graph {
	g := &Graph{
		color: draw.White,
	}
	p.graphs = append(p.graphs, g)
	return g
}

func (p *Plotter) Defer(f func()) {
	p.doAtEnd = f
}

func (g *Graph) X(x any) *Graph {
	g.x = cast(x)
	return g
}

func (g *Graph) Y(y any) *Graph {
	g.y = cast(y)
	return g
}

func (g *Graph) XY(xy any) *Graph {
	xys := cast(xy)
	if len(xys)%2 != 0 {
		// TODO Remove all panics with displaying the error as red text in the
		// window.
		panic("invaild XY values, length is not divisible by 2")
	}
	g.x = make([]float64, len(xys)/2)
	g.y = make([]float64, len(xys)/2)
	for i := range g.x {
		g.x[i] = xys[i*2]
	}
	for i := range g.y {
		g.y[i] = xys[i*2+1]
	}
	return g
}

func (g *Graph) RGB(red, green, blue uint8) *Graph {
	g.color = draw.RGB(float32(red)/255, float32(green)/255, float32(blue)/255)
	return g
}

func (p *Plotter) drawGraphs() {
	mouseX, mouseY := p.MousePosition()

	if p.IsMouseDown(draw.LeftButton) {
		if !p.dragging {
			p.dragX, p.dragY = mouseX, mouseY
			p.dragging = true
		}
	} else {
		p.dragging = false
	}

	// The user does not need to specify values for x. If unspecified, we just
	// make x count up like: 0, 1, 2, 3, 4, ...
	for _, g := range p.graphs {
		if len(g.x) == 0 {
			g.x = make([]float64, len(g.y))
			for i := range g.x {
				g.x[i] = float64(i)
			}
		}
	}

	// If the ranges are at their default value, we calculate the outer bounds
	// of all visible graphs and use that instead.
	if isInf(p.minX) {
		p.minX = math.Inf(1)
		p.maxX = math.Inf(-1)
		p.minY = math.Inf(1)
		p.maxY = math.Inf(-1)

		for _, g := range p.graphs {
			for _, x := range g.x {
				if x < p.minX {
					p.minX = x
				}
				if x > p.maxX {
					p.maxX = x
				}
			}
			for _, y := range g.y {
				if y < p.minY {
					p.minY = y
				}
				if y > p.maxY {
					p.maxY = y
				}
			}
		}

		var xMargin float64 = 1
		if p.minX < p.maxX {
			xMargin = (p.maxX - p.minX) / 10
		}
		p.minX -= xMargin
		p.maxX += xMargin

		var yMargin float64 = 1
		if p.minY < p.maxY {
			yMargin = (p.maxY - p.minY) / 10
		}
		p.minY -= yMargin
		p.maxY += yMargin
	}

	width, height := p.Size()

	t := p.newTransformer()

	validXRange := !isInf(p.minX)
	validYRange := !isInf(p.minY)

	// Drag the view with the mouse.
	if p.dragging && validXRange {
		screenDx := p.dragX - mouseX
		screenDy := mouseY - p.dragY
		if screenDx != 0 || screenDy != 0 {
			dx := float64(screenDx) * t.xFromScreen
			dy := float64(screenDy) * t.yFromScreen
			p.minX += dx
			p.maxX += dx
			p.minY += dy
			p.maxY += dy
			p.dragX, p.dragY = mouseX, mouseY
			t = p.newTransformer()
		}
	}

	// Zoom with the mouse wheel.
	wheelY := p.MouseWheelY()
	if wheelY != 0 && validXRange {
		mx, my := t.fromScreen(mouseX, mouseY)

		scale := math.Pow(1.1, -wheelY)
		newXRange := t.xRange * scale
		newYRange := t.yRange * scale
		p.maxX = p.minX + newXRange
		p.maxY = p.minY + newYRange

		t = p.newTransformer()
		mx2, my2 := t.fromScreen(mouseX, mouseY)
		dx := mx - mx2
		dy := my - my2

		p.minX += dx
		p.maxX += dx
		p.minY += dy
		p.maxY += dy
		t = p.newTransformer()
	}

	// Draw the axes.
	x0, y0 := t.toScreen(0, 0)
	p.DrawLine(0, y0, width, y0, draw.White)
	p.DrawLine(x0, 0, x0, height, draw.White)

	// Draw tick marks.
	var xPrecision, yPrecision int

	if validXRange {
		var xScale float64
		xScale, xPrecision = calcStepsAndPrecision(t.xRange)
		x := float64(round(p.minX/xScale))*xScale - xScale
		for x <= p.maxX {
			if abs(x) > xScale/10 {
				tickX, tickY := t.toScreen(x, 0)
				p.DrawLine(tickX, tickY-3, tickX, tickY+4, draw.White)
				text := fmt.Sprintf("%.*f", xPrecision, x)
				textW, _ := p.GetTextSize(text)
				p.DrawText(text, tickX-textW/2, tickY+5, draw.White)
			}
			x += xScale
		}
	}

	if validYRange {
		var yScale float64
		yScale, yPrecision = calcStepsAndPrecision(t.yRange)
		y := float64(round(p.minY/yScale))*yScale - yScale
		for y <= p.maxY {
			if abs(y) > yScale/10 {
				tickX, tickY := t.toScreen(0, y)
				p.DrawLine(tickX-3, tickY, tickX+4, tickY, draw.White)
				text := fmt.Sprintf("%.*f", yPrecision, y)
				textW, textH := p.GetTextSize(text)
				p.DrawText(text, tickX-5-textW, tickY-textH/2, draw.White)
			}
			y += yScale
		}
	}

	// Draw the graphs.
	for _, g := range p.graphs {
		if len(g.x) == 0 {
			continue
		}

		x, y := t.toScreen(g.x[0], g.y[0])
		for i := 1; i < len(g.x); i++ {
			x2, y2 := t.toScreen(g.x[i], g.y[i])
			p.DrawLine(x, y, x2, y2, g.color)
			x, y = x2, y2
		}
		// DrawLine does not draw the last point in a line, so we have to draw
		// the very last line in the graph ourselves.
		p.DrawPoint(x, y, g.color)
	}

	// Write the current mouse position in the lower right hand corner.
	mx, my := t.fromScreen(mouseX, mouseY)
	mouseText := fmt.Sprintf("%.*f %.*f", xPrecision+1, mx, yPrecision+1, my)
	textW, textH := p.GetTextSize(mouseText)
	p.DrawText(mouseText, width-textW, height-textH, draw.White)
}

func (p *Plotter) newTransformer() transformer {
	width, height := p.Size()
	xRange := p.maxX - p.minX
	yRange := p.maxY - p.minY
	w, h := float64(width-1), float64(height-1)
	xToScreen := w / xRange
	yToScreen := h / yRange
	xFromScreen := 1.0 / xToScreen
	yFromScreen := 1.0 / yToScreen
	return transformer{
		minX:        p.minX,
		minY:        p.minY,
		xRange:      xRange,
		yRange:      yRange,
		xToScreen:   xToScreen,
		yToScreen:   yToScreen,
		xFromScreen: xFromScreen,
		yFromScreen: yFromScreen,
		height:      height,
	}
}

type transformer struct {
	minX        float64
	minY        float64
	xRange      float64
	yRange      float64
	xToScreen   float64
	yToScreen   float64
	xFromScreen float64
	yFromScreen float64
	height      int
}

func (t transformer) toScreen(x, y float64) (screenX, screenY int) {
	screenX = round((x - t.minX) * t.xToScreen)
	screenY = t.height - 1 - round((y-t.minY)*t.yToScreen)
	return
}

func (t transformer) fromScreen(screenX, screenY int) (x, y float64) {
	x = t.minX + float64(screenX)*t.xFromScreen
	y = t.minY + float64(t.height-1-screenY)*t.yFromScreen
	return
}

func calcStepsAndPrecision(theRange float64) (float64, int) {
	steps := theRange / 10
	scale := float64(1)
	prec := 0
	if steps < 1 {
		for steps < 1 {
			steps *= 10
			scale /= 10
			prec++
		}
	} else {
		for steps > 1 {
			steps /= 10
			scale *= 10
		}
	}

	if theRange/scale < 5 {
		scale *= 0.5
	}
	if theRange/scale > 15 {
		scale *= 2
	}

	return scale, prec
}

type Color = draw.Color

func RGB(r, g, b uint8) Color {
	return draw.RGB(float32(r)/255, float32(g)/255, float32(b)/255)
}

var (
	Black       = draw.Black
	White       = draw.White
	Gray        = draw.Gray
	LightGray   = draw.LightGray
	DarkGray    = draw.DarkGray
	Red         = draw.Red
	LightRed    = draw.LightRed
	DarkRed     = draw.DarkRed
	Green       = draw.Green
	LightGreen  = draw.LightGreen
	DarkGreen   = draw.DarkGreen
	Blue        = draw.Blue
	LightBlue   = draw.LightBlue
	DarkBlue    = draw.DarkBlue
	Purple      = draw.Purple
	LightPurple = draw.LightPurple
	DarkPurple  = draw.DarkPurple
	Yellow      = draw.Yellow
	LightYellow = draw.LightYellow
	DarkYellow  = draw.DarkYellow
	Cyan        = draw.Cyan
	LightCyan   = draw.LightCyan
	DarkCyan    = draw.DarkCyan
	Brown       = draw.Brown
	LightBrown  = draw.LightBrown
)

type MouseClick = draw.MouseClick
type MouseButton = draw.MouseButton

const (
	LeftButton   = draw.LeftButton
	MiddleButton = draw.MiddleButton
	RightButton  = draw.RightButton
)

type Key = draw.Key

const (
	KeyA            = draw.KeyA
	KeyB            = draw.KeyB
	KeyC            = draw.KeyC
	KeyD            = draw.KeyD
	KeyE            = draw.KeyE
	KeyF            = draw.KeyF
	KeyG            = draw.KeyG
	KeyH            = draw.KeyH
	KeyI            = draw.KeyI
	KeyJ            = draw.KeyJ
	KeyK            = draw.KeyK
	KeyL            = draw.KeyL
	KeyM            = draw.KeyM
	KeyN            = draw.KeyN
	KeyO            = draw.KeyO
	KeyP            = draw.KeyP
	KeyQ            = draw.KeyQ
	KeyR            = draw.KeyR
	KeyS            = draw.KeyS
	KeyT            = draw.KeyT
	KeyU            = draw.KeyU
	KeyV            = draw.KeyV
	KeyW            = draw.KeyW
	KeyX            = draw.KeyX
	KeyY            = draw.KeyY
	KeyZ            = draw.KeyZ
	Key0            = draw.Key0
	Key1            = draw.Key1
	Key2            = draw.Key2
	Key3            = draw.Key3
	Key4            = draw.Key4
	Key5            = draw.Key5
	Key6            = draw.Key6
	Key7            = draw.Key7
	Key8            = draw.Key8
	Key9            = draw.Key9
	KeyNum0         = draw.KeyNum0
	KeyNum1         = draw.KeyNum1
	KeyNum2         = draw.KeyNum2
	KeyNum3         = draw.KeyNum3
	KeyNum4         = draw.KeyNum4
	KeyNum5         = draw.KeyNum5
	KeyNum6         = draw.KeyNum6
	KeyNum7         = draw.KeyNum7
	KeyNum8         = draw.KeyNum8
	KeyNum9         = draw.KeyNum9
	KeyF1           = draw.KeyF1
	KeyF2           = draw.KeyF2
	KeyF3           = draw.KeyF3
	KeyF4           = draw.KeyF4
	KeyF5           = draw.KeyF5
	KeyF6           = draw.KeyF6
	KeyF7           = draw.KeyF7
	KeyF8           = draw.KeyF8
	KeyF9           = draw.KeyF9
	KeyF10          = draw.KeyF10
	KeyF11          = draw.KeyF11
	KeyF12          = draw.KeyF12
	KeyF13          = draw.KeyF13
	KeyF14          = draw.KeyF14
	KeyF15          = draw.KeyF15
	KeyF16          = draw.KeyF16
	KeyF17          = draw.KeyF17
	KeyF18          = draw.KeyF18
	KeyF19          = draw.KeyF19
	KeyF20          = draw.KeyF20
	KeyF21          = draw.KeyF21
	KeyF22          = draw.KeyF22
	KeyF23          = draw.KeyF23
	KeyF24          = draw.KeyF24
	KeyEnter        = draw.KeyEnter
	KeyNumEnter     = draw.KeyNumEnter
	KeyLeftControl  = draw.KeyLeftControl
	KeyRightControl = draw.KeyRightControl
	KeyLeftShift    = draw.KeyLeftShift
	KeyRightShift   = draw.KeyRightShift
	KeyLeftAlt      = draw.KeyLeftAlt
	KeyRightAlt     = draw.KeyRightAlt
	KeyLeft         = draw.KeyLeft
	KeyRight        = draw.KeyRight
	KeyUp           = draw.KeyUp
	KeyDown         = draw.KeyDown
	KeyEscape       = draw.KeyEscape
	KeySpace        = draw.KeySpace
	KeyBackspace    = draw.KeyBackspace
	KeyTab          = draw.KeyTab
	KeyHome         = draw.KeyHome
	KeyEnd          = draw.KeyEnd
	KeyPageDown     = draw.KeyPageDown
	KeyPageUp       = draw.KeyPageUp
	KeyDelete       = draw.KeyDelete
	KeyInsert       = draw.KeyInsert
	KeyNumAdd       = draw.KeyNumAdd
	KeyNumSubtract  = draw.KeyNumSubtract
	KeyNumMultiply  = draw.KeyNumMultiply
	KeyNumDivide    = draw.KeyNumDivide
	KeyCapslock     = draw.KeyCapslock
	KeyPrint        = draw.KeyPrint
	KeyPause        = draw.KeyPause
)

func cast(x any) []float64 {
	switch x := x.(type) {
	case []float64:
		return x
	case []float32:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []int:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []int64:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []int32:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []int16:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []int8:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []uint:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []uint64:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []uint32:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []uint16:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	case []uint8:
		out := make([]float64, len(x))
		for i := range out {
			out[i] = float64(x[i])
		}
		return out
	}
	panic(fmt.Sprintf("invalid type, slice of numbers expected but have %T", x))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func isInf(x float64) bool {
	return math.IsInf(x, 1) || math.IsInf(x, -1)
}

func round(f float64) int {
	if f < 0 {
		return int(f - 0.5)
	}
	return int(f + 0.5)
}
