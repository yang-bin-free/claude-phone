package engine

type powerInhibitor interface {
	Acquire() error
	Release() error
	Active() bool
}
