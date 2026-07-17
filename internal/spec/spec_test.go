package spec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashStableAndOrderIndependent(t *testing.T) {
	a := &Spec{
		Prompt:  []byte("p"),
		SpecDoc: []byte("s"),
		Fixtures: []File{
			{Path: "b.json", Data: []byte("2")},
			{Path: "a.json", Data: []byte("1")},
		},
	}
	b := &Spec{
		Prompt:  []byte("p"),
		SpecDoc: []byte("s"),
		Fixtures: []File{
			{Path: "a.json", Data: []byte("1")},
			{Path: "b.json", Data: []byte("2")},
		},
	}
	require.Equal(t, a.Hash(), b.Hash())
	require.True(t, strings.HasPrefix(a.Hash(), "sha256:"))
}

func TestHashChangesWithContent(t *testing.T) {
	a := &Spec{Prompt: []byte("p"), SpecDoc: []byte("s")}
	b := &Spec{Prompt: []byte("p"), SpecDoc: []byte("s2")}
	require.NotEqual(t, a.Hash(), b.Hash())
}

func TestHasFixtures(t *testing.T) {
	require.False(t, (&Spec{}).HasFixtures())
	require.True(t, (&Spec{Fixtures: []File{{Path: "a"}}}).HasFixtures())
}
