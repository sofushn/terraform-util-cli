package cli

import "terraform-util/internal/address"

func isModuleAddress(input string) bool {
	return address.IsModule(input)
}
