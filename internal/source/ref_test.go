package source

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	require.True(t, Parse("../specs/x").IsLocal)
	require.True(t, Parse("/abs/x").IsLocal)
	require.True(t, Parse("file:///abs/x").IsLocal)
	require.Equal(t, "/abs/x", Parse("file:///abs/x").Location)

	g := Parse("https://github.com/o/r")
	require.False(t, g.IsLocal)
	require.Equal(t, "https://github.com/o/r", g.Location)
}

func vers(t *testing.T, ss ...string) []*semver.Version {
	out := make([]*semver.Version, len(ss))
	for i, s := range ss {
		v, err := semver.NewVersion(s)
		require.NoError(t, err)
		out[i] = v
	}
	return out
}

func TestPickVersion(t *testing.T) {
	vs := vers(t, "1.0.0", "1.4.0", "1.5.0", "2.0.0")

	got, err := PickVersion("^1.4", vs)
	require.NoError(t, err)
	require.Equal(t, "1.5.0", got.String())

	got, err = PickVersion("*", vs)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", got.String())

	_, err = PickVersion("^3", vs)
	require.Error(t, err)

	_, err = PickVersion("*", nil)
	require.Error(t, err)
}
