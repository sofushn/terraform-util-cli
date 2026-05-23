package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/sofushn/terraform-util-cli/internal/address"
	"github.com/sofushn/terraform-util-cli/internal/app"
)

func printDocList(w io.Writer, items []app.DocItem, details bool) {
	if details && len(items) > 0 {
		printProviderMetadata(w, items[0].Provider, providerDocsWebsiteURL(items[0].Provider))
		fmt.Fprintln(w)
	}

	for _, item := range items {
		fmt.Fprintf(w, "%s/%s\n", item.Kind, item.Name)
	}
}

func printDocPage(w io.Writer, page app.DocPage, details bool) {
	if details {
		printProviderMetadata(w, page.Provider, page.Website)
		fmt.Fprintf(w, "Doc: %s/%s\n", page.Kind, page.Name)
		if page.Source != "" {
			fmt.Fprintf(w, "Source: %s\n", page.Source)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, page.Content)
}

func printModuleDocPage(w io.Writer, page app.ModuleDocPage, details bool) {
	if details {
		fmt.Fprintln(w, "Type: module")
		printModuleMetadata(w, page.Module, page.Website)
		if page.Source != "" {
			fmt.Fprintf(w, "Source: %s\n", page.Source)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, page.Content)
}

func printProviderVersions(w io.Writer, versions []app.ProviderVersion, details bool) {
	if details && len(versions) > 0 {
		provider := versions[0].Provider
		fmt.Fprintf(w, "provider: %s\n", provider.Source)
		fmt.Fprintf(w, "website: %s\n\n", providerWebsiteURLWithoutVersion(provider))
		printVersionsRow(w, []int{len("version"), len("published")}, []string{"version", "published"})
		for _, version := range versions {
			printVersionsRow(w, []int{len("version"), len("published")}, []string{version.Version, publishedDate(version.PublishedAt)})
		}
		return
	}

	fmt.Fprintln(w, "version")
	for _, version := range versions {
		fmt.Fprintln(w, version.Version)
	}
}

func printModuleVersions(w io.Writer, versions []app.ModuleVersion, details bool) {
	if details && len(versions) > 0 {
		module := versions[0].Module
		fmt.Fprintln(w, "type: module")
		fmt.Fprintf(w, "module: %s\n", module.Source)
		fmt.Fprintf(w, "website: %s\n", moduleWebsiteURLWithoutVersion(module))
		if module.RepositoryURL != "" {
			fmt.Fprintf(w, "source: %s\n", module.RepositoryURL)
		}
		fmt.Fprintln(w)
		printVersionsRow(w, []int{len("version"), len("published")}, []string{"version", "published"})
		for _, version := range versions {
			printVersionsRow(w, []int{len("version"), len("published")}, []string{version.Version, publishedDate(version.PublishedAt)})
		}
		return
	}

	fmt.Fprintln(w, "version")
	for _, version := range versions {
		fmt.Fprintln(w, version.Version)
	}
}

func printVersionsRow(w io.Writer, widths []int, values []string) {
	for i := 0; i < len(values); i++ {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", widths[i], values[i])
	}
	fmt.Fprintln(w)
}

func publishedDate(publishedAt string) string {
	if len(publishedAt) >= len("2006-01-02") {
		return publishedAt[:len("2006-01-02")]
	}
	return publishedAt
}

func printProviderMetadata(w io.Writer, provider app.Provider, website string) {
	fmt.Fprintf(w, "Provider: %s\n", provider.Source)
	fmt.Fprintf(w, "Version: %s\n", provider.LatestVersion)
	if website == "" {
		website = providerWebsiteURL(provider)
	}
	if website != "" {
		fmt.Fprintf(w, "Website: %s\n", website)
	}
}

func printModuleMetadata(w io.Writer, module app.Module, website string) {
	fmt.Fprintf(w, "Module: %s\n", module.Source)
	fmt.Fprintf(w, "Version: %s\n", module.LatestVersion)
	if website == "" {
		website = moduleWebsiteURL(module)
	}
	if website != "" {
		fmt.Fprintf(w, "Website: %s\n", website)
	}
}

func providerWebsiteURL(provider app.Provider) string {
	parts := strings.Split(address.TrimRegistryHost(provider.Source), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}

	version := provider.LatestVersion
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}

	return fmt.Sprintf("https://registry.terraform.io/providers/%s/%s/%s", parts[0], parts[1], version)
}

func providerWebsiteURLWithoutVersion(provider app.Provider) string {
	parts := strings.Split(address.TrimRegistryHost(provider.Source), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}

	return fmt.Sprintf("https://registry.terraform.io/providers/%s/%s", parts[0], parts[1])
}

func providerDocsWebsiteURL(provider app.Provider) string {
	website := providerWebsiteURL(provider)
	if website == "" {
		return ""
	}
	return website + "/docs"
}

func moduleWebsiteURL(module app.Module) string {
	parts := strings.Split(address.TrimRegistryHost(module.Source), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}

	version := module.LatestVersion
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}

	return fmt.Sprintf("https://registry.terraform.io/modules/%s/%s/%s/%s", parts[0], parts[1], parts[2], version)
}

