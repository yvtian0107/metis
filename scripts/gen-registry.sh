#!/bin/bash
# gen-registry.sh — Generate web/src/apps/_bootstrap.ts for filtered builds.
# Also patches tsconfig.app.json to exclude non-selected app directories,
# so tsc only checks code relevant to the edition.
#
# Usage: APPS=system,ai ./scripts/gen-registry.sh
#
# If APPS is not set, restores the full version (all app modules + tsconfig).

set -euo pipefail

BOOTSTRAP="web/src/apps/_bootstrap.ts"
TSCONFIG="web/tsconfig.app.json"

# All known frontend app directories (excluding system which is kernel)
ALL_APPS=(ai apm itsm license node observe org)

# No APPS specified → restore full version (deterministic, no git checkout)
if [ -z "${APPS:-}" ]; then
  {
    echo '// App module side-effect imports.'
    echo '// gen-registry.sh replaces this file for filtered builds (APPS=...).'
    for app in "${ALL_APPS[@]}"; do
      echo "import \"./${app}/module\""
    done
  } > "$BOOTSTRAP"
  git checkout -- "$TSCONFIG" 2>/dev/null || true
  echo "[gen-registry] restored full _bootstrap.ts (${ALL_APPS[*]})"
  exit 0
fi

# --- Filtered build ---

# 1. Generate filtered _bootstrap.ts
cat > "$BOOTSTRAP" << 'HEADER'
// Auto-generated — do not edit. Run without APPS to restore full version.
HEADER

SELECTED=()
for app in $(echo "$APPS" | tr ',' '\n'); do
  [ "$app" = "system" ] && continue
  echo "import './${app}/module'" >> "$BOOTSTRAP"
  SELECTED+=("$app")
done

# 2. Compute excluded app directories
EXCLUDES=()
for app in "${ALL_APPS[@]}"; do
  skip=false
  for sel in "${SELECTED[@]}"; do
    [ "$app" = "$sel" ] && skip=true && break
  done
  [ "$skip" = false ] && EXCLUDES+=("$app")
done

# 3. Write tsconfig.app.json with exclude list
EXCLUDE_ENTRIES=""
for i in "${!EXCLUDES[@]}"; do
  [ "$i" -gt 0 ] && EXCLUDE_ENTRIES+=", "
  EXCLUDE_ENTRIES+="\"src/apps/${EXCLUDES[$i]}\""
done

cat > "$TSCONFIG" << EOF
{
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "target": "es2023",
    "lib": ["ES2023", "DOM", "DOM.Iterable"],
    "module": "esnext",
    "types": ["vite/client"],
    "skipLibCheck": true,

    /* Bundler mode */
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "verbatimModuleSyntax": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",

    /* Linting */
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "erasableSyntaxOnly": true,
    "noFallthroughCasesInSwitch": true,

    /* Path aliases */
    "ignoreDeprecations": "6.0",
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src"],
  "exclude": [${EXCLUDE_ENTRIES}]
}
EOF

echo "[gen-registry] generated _bootstrap.ts with APPS=$APPS (excluded: ${EXCLUDES[*]:-none})"
