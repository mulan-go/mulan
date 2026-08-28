package mulan_test

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/mulan-go/mulan/app"
)

func TestSetVersionCmd(t *testing.T) {
	app := mulan.New(&mulan.Meta{Name: "test"}, &struct{}{})
	called := false
	cmd := &cobra.Command{
		Use: "version",
		Run: func(_ *cobra.Command, _ []string) {
			called = true
		},
	}

	app.SetVersionCmd(cmd)
	app.RootCmd().SetArgs([]string{"version"})
	if err := app.RootCmd().Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("custom version command was not executed")
	}
}
