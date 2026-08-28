package mulan

import (
	"context"
	"reflect"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func (app *App) Serve() error {
	wg := errgroup.Group{}
	for _, srv := range app.srvs {
		if app.debug > 0 {
			app.log.Debug("Serving service", zap.String("name", reflect.TypeOf(srv).String()))
		}
		wg.Go(srv.Serve)
	}
	return wg.Wait()
}

func (app *App) Close() error {
	wg := errgroup.Group{}
	for _, srv := range app.srvs {
		if app.debug > 0 {
			app.log.Debug("Closing service", zap.String("name", reflect.TypeOf(srv).String()))
		}
		wg.Go(srv.Close)
	}
	return wg.Wait()
}

func (app *App) Shutdown(ctx context.Context) error {
	wg := errgroup.Group{}
	for _, srv := range app.srvs {
		if app.debug > 0 {
			app.log.Debug("Shutting down service", zap.String("name", reflect.TypeOf(srv).String()))
		}
		wg.Go(func() error { return srv.Shutdown(ctx) })
	}
	return wg.Wait()
}
