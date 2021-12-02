package views

import (
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestSortChartReleases(t *testing.T) {
	sorted := []ChartRelease{
		{
			Name:    "mongodb",
			Version: "0.1.2",
			Repo:    "bitnami",
		},
		{
			Name:    "agora",
			Version: "1.2.2",
			Repo:    "terra-helm",
		},
		{
			Name:    "agora",
			Version: "1.2.3",
			Repo:    "terra-helm",
		},
		{
			Name:    "buffer",
			Version: "0.0.1",
			Repo:    "terra-helm",
		},
		{
			Name:    "cromwell",
			Version: "0.4.0",
			Repo:    "terra-helm",
		},
		{
			Name:    "cromwell",
			Version: "0.20.0",
			Repo:    "terra-helm",
		},
	}

	shuffled := make([]ChartRelease, len(sorted))
	copy(shuffled, sorted)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	SortChartReleases(shuffled)
	assert.Equal(t, sorted, shuffled)
}
