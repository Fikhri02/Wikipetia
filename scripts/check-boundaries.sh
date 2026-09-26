#!/usr/bin/env bash
# Enforces the architecture rules the spec states. These are not style
# preferences: each one, if violated, undoes a decision the design depends on.
set -euo pipefail

cd "$(dirname "$0")/.."
failed=0

fail() {
    echo "BOUNDARY VIOLATION: $1" >&2
    failed=1
}

# Rule 1: no learning package may import engagement.
# This is what keeps pets opt-in and deletable as a directory.
for pkg in course enrolment progress srs events; do
    dir="api/internal/$pkg"
    [ -d "$dir" ] || continue
    if grep -rn --include='*.go' 'wikipetia/internal/engagement' "$dir" 2>/dev/null; then
        fail "$pkg imports internal/engagement"
    fi
done

# Rule 2: no domain package may import net/http.
# Handlers own HTTP so the domain stays reusable by a CLI, a job, or another
# client. internal/platform/auth is the documented exception: middleware is
# HTTP by definition.
while IFS= read -r file; do
    case "$file" in
        api/internal/platform/auth/*) continue ;;
    esac
    if grep -qE '^\s*"net/http"$|^\s*_ "net/http"$' "$file"; then
        fail "$file imports net/http"
    fi
done < <(find api/internal -name '*.go' ! -name '*_test.go')

# Rule 3: no SQL outside the generated code and the query files.
# All SQL lives in db/queries as sqlc sources.
#
# The match is deliberately narrow: UPPERCASE keywords only, at a word
# boundary, on a line that also contains a quote or backtick. A case-insensitive
# match flags English prose -- "multi-select and set replacement" in a comment
# contains both "select" and "set".
while IFS= read -r file; do
    case "$file" in
        api/internal/platform/sqlcgen/*) continue ;;
    esac
    if grep -qE '["`].*\b(SELECT|INSERT INTO|UPDATE|DELETE FROM)\b' "$file"; then
        fail "$file appears to contain inline SQL"
    fi
done < <(find api/internal api/cmd -name '*.go' ! -name '*_test.go')

if [ "$failed" -ne 0 ]; then
    echo "" >&2
    echo "These rules come from the design spec. If a rule is genuinely wrong," >&2
    echo "change the spec and this script together — do not silence it locally." >&2
    exit 1
fi

echo "boundaries OK"
