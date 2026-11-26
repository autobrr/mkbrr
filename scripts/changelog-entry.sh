#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 [--write] <tag>" >&2
  echo "Prints the documentation/changelog.mdx entry for <tag>. --write adds it at the top of the changelog." >&2
  exit 2
}

write=
case "${1:-}" in
  --write) write=1; shift ;;
esac
[ $# -eq 1 ] || usage
tag=$1

root=$(git rev-parse --show-toplevel)
changelog="$root/documentation/changelog.mdx"

git fetch --quiet --tags origin
git rev-parse -q --verify "refs/tags/$tag" >/dev/null || { echo "origin has no tag $tag" >&2; exit 1; }
case "$tag" in
  *-*) echo "$tag is a prerelease and gets no changelog entry" >&2; exit 1 ;;
esac
if [ -n "$write" ] && grep -q "<Update label=\"$tag\"" "$changelog"; then
  echo "$changelog already has $tag" >&2
  exit 1
fi

previous=$(git tag --list 'v*' --sort=-v:refname | grep -v -e - | awk -v t="$tag" 'found { print; exit } $0 == t { found = 1 }')
[ -n "$previous" ] || { echo "found no release tag before $tag" >&2; exit 1; }
date=$(TZ=UTC git for-each-ref --format='%(creatordate:format-local:%Y-%m-%d)' "refs/tags/$tag")
repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)

commits=$(gh api --paginate "repos/$repo/compare/$previous...$tag" \
  --jq '.commits[] | select(.parents | length == 1) | [.sha, (.author.login // ""), (.commit.message | split("\n")[0])] | @tsv')
[ -n "$commits" ] || { echo "found no commits between $previous and $tag" >&2; exit 1; }

unsafe=$(printf '%s\n' "$commits" | awk -F'\t' '$3 ~ /[<{}]/ { print substr($1, 1, 7) ": " $3 }')
if [ -n "$unsafe" ]; then
  echo "these subjects have <, { or }, which MDX reads as code; write their lines by hand:" >&2
  echo "$unsafe" >&2
  exit 1
fi

entry=$(printf '%s\n' "$commits" | awk -F'\t' -v url="https://github.com/$repo" -v tag="$tag" -v previous="$previous" -v date="$date" '
  {
    sha = $1; login = $2; subject = $3
    section = "other"
    if (subject ~ /^[A-Za-z]+(\([^)]*\))?!:/) section = "breaking"
    else if (subject ~ /^feat(\([^)]*\))?:/) section = "feat"
    else if (subject ~ /^fix(\([^)]*\))?:/) section = "fix"
    sub(/^[A-Za-z]+(\([^)]*\))?!?: */, "", subject)
    subject = toupper(substr(subject, 1, 1)) substr(subject, 2)
    if (match(subject, / \(#[0-9]+\)$/)) {
      pr = substr(subject, RSTART + 3, RLENGTH - 4)
      subject = substr(subject, 1, RSTART - 1) " ([#" pr "](" url "/pull/" pr "))"
    }
    line = "- [" substr(sha, 1, 7) "](" url "/commit/" sha "): " subject
    if (login != "") {
      line = line " (@" login ")"
      if (!(login in seen)) { seen[login] = 1; contributors[++n] = login }
    }
    lines[section] = lines[section] line "\n"
  }
  END {
    print "<Update label=\"" tag "\" description=\"" date "\">"
    split("breaking feat fix other", order, " ")
    titles["breaking"] = "Breaking Changes"; titles["feat"] = "New Features"
    titles["fix"] = "Bug fixes"; titles["other"] = "Other work"
    for (i = 1; i <= 4; i++) {
      s = order[i]
      if (lines[s] != "") printf "### %s\n%s\n", titles[s], lines[s]
    }
    print "**Full Changelog**: [" previous "..." tag "](" url "/compare/" previous "..." tag ")"
    print ""
    print "### Contributors"
    for (i = 1; i <= n; i++) print "- @" contributors[i]
    print "</Update>"
  }')

if [ -z "$write" ]; then
  printf '%s\n' "$entry"
  exit 0
fi

tmp=$(mktemp "$changelog.XXXXXX")
ENTRY=$entry awk '!done && /^<Update / { print ENVIRON["ENTRY"]; done = 1 } { print } END { if (!done) exit 1 }' "$changelog" > "$tmp" \
  || { rm -f "$tmp"; echo "found no <Update> entry in $changelog" >&2; exit 1; }
cat "$tmp" > "$changelog"
rm -f "$tmp"
echo "added $tag to $changelog" >&2
