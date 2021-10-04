package chartfetch

import (
	"errors"
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/flock"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"os"
	"path"
	"time"
)

const chartFetchDesc = `
Fetch a chart from a Helm repo and unpack it into a user-supplied directory.

This command is a wrapper around the "helm fetch" command. In fact, it just runs
  helm fetch <chart> --version <version> -d /download/path/ --untar
but with added safety (using flock) for multiple concurrent invocations.

Examples:

# Download version 1.2.3 of the agora chart to /tmp/charts/agora-v1.2.3
chartfetch terra-helm/agora -v 1.2.3 -d /tmp/charts/agora-v1.2.3
`

// lockRetryInterval is the file lock retry interval
const lockRetryInterval = 100 * time.Millisecond

// lockTimeout is how long to wait to obtain a lock before giving up
const lockTimeout = 10 * time.Second

// Options for a chartfetch command
type Options struct {
	Version     string // Version of the chart that should be pulled (also passed directly to `helm fetch` )
	DownloadDir string // DownloadDir Path where chart should be downloaded; if it exists, the chart will not be downloaded again
}

// shellRunner an instance of a shell.Runner used to execute Helm commands.
var shellRunner shell.Runner = shell.NewRealRunner()

// init Initialize logging before anything else
// This way our Cobra error messages will be nicely formatted
func init() {
	initLogging()
}

// Execute is the main method/entrypoint for the chartfetch tool.
func Execute() {
	cobraCommand, err := newCobraCommand()
	if err != nil {
		log.Error().Msgf("Error initializing Cobra command: %v", err)
		os.Exit(1)
	}

	if err := cobraCommand.Execute(); err != nil {
		log.Error().Msgf("%v", err)
		os.Exit(1)
	}
}

// newCobraCommand Construct new Cobra command
func newCobraCommand() (*cobra.Command, error) {
	options := &Options{}
	cobraCommand := &cobra.Command{
		Use:   "chartfetch [chart URL | repo/chartname] [...]",
		Short: "Downloads and unpacks Helm charts to a given directory",
		Long:  chartFetchDesc,
		Args:  cobra.ExactArgs(1),

		// Only print out usage info when user supplies -h/--help
		SilenceUsage: true,

		// Don't print errors, we do it ourselves using a logging library
		SilenceErrors: true,

		// Main body of the command
		RunE: func(cmd *cobra.Command, args []string) error {
			return fetchChart(args[0], options)
		},
	}

	cobraCommand.Flags().StringVarP(&options.Version, "version", "v", "VERSION", "version of the chart to download. Required.")
	cobraCommand.Flags().StringVarP(&options.DownloadDir, "download-dir", "d", "path/to/download/to", "path where chart should be downloaded. Required.")
	if err := cobraCommand.MarkFlagRequired("version"); err != nil {
		return nil, err
	}
	if err := cobraCommand.MarkFlagRequired("download-dir"); err != nil {
		return nil, err
	}

	return cobraCommand, nil
}

// fetchChart actual command logic. Check if the chart directory exists. If so, download it.
func fetchChart(chart string, options *Options) error {
	// if download directory exists or we get an error checking for it, return
	if exists, err := checkDownloadDirExists(options.DownloadDir); exists || err != nil {
		return err
	}

	lockOptions := flock.Options{
		Path:          siblingPath(options.DownloadDir, ".lk", true),
		RetryInterval: lockRetryInterval,
		Timeout:       lockTimeout,
	}

	return flock.WithLock(lockOptions, func() error {
		// check for download directory again to make sure that it doesn't exist.
		// (someone else could have gotten the lock and made it in the mean time)
		if exists, err := checkDownloadDirExists(options.DownloadDir); exists || err != nil {
			return err
		}

		return shellRunner.Run(shell.Command{
			Prog: "helm",
			Args: []string{"fetch", chart, "--untar", "-d", options.DownloadDir},
		})
	})
}

// checkDownloadDir checks if the download directory exists
// returns true if it does
func checkDownloadDirExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		log.Info().Msgf("Download directory %s exists, won't download again", path)
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, fmt.Errorf("unexpected filesystem error: %v", err)
	}
}

// siblingPath is a path utility function. Given a directory path,
// it returns a path in the same parent directory, with the same basename,
// with a user-supplied suffix. The new path may optionally be hidden.
//
// Eg.
//  siblingPath("/tmp/foo", ".lk", true)
// ->
// "/tmp/.foo.lk"
//
// siblingPath("/tmp/foo", "-scratch", false)
// ->
// "/tmp/foo-scratch"
func siblingPath(relpath string, suffix string, hidden bool) string {
	cleaned := path.Clean(relpath)
	parent := path.Dir(cleaned)
	base := path.Base(cleaned)

	var prefix string
	if hidden {
		prefix = "."
	}

	siblingBase := fmt.Sprintf("%s%s%s", prefix, base, suffix)
	return path.Join(parent, siblingBase)
}

// Initialize logging
func initLogging() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}
