# Release version v-prefix normalization

**Date:** 2026-08-08
**Status:** Approved

## Problem

`speclib release v1.4.0` creates the tag `vv1.4.0`. The version argument is
validated with `semver.NewVersion`, which tolerates a leading `v`, and the tag
is then built as `"v" + version` — so a `v`-prefixed input double-prefixes the
tag.

## Design

Normalize the argument instead of rejecting it:

- In `cmd/release.go`, trim a leading lowercase `v` from the argument before
  semver validation: `version := strings.TrimPrefix(args[0], "v")`.
- Everything downstream is unchanged: validation, `tag := "v" + version`, the
  tag-exists check, and the `Released <name> <version> as tag <tag>` message.
  Both `release 1.4.0` and `release v1.4.0` therefore create tag `v1.4.0`.
- An uppercase `V1.4.0` is not trimmed and still fails semver validation with
  the existing `invalid version` error.
- Update the command help text to note the version may optionally be
  `v`-prefixed.

Out of scope: auto-bump keywords (`release patch|minor|major`) — considered
and deliberately deferred.

## Testing

- New `TestReleaseNormalizesVPrefix`: run `release v0.1.0` in a committed
  scaffolded library; assert tag `v0.1.0` exists and no `vv*` tag was created.
- Existing release tests remain unchanged and green.
