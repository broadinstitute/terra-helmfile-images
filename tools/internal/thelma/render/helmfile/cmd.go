package helmfile

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/gitops"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/render/helmfile/argocd"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/shell"
	"sort"
	"strings"
)

// ProgName is the name of the `helmfile` binary
const ProgName = "helmfile"

// Environment variables -- prefixed with THF for "terra-helmfile", used to pass in information to helmfile
const TargetTypeEnvVar = "THF_TARGET_TYPE"
const TargetBaseEnvVar = "THF_TARGET_BASE"
const TargetNameEnvVar = "THF_TARGET_NAME"
const ReleaseNameEnvVar = "THF_RELEASE_NAME"
const ReleaseTypeEnvVar = "THF_RELEASE_TYPE"
const NamespaceEnvVar = "THF_NAMESPACE"
const ClusterAddressEnvVar = "THF_CLUSTER_ADDRESS"
const ClusterNameEnvVar = "THF_CLUSTER_NAME"
const ArgocdProjectEnvVar = "THF_ARGOCD_PROJECT"
const ChartPathEnvVar = "THF_CHART_PATH"
const AppVersionEnvVar = "THF_APP_VERSION"

// Cmd encapsulates low-level parameters for a `helmfile` command
type Cmd struct {
	dir         string
	skipDeps    bool
	logLevel    string
	envVars     []string
	stateValues map[string]string
	selectors   map[string]string
	valuesFiles []string
	outputDir   string
	stdout      bool
}

// newCmd returns a new Cmd object with all fields initialized
func newCmd() *Cmd {
	return &Cmd{
		stateValues: make(map[string]string),
		selectors:   make(map[string]string),
	}
}

func (cmd *Cmd) toShellCommand() shell.Command {
	// Convert helmfile parameters into cli arguments
	var cliArgs []string

	if cmd.logLevel != "" {
		cliArgs = append(cliArgs, fmt.Sprintf("--log-level=%s", cmd.logLevel))
	}

	if len(cmd.selectors) != 0 {
		selectorString := joinKeyValuePairs(cmd.selectors)
		cliArgs = append(cliArgs, fmt.Sprintf("--selector=%s", selectorString))
	}

	if len(cmd.stateValues) != 0 {
		stateValuesString := joinKeyValuePairs(cmd.stateValues)
		cliArgs = append(cliArgs, fmt.Sprintf("--state-values-set=%s", stateValuesString))
	}

	// Append Helmfile command we're running (template)
	cliArgs = append(cliArgs, "template")

	// Append arguments specific to template subcommand
	if cmd.skipDeps {
		// Skip dependencies unless we're rendering a local chart, to save time
		cliArgs = append(cliArgs, "--skip-deps")
	}
	if len(cmd.valuesFiles) > 0 {
		cliArgs = append(cliArgs, fmt.Sprintf("--values=%s", strings.Join(cmd.valuesFiles, ",")))
	}

	if !cmd.stdout {
		outputDirFlag := fmt.Sprintf("--output-dir=%s", cmd.outputDir)
		cliArgs = append(cliArgs, outputDirFlag)
	}

	shellCmd := shell.Command{
		Prog: ProgName,
		Args: cliArgs,
		Dir:  cmd.dir,
		Env:  cmd.envVars,
	}

	return shellCmd
}

func (cmd *Cmd) setTargetEnvVars(t gitops.Target) {
	cmd.addEnvVar(TargetTypeEnvVar, t.Type().String())
	cmd.addEnvVar(TargetBaseEnvVar, t.Base())
	cmd.addEnvVar(TargetNameEnvVar, t.Name())
}

func (cmd *Cmd) setReleaseEnvVars(r gitops.Release) {
	cmd.addEnvVar(ReleaseNameEnvVar, r.Name())
	cmd.addEnvVar(ReleaseTypeEnvVar, r.Type().String())
}

func (cmd *Cmd) setNamespaceEnvVar(r gitops.Release) {
	cmd.addEnvVar(NamespaceEnvVar, r.Namespace())
}

func (cmd *Cmd) setClusterEnvVars(r gitops.Release) {
	cmd.addEnvVar(ClusterNameEnvVar, r.ClusterName())
	cmd.addEnvVar(ClusterAddressEnvVar, r.ClusterAddress())
}

func (cmd *Cmd) setArgocdProjectEnvVar(t gitops.Target) {
	cmd.addEnvVar(ArgocdProjectEnvVar, argocd.GetProjectName(t))
}

func (cmd *Cmd) setDir(dir string) {
	cmd.dir = dir
}

func (cmd *Cmd) setLogLevel(logLevel string) {
	cmd.logLevel = logLevel
}

func (cmd *Cmd) setSkipDeps(skipDeps bool) {
	cmd.skipDeps = skipDeps
}

func (cmd *Cmd) setOutputDir(outputDir string) {
	cmd.outputDir = outputDir
}

func (cmd *Cmd) setStdout(stdout bool) {
	cmd.stdout = stdout
}

func (cmd *Cmd) addValuesFiles(valuesFiles ...string) {
	cmd.valuesFiles = append(cmd.valuesFiles, valuesFiles...)
}

func (cmd *Cmd) setChartPathEnvVar(chartPath string) {
	cmd.addEnvVar(ChartPathEnvVar, chartPath)
}

func (cmd *Cmd) setAppVersionEnvVar(appVersion string) {
	cmd.addEnvVar(AppVersionEnvVar, appVersion)
}

// addEnvVar adds an env var key/value pair to the given cmd instance
func (cmd *Cmd) addEnvVar(name string, value string) {
	cmd.envVars = append(cmd.envVars, fmt.Sprintf("%s=%s", name, value))
}

// joinKeyValuePairs joins map[string]string to string containing comma-separated key-value pairs.
// Eg. { "a": "b", "c": "d" } -> "a=b,c=d"
func joinKeyValuePairs(pairs map[string]string) string {
	var tokens []string
	for k, v := range pairs {
		tokens = append(tokens, fmt.Sprintf("%s=%s", k, v))
	}

	// Sort tokens so they are always supplied in predictable order
	sort.Strings(tokens)

	return strings.Join(tokens, ",")
}
