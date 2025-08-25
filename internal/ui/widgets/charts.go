package widgets

import (
	"math"
	"strings"
)

var blocks = []rune("▁▂▃▄▅▆▇█")

func Spark8(vals []float64, width int) string {
	if len(vals) == 0 || width <= 0 {
		return ""
	}
	// sample evenly over last vals
	step := float64(len(vals)) / float64(width)
	var b strings.Builder
	for i := 0; i < width; i++ {
		idx := int(math.Min(float64(len(vals)-1), math.Floor(float64(i)*step)))
		v := clamp01(vals[idx])
		level := int(math.Round(v * float64(len(blocks)-1)))
		if level < 0 {
			level = 0
		}
		if level > len(blocks)-1 {
			level = len(blocks) - 1
		}
		b.WriteRune(blocks[level])
	}
	return b.String()
}

func Bar(v float64, width int) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}

	fill := int(math.Round(v * float64(width)))

	if v > 0 && fill == 0 {
		fill = 1
	}

	if fill < 0 {
		fill = 0
	}
	if fill > width {
		fill = width
	}

	return strings.Repeat("█", fill) + strings.Repeat(" ", width-fill)
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
