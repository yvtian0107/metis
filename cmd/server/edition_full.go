//go:build !(edition_lite || edition_license)

package main

// Edition: full — imports all optional Apps.
// When no build tag is specified, this file is compiled by default,
// giving you the full-featured build.
//
// Import order determines App seed execution order (via init() → app.Register):
//   Tier 0 (no cross-app deps): org, node, apm, observe, license
//   Tier 1 (depends on Tier 0):  ai (optional dep on Org)
//   Tier 2 (depends on Tier 1):  itsm (queries ai_agents table)
import (
	_ "metis/internal/app/org"
	_ "metis/internal/app/node"
	_ "metis/internal/app/apm"
	_ "metis/internal/app/observe"
	_ "metis/internal/app/license"
	_ "metis/internal/app/ai"
	_ "metis/internal/app/itsm"
)
