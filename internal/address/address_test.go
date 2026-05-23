package address

import "testing"

func TestParseProvider(t *testing.T) {
	tests := []struct {
		input     string
		namespace string
		name      string
		localName string
		source    string
	}{
		{input: "aws", namespace: "hashicorp", name: "aws", localName: "aws", source: "hashicorp/aws"},
		{input: "hashicorp/aws", namespace: "hashicorp", name: "aws", localName: "aws", source: "hashicorp/aws"},
		{input: "registry.terraform.io/hashicorp/aws", namespace: "hashicorp", name: "aws", localName: "aws", source: "hashicorp/aws"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			provider, err := ParseProvider(tt.input)
			if err != nil {
				t.Fatalf("parse provider: %v", err)
			}
			if provider.Namespace != tt.namespace || provider.Name != tt.name || provider.LocalName != tt.localName || provider.Source != tt.source {
				t.Fatalf("unexpected provider: %#v", provider)
			}
		})
	}
}

func TestParseModule(t *testing.T) {
	tests := []string{
		"terraform-aws-modules/vpc/aws",
		"registry.terraform.io/terraform-aws-modules/vpc/aws",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			module, err := ParseModule(input)
			if err != nil {
				t.Fatalf("parse module: %v", err)
			}
			if module.Namespace != "terraform-aws-modules" || module.Name != "vpc" || module.Provider != "aws" || module.Source != "terraform-aws-modules/vpc/aws" {
				t.Fatalf("unexpected module: %#v", module)
			}
		})
	}
}

func TestParseRejectsInvalidShapes(t *testing.T) {
	if _, err := ParseProvider("namespace/name/provider"); err == nil {
		t.Fatalf("expected provider parse to reject module-shaped input")
	}
	if _, err := ParseModule("hashicorp/aws"); err == nil {
		t.Fatalf("expected module parse to reject provider-shaped input")
	}
}
