package embedded

import (
	_ "embed"
)

//go:embed scripts/zsh-duckhist.zsh
var ZshIntegrationScript string

// GetZshIntegrationScript returns the content of the zsh integration script
func GetZshIntegrationScript() string {
	return ZshIntegrationScript
}
