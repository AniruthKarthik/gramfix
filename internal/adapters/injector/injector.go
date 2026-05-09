package injector

import "errors"

var ErrNoInjectorProvider = errors.New("no compatible injector provider found")

type Injector interface {
	SendPaste() error
	SendCopy() error
	IsAvailable() bool
}