func moduleWebsiteURLWithoutVersion(module app.Module) string {
	parts := strings.Split(address.TrimRegistryHost(module.Source), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}

	return fmt.Sprintf("https://registry.terraform.io/modules/%s/%s/%s", parts[0], parts[1], parts[2])
}

func printProviderSearchResults(w io.Writer, providers []app.Provider, details bool) {
	printProviderSearchHeader(w, details)
	printProviderSearchRows(w, providers, details)
}

func printSearchHeader(w io.Writer, details bool, includeType bool) {
	values := []string{"source", "name", "version", "downloads", "tier", "verified"}
	if includeType {
		values = append([]string{"type"}, values...)
	}
	printGenericSearchRow(w, searchColumnWidths(includeType), details, includeType, values)
}

func printSearchRows(w io.Writer, results []app.SearchResult, details bool, includeType bool) {
	widths := searchColumnWidths(includeType)
	for _, result := range results {
		verified := ""
		if result.Verified {
			verified = "true"
		}

		downloads := ""
		if details {
			downloads = fmt.Sprintf("%d", result.Downloads)
		}

		values := []string{
			result.Source,
			result.Name,
			result.LatestVersion,
			downloads,
			result.Tier,
			verified,
		}
		if includeType {
			values = append([]string{string(result.Type)}, values...)
		}
		printGenericSearchRow(w, widths, details, includeType, values)
	}
}

func printProviderSearchHeader(w io.Writer, details bool) {
	printSearchHeader(w, details, false)
}

func printProviderSearchRows(w io.Writer, providers []app.Provider, details bool) {
	printSearchRows(w, appProviderSearchResults(providers), details, false)
}

func appProviderSearchResults(providers []app.Provider) []app.SearchResult {
	results := make([]app.SearchResult, 0, len(providers))
	for _, provider := range providers {
		results = append(results, app.SearchResult{
			Type:          app.SearchTypeProvider,
			Source:        provider.Namespace + "/" + provider.Name,
			Name:          provider.DisplayName,
			LatestVersion: provider.LatestVersion,
			Downloads:     provider.Downloads,
			Verified:      provider.Verified,
			Tier:          provider.Tier,
		})
	}
	return results
}

func searchColumnWidths(includeType bool) []int {
	widths := []int{36, 10, 42, 12, 10, len("verified")}
	if includeType {
		return append([]int{len("provider")}, widths...)
	}
	return widths
}

func printGenericSearchRow(w io.Writer, widths []int, details bool, includeType bool, values []string) {
	row := values
	rowWidths := widths
	if !details {
		if includeType {
			row = []string{values[0], values[1], values[2], values[3], values[6]}
			rowWidths = []int{widths[0], widths[1], widths[2], widths[3], widths[6]}
		} else {
			row = []string{values[0], values[1], values[2], values[5]}
			rowWidths = []int{widths[0], widths[1], widths[2], widths[5]}
		}
	}

	for i := 0; i < len(row); i++ {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", rowWidths[i], truncateColumn(row[i], rowWidths[i]))
	}
	fmt.Fprintln(w)
}

func truncateColumn(value string, width int) string {
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:0]
	}

	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}
