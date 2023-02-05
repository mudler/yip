package herd

import (
	"context"
	"sync"
)

type OpState struct {
	sync.Mutex
	fn         []func(context.Context) error
	err        error
	fatal      bool
	background bool
	weak       bool
}

func (o *OpState) toGraphEntry(name string) GraphEntry {
	return GraphEntry{
		WithCallback: o.fn != nil,
		Callback:     o.fn,
		Error:        o.err,
		Background:   o.background,
		WeakDeps:     o.weak,
		Fatal:        o.fatal,
		Name:         name,
	}
}
