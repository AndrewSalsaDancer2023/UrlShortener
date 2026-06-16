package utils

import (
	"context"
	"errors"
	"sync"
)

type FinalizerFn struct {
	name string
	fn   func(context.Context) error
}

type Finalizer struct {
	once  sync.Once
	funcs []FinalizerFn
}

func (f *Finalizer) add(name string, fnc func(context.Context) error) {
	f.funcs = append(f.funcs, FinalizerFn{name: name, fn: fnc})
}

func (f *Finalizer) closeAll(ctx context.Context) error {
	var result error
	f.once.Do(func() {
		funcs := f.funcs
		f.funcs = nil

		var errs []error
		for i := len(funcs) - 1; i >= 0; i-- {
			f := funcs[i]
			if err := f.fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		result = errors.Join(errs...)
	})
	return result
}

var globalFinaizler = &Finalizer{}

func Add(name string, fnc func(context.Context) error) {
	globalFinaizler.funcs = append(globalFinaizler.funcs, FinalizerFn{name: name, fn: fnc})
}

func CloseAll(ctx context.Context) {
	globalFinaizler.closeAll(ctx)
}
