package tui

import (
	"fmt"
	"sort"
	"strings"
)

type rgb struct{ r, g, b int }

// Theme is a complete terminal palette. State always has a textual marker;
// these colors reinforce meaning but never carry it alone.
type Theme struct {
	Name       string
	Background rgb
	Text       rgb
	Accent     rgb
	Success    rgb
	Warning    rgb
	Failure    rgb
	Muted      rgb
}

var themes = map[string]Theme{
	"night-flight": {
		Name: "night-flight", Background: rgb{7, 17, 31}, Text: rgb{216, 226, 240},
		Accent: rgb{110, 168, 255}, Success: rgb{100, 212, 155},
		Warning: rgb{242, 185, 75}, Failure: rgb{255, 107, 107}, Muted: rgb{130, 144, 164},
	},
	"paper-trail": {
		Name: "paper-trail", Background: rgb{245, 240, 228}, Text: rgb{32, 37, 44},
		Accent: rgb{36, 93, 150}, Success: rgb{52, 122, 69},
		Warning: rgb{133, 87, 0}, Failure: rgb{178, 58, 58}, Muted: rgb{95, 100, 98},
	},
	"high-contrast": {
		Name: "high-contrast", Background: rgb{0, 0, 0}, Text: rgb{255, 255, 255},
		Accent: rgb{0, 220, 255}, Success: rgb{80, 255, 100},
		Warning: rgb{255, 230, 0}, Failure: rgb{255, 70, 70}, Muted: rgb{190, 190, 190},
	},
}

var themeAliases = map[string]string{
	"night": "night-flight", "paper": "paper-trail", "contrast": "high-contrast",
}

// ParseTheme resolves a theme name or its short alias.
func ParseTheme(name string) (Theme, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "night-flight"
	}
	if canonical, ok := themeAliases[name]; ok {
		name = canonical
	}
	theme, ok := themes[name]
	if !ok {
		return Theme{}, fmt.Errorf("unknown theme %q (choose %s)", name, strings.Join(ThemeNames(), ", "))
	}
	return theme, nil
}

// ThemeNames returns the stable canonical theme names.
func ThemeNames() []string {
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ansiForeground(c rgb) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.r, c.g, c.b)
}

func ansiCanvas(t Theme) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%d;38;2;%d;%d;%dm", t.Background.r, t.Background.g, t.Background.b, t.Text.r, t.Text.g, t.Text.b)
}
