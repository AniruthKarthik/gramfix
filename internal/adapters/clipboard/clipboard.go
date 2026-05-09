package clipboard

import "errors"

var ErrNoClipboardProvider = errors.New("no compatible clipboard provider found")

type Clipboard interface {
	GetPrimary() (string, error)
	GetClipboard() (string, error)
	SetClipboard(text string) error
	IsAvailable() bool
}
