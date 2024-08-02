package plan

import (
	"context"
	"strings"

	"github.com/google/go-github/v63/github"
)

func kubernetesReleaseVersions(ctx context.Context) ([]string, error) {
	ghc := github.NewClient(nil)

	releaseNames := make([]string, 0)

	for i := 1; i <= 10; i++ {
		releases, _, err := ghc.Repositories.ListReleases(ctx, "kubernetes", "kubernetes", &github.ListOptions{
			PerPage: 100,
			Page:    i,
		})
		if err != nil {
			return nil, err
		}

		if len(releases) == 0 {
			return releaseNames, nil
		}

		for _, release := range releases {
			if !strings.Contains(release.GetTagName(), "-") {
				// 筛选出正式版本
				releaseNames = append(releaseNames, *release.TagName)
				if len(releaseNames) >= 200 {
					return releaseNames, nil
				}
			}
		}
	}
	return releaseNames, nil
}
