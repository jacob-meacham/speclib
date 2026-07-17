package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

type File struct {
	Path string
	Data []byte
}

type Spec struct {
	Prompt   []byte
	SpecDoc  []byte
	Fixtures []File
}

func (s *Spec) HasFixtures() bool { return len(s.Fixtures) > 0 }

func (s *Spec) Hash() string {
	h := sha256.New()
	writeField(h, "prompt", s.Prompt)
	writeField(h, "spec", s.SpecDoc)

	fixtures := append([]File(nil), s.Fixtures...)
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].Path < fixtures[j].Path })
	for _, f := range fixtures {
		writeField(h, "fixture:"+f.Path, f.Data)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// writeField length-prefixes each field so concatenation is unambiguous.
func writeField(h interface{ Write([]byte) (int, error) }, name string, data []byte) {
	var lenbuf [8]byte
	putUint64(lenbuf[:], uint64(len(name)))
	h.Write(lenbuf[:])
	h.Write([]byte(name))
	putUint64(lenbuf[:], uint64(len(data)))
	h.Write(lenbuf[:])
	h.Write(data)
}

func putUint64(b []byte, v uint64) {
	for i := 0; i < 8; i++ {
		b[i] = byte(v >> (8 * i))
	}
}
