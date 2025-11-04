package interactive

import (
	"image/color"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// {
// 	'gunmetal': {
// 		DEFAULT: '#1f363d',
// 		100: '#060b0c',
// 		200: '#0c1618', 300: '#122025', 400: '#192b31', 500: '#1f363d', 600: '#3b6775', 700: '#5898ab', 800: '#90bac7', 900: '#c7dde3'
// 	},
// 	'cerulean': {
// 		DEFAULT: '#40798c', 100: '#0d181c', 200: '#1a3038', 300: '#274954', 400: '#336170', 500: '#40798c', 600: '#579bb2', 700: '#81b4c5',
// 		800: '#abcdd8', 900: '#d5e6ec'
// 	},
// 	'verdigris': {
// 		DEFAULT: '#70a9a1', 100: '#152321', 200: '#2a4642', 300: '#3f6964', 400: '#548c85',
// 		500: '#70a9a1', 600: '#8cbab4', 700: '#a9cbc7', 800: '#c6ddda', 900: '#e2eeec'
// 	},
// 	'cambridge_blue': {
// 		DEFAULT: '#9ec1a3', 100: '#1b2b1e', 200: '#37563c', 300: '#528159', 400: '#74a67b',
// 		500: '#9ec1a3', 600: '#b2ceb6', 700: '#c5dac8', 800: '#d8e6db', 900: '#ecf3ed'
// 	},
// 	'tea_green': {
// 		DEFAULT: '#cfe0c3', 100: '#28371c', 200: '#4f6e39', 300: '#77a655', 400: '#a3c38b',
// 		500: '#cfe0c3', 600: '#d8e6cf', 700: '#e2ecdb', 800: '#ecf3e7', 900: '#f5f9f3'
// 	}
// }

const (
	Gunmetal      = "#1f363d"
	Cerulean      = "#40798c"
	Verdigris     = "#70a9a1"
	CambridgeBlue = "#9ec1a3"
	TeaGreen      = "#cfe0c3"
)

var (
	background_color = lipgloss.Color("#192b31")
	foreground_color = lipgloss.Color(TeaGreen)
	selected_color   = lipgloss.Color(Cerulean)
	cursor_color     = lipgloss.Color(Verdigris)
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

var color_subtle = lipgloss.Color("#626262")
var color_subtle_separator = lipgloss.Color("#3C3C3C")
var color_subtle_desc = lipgloss.Color("#4A4A4A")
