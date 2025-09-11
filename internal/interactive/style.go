package interactive

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// var color_subtle = lipgloss.AdaptiveColor{
// 	Light: "#909090",
// 	Dark:  "#626262",
// }
// var color_subtle_separator = lipgloss.AdaptiveColor{
// 	Light: "#DDDADA",
// 	Dark:  "#3C3C3C",
// }
// var color_subtle_desc = lipgloss.AdaptiveColor{
// 	Light: "#B2B2B2",
// 	Dark:  "#4A4A4A",
// }

const (
	Gunmetal      = "#1f363d"
	Cerulean      = "#40798c"
	Verdigris     = "#70a9a1"
	CambridgeBlue = "#9ec1a3"
	TeaGreen      = "#cfe0c3"
)

var (
	background_color       = lipgloss.Color("#192b31")
	foreground_color       = lipgloss.Color(TeaGreen)
	selected_color         = lipgloss.Color(Cerulean)
	cursor_color           = lipgloss.Color(Verdigris)
	color_subtle           = lipgloss.Color("#626262")
	color_subtle_separator = lipgloss.Color("#3C3C3C")
	color_subtle_desc      = lipgloss.Color("#4A4A4A")
)

func saturation(color color.Color, s float64) color.Color {
	r, g, b, _ := color.RGBA()
	c := colorful.Color{
		R: float64(r) / 255,
		G: float64(g) / 255,
		B: float64(b) / 255,
	}
	h, l, _ := c.HSLuv()
	c = colorful.HSLuv(h, s, l)
	return lipgloss.Color(c.Hex())
}

func luminance(color color.Color, l float64) color.Color {
	r, g, b, _ := color.RGBA()
	c := colorful.Color{
		R: float64(r) / 255,
		G: float64(g) / 255,
		B: float64(b) / 255,
	}
	h, _, s := c.HSLuv()
	c = colorful.HSLuv(h, s, l)
	return lipgloss.Color(c.Hex())
}
