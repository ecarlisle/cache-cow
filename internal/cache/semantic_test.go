package cache

import (
	"math"
	"testing"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}

	got := CosineSimilarity(a, b)
	if math.Abs(float64(got-1.0)) > 0.001 {
		t.Errorf("identical vectors: got %f, want 1.0", got)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{-1, 0}

	got := CosineSimilarity(a, b)
	if math.Abs(float64(got+1.0)) > 0.001 {
		t.Errorf("opposite vectors: got %f, want -1.0", got)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}

	got := CosineSimilarity(a, b)
	if math.Abs(float64(got)) > 0.001 {
		t.Errorf("orthogonal vectors: got %f, want 0.0", got)
	}
}

func TestCosineSimilarityPartial(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0.5, 0.5}

	got := CosineSimilarity(a, b)
	expected := float32(math.Sqrt(2) / 2)
	if math.Abs(float64(got-expected)) > 0.01 {
		t.Errorf("partial: got %f, want %f", got, expected)
	}
}

func TestCosineSimilarityEmpty(t *testing.T) {
	if got := CosineSimilarity(nil, nil); got != 0 {
		t.Errorf("nil: got %f, want 0", got)
	}
	if got := CosineSimilarity([]float32{}, []float32{}); got != 0 {
		t.Errorf("empty: got %f, want 0", got)
	}
}

func TestCosineSimilarityMismatchedLength(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0}

	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("mismatched: got %f, want 0", got)
	}
}

func TestCosineSimilarityZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 0, 0}

	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("zero vector: got %f, want 0", got)
	}
}

func TestDecodeVector(t *testing.T) {
	original := []float32{1.5, -2.5, 3.0}
	encoded := make([]byte, len(original)*4)
	for i, v := range original {
		bits := math.Float32bits(v)
		encoded[i*4] = byte(bits)
		encoded[i*4+1] = byte(bits >> 8)
		encoded[i*4+2] = byte(bits >> 16)
		encoded[i*4+3] = byte(bits >> 24)
	}

	decoded, err := decodeVector(encoded)
	if err != nil {
		t.Fatalf("decodeVector: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("expected 3 values, got %d", len(decoded))
	}
	if math.Abs(float64(decoded[0]-1.5)) > 0.001 {
		t.Errorf("decoded[0]: got %f, want 1.5", decoded[0])
	}
	if math.Abs(float64(decoded[1]+2.5)) > 0.001 {
		t.Errorf("decoded[1]: got %f, want -2.5", decoded[1])
	}
}

func TestDecodeVectorInvalid(t *testing.T) {
	_, err := decodeVector([]byte{1, 2, 3})
	if err == nil {
		t.Errorf("expected error for 3-byte input")
	}
}
