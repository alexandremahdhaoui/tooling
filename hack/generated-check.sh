#!/bin/sh
set -eu

# Generated code is committed here and read by consumers straight from the
# repo, so a copy that no longer matches its source is a second source of
# truth. Nothing checked it: the release gates on a -dirty revision, but that
# revision is measured BEFORE any stage runs, so whatever a build regenerates
# is invisible to it. This is that check, at build time, with per-file blame.
#
# Build output the repo does not ignore lands here too. It is the same defect
# seen from the other side: an unignored artifact dirties every revision, so
# the pipeline mints a new one every run and the loop never settles.
#
# A plain build is enough, and no flag or trick is needed, because a
# generator always runs. Reusing output is a capability an engine declares
# and no generator declares it: what a generator writes depends on the
# generator, and the record holds only what it read, so a changed template
# would otherwise leave every engine stale with every digest still matching.
# This gate passed on exactly that once, which is how the rule was found.
#
# The comparison is before-and-after and never the whole tree. A gate that
# failed on any dirty file would fail on the work in progress of whoever ran
# it, which teaches people to ignore it.

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

git status --porcelain | sort >"$work/before"

forge build >/dev/null

git status --porcelain | sort >"$work/after"

comm -13 "$work/before" "$work/after" >"$work/written"

if [ -s "$work/written" ]; then
    echo "the build changed files that were committed or ignored as they were:" >&2
    cat "$work/written" >&2
    echo >&2
    echo "either the generated code does not match its source - run" >&2
    echo "'forge build' and commit the result - or the build writes" >&2
    echo "output this repo does not gitignore." >&2
    exit 1
fi

echo "the build changed nothing: generated code matches its source and its output is ignored"
