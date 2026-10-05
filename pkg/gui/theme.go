package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// PadrTheme provides a modern, sleek developer-focused dark palette (Linear / Vercel style)
type PadrTheme struct{}

var _ fyne.Theme = (*PadrTheme)(nil)

func (t *PadrTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 11, G: 15, B: 23, A: 255} // #0B0F17 deep slate
	case theme.ColorNameButton:
		return color.RGBA{R: 26, G: 34, B: 50, A: 255} // #1A2232
	case theme.ColorNameDisabledButton:
		return color.RGBA{R: 18, G: 22, B: 32, A: 255}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 71, G: 85, B: 105, A: 255} // #475569
	case theme.ColorNameError:
		return color.RGBA{R: 239, G: 68, B: 68, A: 255} // #EF4444
	case theme.ColorNameFocus:
		return color.RGBA{R: 99, G: 102, B: 241, A: 120} // #6366F1 accent
	case theme.ColorNameForeground:
		return color.RGBA{R: 248, G: 250, B: 252, A: 255} // #F8FAFC
	case theme.ColorNameHover:
		return color.RGBA{R: 35, G: 46, B: 68, A: 255}
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 15, G: 20, B: 30, A: 255} // #0F141E
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 100, G: 116, B: 139, A: 255} // #64748B
	case theme.ColorNamePressed:
		return color.RGBA{R: 45, G: 58, B: 85, A: 255}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 99, G: 102, B: 241, A: 255} // #6366F1 Indigo primary
	case theme.ColorNameScrollBar:
		return color.RGBA{R: 40, G: 50, B: 72, A: 200}
	case theme.ColorNameShadow:
		return color.RGBA{R: 0, G: 0, B: 0, A: 80}
	case theme.ColorNameSuccess:
		return color.RGBA{R: 16, G: 185, B: 129, A: 255} // #10B981 Emerald
	case theme.ColorNameWarning:
		return color.RGBA{R: 245, G: 158, B: 11, A: 255} // #F59E0B Amber
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (t *PadrTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *PadrTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *PadrTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputRadius:
		return 6
	default:
		return theme.DefaultTheme().Size(name)
	}
}
