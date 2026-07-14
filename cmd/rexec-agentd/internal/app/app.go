// Package app holds the remote-exec configuration and construction.
package app

import "github.com/inovacc/mantle/bootstrap"

// App is the remote-exec daemon config, embedding mantle's Base.
type App struct {
	bootstrap.Base `mapstructure:",squash" yaml:",inline"`
}

// New returns App seeded with defaults.
func New() *App {
	a := &App{Base: bootstrap.DefaultBase()}
	return a
}
