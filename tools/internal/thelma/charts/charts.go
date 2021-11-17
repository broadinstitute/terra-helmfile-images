package charts

import (
	"fmt"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/dependency"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo/gcs"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/source"
	"github.com/rs/zerolog/log"
	"strings"
)

// TODO:

//* on push to master:
//* if the commit that triggered this was authored by broadbot and includes the message "chart release", do nothing.
//* identify changed charts + their dependents.
//  `git diff --name-only HEAD~1`.
//  `git commit -m 'foo'`
//* git diff --name-only HEAD~1 charts/
// ^ The above will be done by a GitHub action shell script, which will then pass the list of charts in using:
//
// thelma charts publish [charts...]
// Eg.
// thelma charts publish agora buffer sherlock-reporter
//
// What this does:
//  * for each chart, in TOPOLOGICAL order:
//    * bump version in Chart.yaml if needed.
//        what if instead of looking at git history, we look at the index.yaml for the official Helm repo?
//        this is less error-prone, we can just parse the yaml.
//    * run `helm dependency build`.
//        (this will populate charts/ subdirectory).
//    * run `helm docs` to regenerate README in all charts
//    * build chart
//        helm chart package into tmp dir
//    * publish chart to GCS:
//        download index.yaml into tmp dir
//        run helm repo index --update to update the index
//    * bump chart version in versions/app/dev.yaml or versions/cluster/dev.yaml IF autorelease is enabled.
//  * commit all changes in git to master.
//    * Question: What happens if another change is merged to master at exactly the same time?
//      * We'll run into git conflicts, right?
//      * Keep in mind that the doc commits and version updates don't really matter so much...
//      * OKAY GOOD NEWS! GHA now supports a concurrency key. Whew.
//        * Do we still want a GCS lock in place for Helm repo updates?
//* commit all changes in git to master.
//* this MUST support a dry-run mode, where we don't actually publish to GCS or commit changes.

func Publish(chartsToPublish []string, bucketName string, sourceDir string, app *app.ThelmaApp, dryRun bool) error {
	src, err := source.NewSourceDirectory(sourceDir, app.ShellRunner)
	if err != nil {
		return err
	}

	for _, chart := range chartsToPublish {
		if !src.HasChart(chart) {
			return fmt.Errorf("chart %q does not exist in source dir %s", chart, sourceDir)
		}
	}

	depGraph, err := dependency.NewGraph(src)
	if err != nil {
		return err
	}
	chartsToPublish = depGraph.WithDependents(chartsToPublish)

	log.Debug().Msgf("Identified %d charts to publish: %s", len(chartsToPublish), strings.Join(chartsToPublish, ","))
	depGraph.TopoSort(chartsToPublish)

	bucket := gcs.New(bucketName)
	chartUploader, err := repo.NewUploader(bucket, app)
	if err != nil {
		return err
	}

	if err := chartUploader.LockRepo(); err != nil {
		return err
	}
	defer func() {
		if err := chartUploader.UnlockRepo(); err != nil {
			log.Error().Msgf("Error unlocking Helm repo %s: %v", bucketName, err)
		}
	}()

	for _, chartName := range chartsToPublish {
		chart, err := src.GetChart(chartName)
		if err != nil {
			return err
		}

		if err := chart.GenerateDocs(); err != nil {
			return err
		}
		index, err := chartUploader.LoadIndex()
		if err != nil {
			return err
		}
		if err := chart.BumpChartVersion(index.LatestVersion(chartName)); err != nil {
			return err
		}
		if err := chart.BuildDependencies(); err != nil {
			return err
		}
		if err := chart.PackageChart(chartUploader.ChartStagingDir()); err != nil {
			return err
		}
	}

	if dryRun {
		log.Info().Msgf("Not uploading charts, this is a dry run")
		return nil
	}

	uploaded, err := chartUploader.UpdateRepo()
	if err != nil {
		return err
	}

	log.Info().Msgf("Uploaded %d charts", uploaded)

	return nil
}
