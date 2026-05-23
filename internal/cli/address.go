package cli

import "github.com/sofushn/terraform-util-cli/internal/address"

func isModuleAddress(input string) bool {
	return address.IsModule(input)
}
