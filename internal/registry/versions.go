package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

type ProviderVersion struct {
	Version     string
	PublishedAt string
}

func (c Client) ListProviderVersions(ctx context.Context, provider Provider) ([]ProviderVersion, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/v2/providers/%s/%s", c.BaseURL, url.PathEscape(provider.Namespace), url.PathEscape(provider.Name)))
	if err != nil {
		return nil, err
	}
	params := endpoint.Query()
	params.Set("include", "provider-versions")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	var response v2ProviderWithVersionsResponse
	if err := c.doJSON(req, &response); err != nil {
		return nil, err
	}

	versions := make([]ProviderVersion, 0, len(response.Included))
	for _, item := range response.Included {
		if item.Type != "provider-versions" {
			continue
		}
		versions = append(versions, ProviderVersion{
			Version:     item.Attributes.Version,
			PublishedAt: item.Attributes.PublishedAt,
		})
	}

	sortProviderVersions(versions)
	return versions, nil
}

func sortProviderVersions(versions []ProviderVersion) {
	sort.SliceStable(versions, func(i, j int) bool {
		left := semver.Canonical("v" + versions[i].Version)
		right := semver.Canonical("v" + versions[j].Version)
		if left != "" && right != "" && left != right {
			return semver.Compare(left, right) > 0
		}
		if versions[i].PublishedAt != versions[j].PublishedAt {
			return versions[i].PublishedAt > versions[j].PublishedAt
		}
		return strings.Compare(versions[i].Version, versions[j].Version) > 0
	})
}
