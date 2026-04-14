//go:build !(edition_lite || edition_license)

package main

// Edition: full — imports all optional Apps.
// When no build tag is specified, this file is compiled by default,
// giving you the full-featured build.
import (
	_ "metis/internal/app/ai"
	_ "metis/internal/app/apm"
	_ "metis/internal/app/license"
	_ "metis/internal/app/node"
	_ "metis/internal/app/observe"
	_ "metis/internal/app/org"
)
