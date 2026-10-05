#!/usr/bin/env bash
# Print reader feedback and failed searches for the mkbrr.com docs, from the Mintlify analytics API.
# Usage: scripts/docs-feedback.sh [days]   (default: 30)
# Needs MINTLIFY_API_KEY (admin key, "mint_...") and MINTLIFY_PROJECT_ID.
# Both are on https://app.mintlify.com/settings/organization/api-keys.
set -euo pipefail

: "${MINTLIFY_API_KEY:?set MINTLIFY_API_KEY to a Mintlify admin API key}"
: "${MINTLIFY_PROJECT_ID:?set MINTLIFY_PROJECT_ID}"

days="${1:-30}"
from="$(date -u -d "-${days} days" +%F)"
docs_dir="$(dirname "$0")/../documentation"

# fetch ENDPOINT KEY: print every item of array KEY, one JSON object per line, across all pages.
# ponytail: stops after 10 pages (1000 items). The API allows 100 requests per hour.
fetch() {
	local cursor="" n body
	for n in {1..10}; do
		body="$(curl -fsS -G "https://api.mintlify.com/v1/analytics/${MINTLIFY_PROJECT_ID}/$1" \
			-H "Authorization: Bearer ${MINTLIFY_API_KEY}" \
			--data-urlencode "dateFrom=${from}" --data-urlencode "limit=100" \
			${cursor:+--data-urlencode "cursor=${cursor}"})"
		jq -c ".$2[]" <<<"$body"
		cursor="$(jq -r '.nextCursor // empty' <<<"$body")"
		[[ -n "$cursor" ]] || return 0
	done
	echo "warning: stopped after ${n} pages of $1" >&2
}

echo "## Feedback since ${from}, by page"
pages="$(cd "$docs_dir" && find . -name '*.mdx' | sed 's|^\./||; s|\.mdx$||' | jq -Rs 'split("\n")')"
fetch feedback feedback | jq -rs --argjson pages "$pages" '
	map(.path |= (sub("^[a-z]+://[^/]+"; "") | ltrimstr("/") | if . == "" then "introduction" end))
	| group_by(.path)
	| map({path: .[0].path,
	       down: map(select(.helpful == false)) | length,
	       up: map(select(.helpful == true)) | length,
	       comments: map(.comment // empty | select(. != ""))})
	| sort_by(-.down)[]
	| "- /\(.path): \(.down) down, \(.up) up, "
	  + (if .path | IN($pages[]) then "documentation/\(.path).mdx" else "(no file)" end),
	  (.comments[] | "    > " + gsub("\n"; " "))'

echo
echo "## Searches since ${from} where no reader clicked a result"
fetch searches searches | jq -rs '
	map(select(.ctr == 0)) | sort_by(-.hits)[]
	| "- \(.searchQuery) (\(.hits) searches)"'
