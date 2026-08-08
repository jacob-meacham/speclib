# Release v-prefix Normalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `speclib release v1.4.0` creates tag `v1.4.0` (not `vv1.4.0`) by trimming a leading `v` from the version argument.

**Architecture:** Single change in the release command: normalize the argument with `strings.TrimPrefix` before semver validation, so tag construction (`"v" + version`) and messaging work unchanged for both `1.4.0` and `v1.4.0` inputs.

**Tech Stack:** Go, cobra, Masterminds/semver, testify.

## Global Constraints

- Spec: `docs/specs/2026-08-08-release-v-prefix-design.md`
- Only a leading lowercase `v` is trimmed; `V1.4.0` must still fail semver validation with the existing `invalid version` error.
- Out of scope: auto-bump keywords (`release patch|minor|major`).

---

### Task 1: Trim leading "v" in the release command

**Files:**
- Modify: `cmd/release.go:14-22` (help text + version normalization)
- Test: `cmd/release_test.go`

**Interfaces:**
- Consumes: existing test helpers `makeCommittedLib(t, dir)` and `runCmd(t, dir, args...)` from `cmd/release_test.go` / `cmd/root_test.go`.
- Produces: no new exported symbols; `release` accepts `v`-prefixed versions.

- [ ] **Step 1: Write the failing test**

Append to `cmd/release_test.go`:

```go
func TestReleaseNormalizesVPrefix(t *testing.T) {
	dir := t.TempDir()
	libDir := makeCommittedLib(t, dir)

	out, err := runCmd(t, libDir, "release", "v0.1.0")
	require.NoError(t, err)
	require.Contains(t, out, "tag v0.1.0")

	tagOut, err := exec.Command("git", "-C", libDir, "tag", "--list").CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(tagOut), "v0.1.0")
	require.NotContains(t, string(tagOut), "vv0.1.0")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd -run TestReleaseNormalizesVPrefix -v`
Expected: FAIL — the buggy code prints `... as tag vv0.1.0`, so `require.Contains(t, out, "tag v0.1.0")` fails (and the created tag is `vv0.1.0`).

- [ ] **Step 3: Write minimal implementation**

In `cmd/release.go`, change the argument handling in `RunE` and the help text:

```go
return &cobra.Command{
	Use:   "release <version>",
	Short: "Lint, then tag a release of the spec-library in the current directory",
	Long: `Lint the spec-library in the current directory, then create git tag v<version>.

The version may optionally include a leading "v": both 1.4.0 and v1.4.0
create the tag v1.4.0.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		version := strings.TrimPrefix(args[0], "v")
		if _, err := semver.NewVersion(version); err != nil {
			return fmt.Errorf("invalid version %q: not semver: %w", version, err)
		}
		tag := "v" + version
		// ... rest unchanged
```

(`strings` is already imported in `cmd/release.go`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd -run TestReleaseNormalizesVPrefix -v`
Expected: PASS

- [ ] **Step 5: Run the full suite**

Run: `go test ./...`
Expected: all packages PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/release.go cmd/release_test.go
git commit -m "fix: accept v-prefixed versions in speclib release"
```
