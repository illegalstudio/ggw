// Package skills provides the AI agent skill bundled with ggw.
package skills

import "embed"

// bundled contains the skill exactly as it is installed for the user.
//
//go:embed ggw
var bundled embed.FS
