package mulan

import (
	"bytes"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func versionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Aliases: []string{"v"},
		Short:   "Show Version",
		Run: func(cmd *cobra.Command, args []string) {
			if app.Commit == "" || app.BuildAt == "" {
				bi, ok := debug.ReadBuildInfo()
				if ok {
					for _, setting := range bi.Settings {
						if setting.Key == "vcs.revision" && app.Commit == "" {
							app.Commit = setting.Value
						}
						if setting.Key == "vcs.time" && app.BuildAt == "" {
							app.BuildAt = setting.Value
						}
					}
				}
			}
			buf := bytes.NewBuffer(nil)
			buf.WriteString(app.Name)
			buf.WriteByte('\n')
			if app.CNName != "" {
				buf.WriteString(app.CNName)
				buf.WriteByte('\n')
			}
			if app.ID != "" {
				buf.WriteString(app.ID)
				buf.WriteByte('\n')
			}
			if app.Version != "" {
				buf.WriteString(app.Version)
				buf.WriteByte('\n')
			}
			if app.Commit != "" {
				buf.WriteString(app.Commit)
				buf.WriteByte('\n')
			}
			if app.BuildAt != "" {
				buf.WriteString(app.BuildAt)
				buf.WriteByte('\n')
			}
			if app.Description != "" {
				buf.WriteString(app.Description)
				buf.WriteByte('\n')
			}
			fmt.Fprintln(os.Stdout, buf.String())
		},
	}
}

func (app *App) SetVersionCmd(cmd *cobra.Command) *App {
	app.rootCmd.RemoveCommand(app.versionCmd)
	app.versionCmd = cmd
	app.internalCmd()
	return app
}

func (app *App) internalCmd() {
	if app.versionCmd == nil {
		app.versionCmd = versionCmd(app)
	}
	app.rootCmd.AddCommand(app.versionCmd)
}
