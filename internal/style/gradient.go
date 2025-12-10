package style

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

// blendColors returns a slice of colors blended between the given stops.
// Blending is done in Hcl to stay in gamut.
func blendColors(size int, stops ...color.Color) []color.Color {
	if len(stops) < 2 {
		return nil
	}

	stopsPrime := make([]colorful.Color, len(stops))
	for i, k := range stops {
		stopsPrime[i], _ = colorful.MakeColor(k)
	}

	numSegments := len(stopsPrime) - 1
	blended := make([]color.Color, 0, size)

	// Calculate how many colors each segment should have.
	segmentSizes := make([]int, numSegments)
	baseSize := size / numSegments
	remainder := size % numSegments

	// Distribute the remainder across segments.
	for i := range numSegments {
		segmentSizes[i] = baseSize
		if i < remainder {
			segmentSizes[i]++
		}
	}

	// Generate colors for each segment.
	for i := range numSegments {
		c1 := stopsPrime[i]
		c2 := stopsPrime[i+1]
		segmentSize := segmentSizes[i]

		for j := range segmentSize {
			var t float64
			if segmentSize > 1 {
				t = float64(j) / float64(segmentSize-1)
			}
			c := c1.BlendHcl(c2, t)
			blended = append(blended, c)
		}
	}

	return blended
}

// ApplyBoldForegroundGrad renders a given string with a horizontal gradient
// foreground and bold styling.
func ApplyBoldForegroundGrad(input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}

	// Split into grapheme clusters for proper unicode handling.
	var clusters []string
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, string(gr.Runes()))
	}

	if len(clusters) == 1 {
		c, _ := colorful.MakeColor(color1)
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Hex())).Render(input)
	}

	ramp := blendColors(len(clusters), color1, color2)
	for i, c := range ramp {
		cf, _ := colorful.MakeColor(c)
		clusters[i] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cf.Hex())).Render(clusters[i])
	}

	var o strings.Builder
	for _, c := range clusters {
		o.WriteString(c)
	}
	return o.String()
}
