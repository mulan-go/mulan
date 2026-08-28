package mulan

import (
	"context"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/mulan-go/mulan/app/service"
)

func UnmarshalConfigOpt(dc *mapstructure.DecoderConfig) { dc.TagName = "json" }

type (
	ActionFunc  func(cmd *cobra.Command, args []string) error
	PreInitFunc func(context.Context) error

	Meta struct {
		ID          string
		Name        string
		CNName      string
		Description string
		Version     string
		Commit      string
		BuildAt     string
	}
	App struct {
		*Meta
		debug   int
		rootCmd *cobra.Command
		preInit PreInitFunc
		log     *zap.Logger
		conf    any
		srvs    []service.Servicer

		versionCmd *cobra.Command
	}
)

func New(meta *Meta, cfg any) *App {
	_ = godotenv.Load()
	zap.ReplaceGlobals(zap.Must(zap.NewProduction()))

	std := &App{
		Meta: meta,
		conf: cfg,
		log:  zap.L(),
		srvs: make([]service.Servicer, 0),
		rootCmd: &cobra.Command{
			CompletionOptions: cobra.CompletionOptions{
				HiddenDefaultCmd: true,
			},
			SilenceErrors: true,
			SilenceUsage:  true,
			RunE: func(_ *cobra.Command, _ []string) error {
				panic("execute action not implemented")
			},
		},
	}
	std.internalCmd()
	return std
}

func (app *App) SetPreInit(f PreInitFunc) *App  { app.preInit = f; return app }
func (app *App) SetLogger(log *zap.Logger) *App { app.log = log; return app }
func (app *App) RootCmd() *cobra.Command        { return app.rootCmd }
func (app *App) Conf() any                      { return app.conf }

func (app *App) AddService(srvs ...service.Servicer) *App {
	app.srvs = append(app.srvs, srvs...)
	return app
}

func (app *App) AddCommand(cmd ...*cobra.Command) *App {
	for _, c := range cmd {
		if c.Use == "version" {
			app.log.Warn("internal command cannot be added", zap.String("command", c.Use))
			continue
		}
		app.rootCmd.AddCommand(c)
	}
	return app
}

// AddFlagSet 添加 FlagSet 到 rootCmd，可选绑定到 viper
func (app *App) AddFlagSet(fs ...*pflag.FlagSet) *App {
	flags := app.rootCmd.Flags()
	for _, f := range fs {
		flags.AddFlagSet(f)
	}
	return app
}

// BindFlags 绑定 rootCmd 的 flags 到 viper（可选调用）
func (app *App) BindFlags() *App {
	_ = viper.BindPFlags(app.rootCmd.PersistentFlags())
	_ = viper.BindPFlags(app.rootCmd.Flags())
	return app
}

func loadConfig(app *App) func(cmd *cobra.Command, args []string) (err error) {
	return func(cmd *cobra.Command, args []string) (err error) {
		fs := cmd.Flags()
		configPath, _ := fs.GetString("config")
		configSource := "env"
		if configPath != "" {
			configSource = "args"
			viper.SetConfigFile(configPath)
		} else {
			viper.AddConfigPath(".")
			viper.SetConfigName("config")
			viper.SetConfigType("")
		}
		err = viper.ReadInConfig()
		if err != nil {
			app.log.Warn("Failed to read config", zap.String("source", viper.ConfigFileUsed()), zap.Error(err))
			configSource = ""
		}
		// Viper config unmarshal to app.conf
		err = app.UnmarshalConfig(app.conf)
		if err != nil {
			app.log.Error("Failed to unmarshal config", zap.Error(err))
			return err
		}
		app.log.Info("Config loaded from", zap.String("source", configSource))
		return nil
	}
}

func (app *App) UnmarshalConfig(cfg any) error {
	return viper.Unmarshal(cfg, UnmarshalConfigOpt)
}

func (app *App) Execute(ctx context.Context, action ...ActionFunc) (err error) {
	// 添加内置 flags
	flags := pflag.NewFlagSet("app", pflag.ContinueOnError)
	flags.StringP("config", "c", "", "config file")
	flags.CountVarP(&app.debug, "debug", "d", "debug mode")
	_ = viper.BindPFlags(flags)
	app.rootCmd.PersistentFlags().AddFlagSet(flags)

	viper.SetEnvPrefix(app.Name)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	// 加载配置
	app.rootCmd.PreRunE = loadConfig(app)

	// Action
	var currentAction ActionFunc
	if len(action) > 0 && action[0] != nil {
		currentAction = action[0]
	} else {
		currentAction = defaultAction(app)
	}
	app.rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if app.preInit != nil {
			err := app.preInit(cmd.Context())
			if err != nil {
				return err
			}
		}
		return currentAction(cmd, args)
	}

	return app.rootCmd.ExecuteContext(ctx)
}
