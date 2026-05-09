package grammar

type Engine interface {
	Correct(text string) (string, error)
	IsAvailable() bool
}
