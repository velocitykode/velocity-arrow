package store

import (
	"encoding/binary"
	"math"
	"os"
	"strings"
)

// encodeEmbedding packs a float32 vector into little-endian bytes (4 bytes per
// component). A nil or empty vector encodes to an empty (non-nil) byte slice so
// the BLOB column round-trips as "no embedding".
func encodeEmbedding(vec []float32) []byte {
	if len(vec) == 0 {
		return []byte{}
	}
	out := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}

// decodeEmbedding unpacks little-endian float32 bytes back into a vector. A
// length that is not a multiple of 4 yields a nil vector (treated as "no
// embedding") rather than a partial decode.
func decodeEmbedding(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}

// joinTags renders tags as a comma-joined string for storage.
func joinTags(tags []string) string {
	return strings.Join(tags, ",")
}

// splitTags reverses joinTags. An empty stored string yields a nil slice.
func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// truncate removes any existing file at path so Create starts from a clean
// database. A missing file is not an error.
func truncate(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// cosine returns the cosine similarity of two equal-length vectors. It returns 0
// when either vector is empty, lengths differ, or either magnitude is zero, so a
// degenerate vector never poisons ranking.
func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
