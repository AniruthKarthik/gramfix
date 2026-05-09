package platform

import (
	"errors"

	"github.com/user/gramfix/internal/adapters/clipboard"
	"github.com/user/gramfix/internal/adapters/injector"
	"github.com/user/gramfix/internal/adapters/grammar"
)

type Adapters struct {
	Clipboard clipboard.Clipboard
	Injector  injector.Injector
	Grammar   grammar.Engine
}

func Detect() (*Adapters, error) {
	adapters := &Adapters{}

	// Detect Clipboard
	isWayland := false
	waylandClip := &clipboard.WaylandClipboard{}
	if waylandClip.IsAvailable() {
		adapters.Clipboard = waylandClip
		isWayland = true
	} else {
		x11Clip := &clipboard.X11Clipboard{}
		if x11Clip.IsAvailable() {
			adapters.Clipboard = x11Clip
		} else {
			return nil, errors.New("no supported clipboard provider found (tried wl-clipboard, xclip, xsel)")
		}
	}

	// Detect Injector
	if isWayland {
		wtype := &injector.WtypeInjector{}
		if wtype.IsAvailable() {
			adapters.Injector = wtype
		} else {
			ydotool := &injector.YdotoolInjector{}
			if ydotool.IsAvailable() {
				adapters.Injector = ydotool
			} else {
				return nil, errors.New("no supported text injector found for Wayland (tried wtype, ydotool)")
			}
		}
	} else {
		xdotool := &injector.XdotoolInjector{}
		if xdotool.IsAvailable() {
			adapters.Injector = xdotool
		} else {
			ydotool := &injector.YdotoolInjector{}
			if ydotool.IsAvailable() {
				adapters.Injector = ydotool
			} else {
				return nil, errors.New("no supported text injector found for X11 (tried xdotool, ydotool)")
			}
		}
	}

	// Detect Grammar Engine
	lt := &grammar.LanguageToolEngine{}
	if lt.IsAvailable() {
		adapters.Grammar = lt
	} else {
		return nil, errors.New("LanguageTool not found")
	}

	return adapters, nil
}
