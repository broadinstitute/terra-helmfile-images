package cli

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app/config"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

// commandName name of the thelma tool
const commandName = "thelma"

// envPrefix prefix to use for configuration environment variables.
const envPrefix = commandName // prefix is automatically capitalized by Viper to "THELMA"

// defaultLogLevel default level for logging, valid options are whatever zerolog accepts (eg. "debug", "trace")
const defaultLogLevel = "info"

// configRepoName name to use to refer to terra-helmfile config repo
const configRepoName = "terra-helmfile"

// globalUsage common usage string printed for all subcommands
const globalUsage = `CLI tools for interacting with Terra's Helm charts

Environment variables:
| Name                               | Description                                                                       |
|------------------------------------|-----------------------------------------------------------------------------------|
| $THELMA_HOME                       | Required. Path to terra-helmfile clone.                                           |
| $THELMA_LOGLEVEL                   | Logging verbosity. One of error, warn, info (default), debug, or trace            |
| $THELMA_TMPDIR                     | Path where Thelma should generate temporary files. Defaults to OS tmp dir.        |
`

func init() {
	// Initialize logging
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// ThelmaContext is used to share state with subcommands
type ThelmaContext struct {
	app *app.ThelmaApp
}

// ThelmaCLI represents a complete command-line interface for Thelma, including subcommands
type ThelmaCLI struct {
	context         *ThelmaContext
	rootCommand     *cobra.Command
	configOverrides map[string]interface{}
	shellRunner     shell.Runner
	chartsCLI       *chartsCLI
	renderCLI       *renderCLI
}

// Execute is the main method/entrypoint for Thelma
func Execute() {
	cli := newThelmaCLI()

	if err := cli.execute(); err != nil {
		log.Error().Msgf("%v", err)
		os.Exit(1)
	}
}

// execute executes a command
func (cli *ThelmaCLI) execute() error {
	return cli.rootCommand.Execute()
}

// setArgs (for use in tests only) sets command-line arguments on the cobra command
func (cli *ThelmaCLI) setArgs(args []string) {
	cli.rootCommand.SetArgs(args)
}

// setHome (for use in tests only) makes it possible to set THELMA_HOME to
// a custom path for testing
func (cli *ThelmaCLI) setHome(path string) {
	cli.configOverrides[config.Keys.Home] = path
}

// setLogLevel (for use in tests only) makes it possible to set THELMA_LOGLEVEL to
// a custom value for testing
func (cli *ThelmaCLI) setLogLevel(level string) {
	cli.configOverrides[config.Keys.LogLevel] = level
}

// settShellRunner (for use in tests only) configures this CLI instance to use the given shell runner
func (cli *ThelmaCLI) setShellRunner(runner shell.Runner) {
	cli.shellRunner = runner
}

// newThelmaCLI constructs a new Thelma CLI
func newThelmaCLI() *ThelmaCLI {
	ctx := &ThelmaContext{}
	cfgOverrides := make(map[string]interface{})

	rootCommand := &cobra.Command{
		Use:           commandName,
		Short:         "CLI tools for Terra Helm",
		Long:          globalUsage,
		SilenceUsage:  true, // Only print out usage error when user supplies -h/--help
		SilenceErrors: true, // Don't print errors, we do it ourselves using a logging library
	}

	cli := ThelmaCLI{
		context:         ctx,
		configOverrides: cfgOverrides,
		rootCommand:     rootCommand,
		chartsCLI:       newChartsCLI(ctx),
		renderCLI:       newRenderCLI(ctx),
	}

	// Use a PersistentPreRunE hook to initialize config, logging, etc before all child commands run.
	rootCommand.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(cfgOverrides)
		if err != nil {
			return err
		}

		_app, err := app.NewWithOptions(cfg, app.Options{Runner: cli.shellRunner})
		if err != nil {
			return err
		}

		// Add app to context so subcommands can access it
		ctx.app = _app

		// Set log level
		setLogLevel(ctx.app.Config.LogLevel())

		return nil
	}

	rootCommand.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		return ctx.app.Paths.Cleanup()
	}

	// Add subcommands
	rootCommand.AddCommand(
		cli.chartsCLI.cobraCommand,
		cli.renderCLI.cobraCommand,
	)

	return &cli
}

// loadConfig builds a new Thelma config
func loadConfig(overrides map[string]interface{}) (*config.Config, error) {
	_viper := viper.New()

	// Set defaults
	_viper.SetDefault(config.Keys.Home, "")
	_viper.SetDefault(config.Keys.LogLevel, defaultLogLevel)
	_viper.SetDefault(config.Keys.Tmpdir, os.TempDir())

	// Configure Viper:
	// automatically interpret env vars prefixed with THELMA_ as config settings
	_viper.SetEnvPrefix(envPrefix)
	// map dashes to underscores ("THELMA_LOG_LEVEL" is mapped to the key "log-level")
	_viper.SetEnvKeyReplacer(configKeyReplacer())
	// automatically load config values from environment
	_viper.AutomaticEnv()

	// apply configuration overrides (these are used in tests)
	for k, v := range overrides {
		_viper.Set(k, v)
	}

	// Validation
	// Make sure home dir is configured and exists
	homePath := _viper.GetString(config.Keys.Home)
	if homePath == "" {
		return nil, fmt.Errorf("please specify path to %s clone via the environment variable %s", configRepoName, configKeyToEnvVar(config.Keys.Home))
	}
	fullPath, err := expandAndVerifyExists(homePath, fmt.Sprintf("%s clone", configRepoName))
	if err != nil {
		return nil, err
	}
	_viper.Set(config.Keys.Home, fullPath)

	// Make sure log level is valid
	logLevel := _viper.GetString(config.Keys.LogLevel)
	if _, err := zerolog.ParseLevel(logLevel); err != nil {
		log.Warn().Msgf("Invalid log level %v, setting to %s", logLevel, defaultLogLevel)
		_viper.Set(config.Keys.LogLevel, defaultLogLevel)
	}

	// Convert viper config to a simple immutable config struct and return
	cfg := config.Data{}
	if err := _viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error loading configuration: %v", err)
	}
	return config.New(cfg), nil
}

// Adjust logging verbosity based on CLI options
func setLogLevel(levelStr string) {
	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to parse log level %q: %v", levelStr, err)
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		return
	}
	zerolog.SetGlobalLevel(level)
}

// configKeyToEnvVar transform config key to corresponding environment variable
// eg.
// "log-level" -> "THELMA_LOG_LEVEL"
func configKeyToEnvVar(key string) string {
	prefix := strings.ToUpper(envPrefix)
	key = configKeyReplacer().Replace(key)
	key = strings.ToUpper(key)
	return fmt.Sprintf("%s_%s", prefix, key)
}

// configKeyReplacer returns a string replacer that substitutes "-" with "_"
func configKeyReplacer() *strings.Replacer {
	return strings.NewReplacer("-", "_")
}

// Expand relative path to absolute.
// This is necessary for many arguments because Helmfile assumes paths
// are relative to helmfile.yaml and we want them to be relative to CWD.
func expandAndVerifyExists(filePath string, description string) (string, error) {
	expanded, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(expanded); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist: %s", description, expanded)
	} else if err != nil {
		return "", fmt.Errorf("error reading %s %s: %v", description, expanded, err)
	}

	return expanded, nil
}
