package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// PadrTheme provides a high-contrast modern dark palette
type PadrTheme struct{}

var _ fyne.Theme = (*PadrTheme)(nil)

func (t *PadrTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 15, G: 23, B: 42, A: 255} // #0F172A slate-900
	case theme.ColorNameButton:
		return color.RGBA{R: 30, G: 41, B: 59, A: 255} // #1E293B slate-800
	case theme.ColorNameDisabledButton:
		return color.RGBA{R: 20, G: 28, B: 42, A: 255}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 148, G: 163, B: 184, A: 255} // #94A3B8 slate-400 (clearly visible!)
	case theme.ColorNameError:
		return color.RGBA{R: 244, G: 63, B: 94, A: 255} // #F43F5E rose-500
	case theme.ColorNameFocus:
		return color.RGBA{R: 99, G: 102, B: 241, A: 180} // #6366F1 indigo-500
	case theme.ColorNameForeground:
		return color.RGBA{R: 248, G: 250, B: 252, A: 255} // #F8FAFC slate-50 (pure high contrast white)
	case theme.ColorNameHover:
		return color.RGBA{R: 51, G: 65, B: 85, A: 255} // #334155 slate-700
	case theme.ColorNameInputBackground:
		return color.RGBA{R: 30, G: 41, B: 59, A: 255} // #1E293B (distinct, crisp contrast against background!)
	case theme.ColorNamePlaceHolder:
		return color.RGBA{R: 148, G: 163, B: 184, A: 255} // #94A3B8
	case theme.ColorNamePressed:
		return color.RGBA{R: 71, G: 85, B: 105, A: 255}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 99, G: 102, B: 241, A: 255} // #6366F1 Indigo primary
	case theme.ColorNameScrollBar:
		return color.RGBA{R: 71, G: 85, B: 105, A: 200}
	case theme.ColorNameShadow:
		return color.RGBA{R: 0, G: 0, B: 0, A: 100}
	case theme.ColorNameSuccess:
		return color.RGBA{R: 16, G: 185, B: 129, A: 255} // #10B981 emerald
	case theme.ColorNameWarning:
		return color.RGBA{R: 245, G: 158, B: 11, A: 255} // #F59E0B amber
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
		return 10
	case theme.SizeNameInlineIcon:
		return 20
	case theme.SizeNameScrollBar:
		return 10
	case theme.SizeNameText:
		return 14
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameInputRadius:
		return 6
	default:
		return theme.DefaultTheme().Size(name)
	}
}
