package address

import (
	"fmt"
	"strings"
)

const RegistryHost = "registry.terraform.io"

type Provider struct {
	Namespace string
	Name      string
	LocalName string
	Source    string
}

type Module struct {
	Namespace string
	Name      string
	Provider  string
	Source    string
}

func TrimRegistryHost(input string) string {
	return strings.TrimPrefix(strings.TrimSpace(input), RegistryHost+"/")
}

func ParseProvider(input string) (Provider, error) {
	source := TrimRegistryHost(input)
	if source == "" {
		return Provider{}, fmt.Errorf("provider is required")
	}

	parts := strings.Split(source, "/")
	for _, part := range parts {
		if part == "" {
			return Provider{}, fmt.Errorf("invalid provider %q", input)
		}
	}

	switch len(parts) {
	case 1:
		return Provider{
			Namespace: "hashicorp",
			Name:      parts[0],
			LocalName: parts[0],
			Source:    "hashicorp/" + parts[0],
		}, nil
	case 2:
		return Provider{
			Namespace: parts[0],
			Name:      parts[1],
			LocalName: parts[1],
			Source:    parts[0] + "/" + parts[1],
		}, nil
	default:
		return Provider{}, fmt.Errorf("invalid provider %q", input)
	}
}

func ParseModule(input string) (Module, error) {
	source := TrimRegistryHost(input)
	parts := strings.Split(source, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Module{}, fmt.Errorf("module source must be namespace/name/provider")
	}

	return Module{
		Namespace: parts[0],
		Name:      parts[1],
		Provider:  parts[2],
		Source:    parts[0] + "/" + parts[1] + "/" + parts[2],
	}, nil
}

func IsModule(input string) bool {
	_, err := ParseModule(input)
	return err == nil
}

func ProviderSource(namespace string, name string) string {
	return RegistryHost + "/" + namespace + "/" + name
}

func ModuleSource(namespace string, name string, provider string) string {
	return RegistryHost + "/" + namespace + "/" + name + "/" + provider
}
