package mulan

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func defaultAction(app *App) ActionFunc {
	return func(cmd *cobra.Command, _ []string) error {
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

		go func() {
			err := app.Serve()
			if err != nil {
				zap.L().Error("Failed to run server", zap.Error(err))
				sig <- os.Interrupt
			}
		}()

		switch <-sig {
		case os.Interrupt: // ctrl+c
			app.Close()
		case syscall.SIGTERM:
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			app.Shutdown(ctx)
		}
		return nil
	}
}
