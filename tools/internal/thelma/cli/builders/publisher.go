package builders

import (
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/app"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/publish"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/charts/repo"
	"github.com/broadinstitute/terra-helmfile-images/tools/internal/thelma/utils/gcp/bucket"
	"github.com/rs/zerolog/log"
)

// PublisherBuilder is an interface for constructing a publisher for a given thelma app
type PublisherBuilder interface {
	// Publisher returns the constructed Publisher instance
	Publisher() publish.Publisher
	// Close Publisher and all associated resources
	Close() error
	// CloseWarn is like Close, but logs error instead of returning it (suitable for use in defer)
	CloseWarn()
}

type publisherBuilder struct {
	publisher publish.Publisher
	repo      repo.Repo
	bucket    bucket.Bucket
}

func Publisher(app *app.ThelmaApp, bucketName string, dryRun bool) (PublisherBuilder, error) {
	_bucket, err := bucket.NewBucket(bucketName)
	if err != nil {
		return nil, err
	}

	_repo := repo.NewRepo(_bucket)
	scratchDir, err := app.Paths.CreateScratchDir("publisher")
	if err != nil {
		return nil, err
	}

	publisher, err := publish.NewPublisher(_repo, app.ShellRunner, scratchDir, dryRun)
	if err != nil {
		return nil, err
	}

	return &publisherBuilder{
		publisher: publisher,
		repo:      _repo,
		bucket:    _bucket,
	}, nil
}

func (pb *publisherBuilder) Publisher() publish.Publisher {
	return pb.publisher
}

func (pb *publisherBuilder) CloseWarn() {
	if err := pb.Close(); err != nil {
		log.Warn().Msgf("error closing publisher: %v", err)
	}
}

func (pb *publisherBuilder) Close() error {
	if err := pb.publisher.Close(); err != nil {
		return err
	}

	return pb.bucket.Close()
}
