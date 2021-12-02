package resolver

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/shell"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/source"
	"github.com/rs/zerolog/log"
	"os"
	"path"
	"sync"
)

type Updater interface {
	// True if the chart exists in the source directory, else false.
	// Returns error in the event of an unexpected filesystem i/o error
	ChartExists(chartName string) (bool, error)
	// Run `helm dependency update` on the chart source dir if it has not already been done once by this Updater
	UpdateIfNeeded(chartName string) (string, error)
	// Returns version of the chart in Chart.yaml
	SourceVersion(chartName string) (string, error)
}

type updater struct {
	sourceDir string
	globalLock sync.Mutex
	updateLocks map[string]*sync.Mutex
	runner shell.Runner
}

func NewUpdater(sourceDir string, runner shell.Runner) Updater {
	return &updater{
		sourceDir: sourceDir,
		updateLocks: make(map[string]*sync.Mutex),
		runner: runner,
	}
}

func (u *updater) ChartExists(chartName string) (bool, error) {
	chartPath := u.chartSourcePath(chartName)
	_, err := os.Stat(chartPath)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		} else {
			return false, fmt.Errorf("error checking for chart at %s: %v", chartPath, err)
		}
	}

	return true, nil
}

func (u *updater) SourceVersion(chartName string) (string, error) {
	chart, err := u.getChart(chartName)
	if err != nil {
		return "", fmt.Errorf("error reading chart %s: %v", chartName, err)
	}

	return chart.ManifestVersion(), nil
}


func (u *updater) UpdateIfNeeded(chartName string) (string, error) {
	chart, err := u.getChart(chartName)
	if err != nil {
		return "", fmt.Errorf("error updating chart %s: %v", chartName, err)
	}

	lock, alreadyUpdated := u.getUpdateLock(chartName)
	if alreadyUpdated {
		log.Debug().Msgf("source copy of chart %s was already updated, won't update again", chartName)
		return chart.Path(), nil
	}

	lock.Lock()
	defer lock.Unlock()

	err = chart.UpdateDependencies()
	if err != nil {
		return "", fmt.Errorf("error updating chart source directory %s: %v", chart.Path(), err)
	}

	return chart.Path(), nil
}

// Returns source.Chart instance for the given chart name
func (u *updater) getChart(chartName string) (source.Chart, error) {
	return source.NewChart(u.chartSourcePath(chartName), u.runner)
}

// Returns lock for the given chart name, and a boolean (true if a lock already existed, false otherwise)
func (u *updater) getUpdateLock(chartName string) (*sync.Mutex, bool) {
	u.globalLock.Lock()
	defer u.globalLock.Unlock()

	_, exists := u.updateLocks[chartName]
	if !exists {
		u.updateLocks[chartName] = &sync.Mutex{}
	}

	return u.updateLocks[chartName], exists
}

func (u *updater) chartSourcePath(chartName string) string {
	return path.Join(u.sourceDir, chartName)
}
