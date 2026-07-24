# curl | sh installer — design

**Status:** approved design, pre-implementation
**Date:** 2026-07-23
**Author:** Jacob Meacham (with Claude)

## Summary

A single POSIX-sh script at the repo root, fetched raw from GitHub and piped
to `sh`, that downloads the right release binary for the caller's platform,
verifies its checksum, and installs it onto the PATH:

```sh
curl -fsSL https://raw.githubusercontent.com/jacob-meacham/speclib/main/install.sh | sh
```

## Interface

- Default install dir: `/usr/local/bin` (already on PATH on macOS and Linux).
  Falls back to `sudo` for the copy only when the dir is not writable, with a
  message saying why.
- `SPECLIB_INSTALL_DIR` overrides the destination (created if missing; a
  warning is printed if it is not on PATH).
- `SPECLIB_VERSION` pins a version (with or without leading `v`); default is
  the latest release, resolved by following the
  `https://github.com/<repo>/releases/latest` redirect — not the GitHub API,
  which is rate-limited for unauthenticated callers (CI runners share IPs).

## Behavior

1. Require `curl`. Detect platform: `uname -s` → linux/darwin, `uname -m` →
   amd64/arm64. Anything else exits with a pointer to
   `go install github.com/jacob-meacham/speclib@latest`.
2. Resolve the version (env override or latest-redirect).
3. Download `speclib_<ver>_<os>_<arch>.tar.gz` and `checksums.txt` into a
   `mktemp -d` cleaned up via `trap ... EXIT`.
4. Verify the tarball's sha256 against `checksums.txt` (`sha256sum`, falling
   back to `shasum -a 256`; abort if neither exists). Checksum mismatch is a
   hard abort — nothing is installed.
5. Extract and `install -m 0755` into the target dir (sudo only if needed).
6. Print the result of running `<dir>/speclib --version`.

`set -eu` throughout; every failure path prints a one-line cause and
remediation to stderr; `curl -fsSL` makes HTTP errors fail loudly.

## Testing

- `shellcheck install.sh` runs in CI (zero findings).
- A CI `installer` job on `ubuntu-latest` and `macos-latest` executes the
  actual script twice against the real released artifacts: once resolving
  latest, once pinned via `SPECLIB_VERSION=0.1.0` asserting the exact
  `speclib version 0.1.0` output. `SPECLIB_INSTALL_DIR` points at
  `$RUNNER_TEMP` so no sudo path runs in CI.
- Pre-push local verification: both paths run in a sandbox against the real
  v0.1.0 release.

## README

The one-liner becomes the primary install method; `go install` and manual
download remain as alternatives.

## Out of scope

Homebrew tap, Windows support, self-update, and signature (vs checksum)
verification.
