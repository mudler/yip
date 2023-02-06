package herd

import "context"

// OpOption defines the operation settings.
type OpOption func(string, *OpState, *Graph) error

var NoOp OpOption = func(s string, os *OpState, g *Graph) error { return nil }

// FatalOp makes the operation fatal.
// Any error will make the DAG to stop and return the error immediately.
var FatalOp OpOption = func(key string, os *OpState, g *Graph) error {
	os.fatal = true
	return nil
}

// Background runs the operation in the background.
var Background OpOption = func(key string, os *OpState, g *Graph) error {
	os.background = true
	return nil
}

// WeakDeps sets the dependencies of the job as "weak".
// Any failure of the jobs which depends on won't impact running the job.
// By default, a failure job will make also fail all the children - this is option
// disables this behavor and make the child start too.
var WeakDeps OpOption = func(key string, os *OpState, g *Graph) error {
	os.weak = true
	return nil
}

// WithDeps defines an operation dependency.
// Dependencies can be expressed as a string.
// Note: before running the DAG you must define all the operations.
func WithDeps(deps ...string) OpOption {
	return func(key string, os *OpState, g *Graph) error {
		for _, d := range deps {
			if err := g.Graph.DependOn(key, d); err != nil {
				return err
			}
		}
		return nil
	}
}

// ConditionalOption defines an option that is enabled only if the
// conditional callback returns true.
func ConditionalOption(condition func() bool, op OpOption) OpOption {
	if condition() {
		return op
	}

	return NoOp
}

// WithCallback associates a callback to the operation to be executed
// when the DAG is walked-by.
func WithCallback(fn ...func(context.Context) error) OpOption {
	return func(s string, os *OpState, g *Graph) error {
		os.fn = append(os.fn, fn...)
		return nil
	}
}
