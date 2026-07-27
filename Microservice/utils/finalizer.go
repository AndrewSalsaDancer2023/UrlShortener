package utils

import (
	"context"
	"errors"
	"sync"
)

type FinalizerFn struct {
	name string
	// fn   func(context.Context) error
	fn func() error
}

type Finalizer struct {
	once  sync.Once
	funcs []FinalizerFn
}

func (f *Finalizer) add(name string, fnc func() error) {
	f.funcs = append(f.funcs, FinalizerFn{name: name, fn: fnc})
}

func (f *Finalizer) closeAll(ctx context.Context) error {
	var result error
	f.once.Do(func() {
		funcs := f.funcs
		f.funcs = nil

		var errBuf SafeErrorBuffer
		var wg sync.WaitGroup
		// var errs []error

		for i := len(funcs) - 1; i >= 0; i-- {
			ShutdownResourceParallel(ctx, &wg, &errBuf, funcs[i].name, funcs[i].fn)
			// f := funcs[i]
			// if err := f.fn(); err != nil {
			// 	errs = append(errs, err)
			// }
		}
		// result = errors.Join(errs...)
		wg.Wait()
		result = errors.Join(errBuf.GetErrors()...)
	})
	return result
}

var globalFinaizler = &Finalizer{}

func AddFinalizerFunction(name string, fnc func() error) {
	globalFinaizler.funcs = append(globalFinaizler.funcs, FinalizerFn{name: name, fn: fnc})
}

func CloseAll(ctx context.Context) error {
	return globalFinaizler.closeAll(ctx)
}
