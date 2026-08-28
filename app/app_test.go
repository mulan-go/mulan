package mulan_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/mulan-go/mulan/app"
)

func testAction(app *mulan.App) func(cmd *cobra.Command, _ []string) error {
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

		// close server after 10 seconds for test
		go func() {
			<-time.After(10 * time.Second)
			sig <- os.Interrupt
		}()

		switch <-sig {
		case os.Interrupt:
			app.Close()
		case syscall.SIGTERM:
			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			app.Shutdown(ctx)
		}
		return nil
	}
}

func TestApp(t *testing.T) {
	Conf = &Config{}
	meta := &mulan.Meta{
		ID:          "com.virzz.mulan.example",
		Name:        "example",
		Description: "ExampleService",
		Version:     Version,
		Commit:      Commit,
		BuildAt:     BuildAt,
	}
	std := mulan.New(meta, Conf)

	routerFunc := func(api gin.IRouter) {
		api.Handle("GET", "/", func(c *gin.Context) {
			c.String(200, "Hello, World!")
		})
	}
	webSrv := NewWebService(&Conf.Web, routerFunc)
	std.AddService(webSrv)

	if err := std.Execute(context.Background(), testAction(std)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
