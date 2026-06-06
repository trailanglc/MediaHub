package runtime

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Options controls which components RunAll starts.
type Options struct {
	SkipWorker bool
}

// RunAll starts API, scheduler and optionally worker until ctx is cancelled or a component fails.
func RunAll(ctx context.Context, s *Shared, opts Options) error {
	gctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var runErr error

	record := func(err error) {
		if err == nil || err == context.Canceled {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if runErr == nil {
			runErr = err
			cancel()
		}
	}

	s.Logger.Info("gateway starting components",
		zap.Bool("api", true),
		zap.Bool("scheduler", true),
		zap.Bool("worker", !opts.SkipWorker),
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		record(RunAPI(gctx, s))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		record(RunScheduler(gctx, s))
	}()

	if !opts.SkipWorker {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record(RunWorker(gctx, s))
		}()
	} else {
		s.Logger.Info("worker skipped", zap.String("reason", "SKIP_WORKER=1"))
	}

	<-ctx.Done()
	cancel()
	wg.Wait()

	if runErr != nil {
		return runErr
	}
	return ctx.Err()
}
