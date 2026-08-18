package base62_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/internal/base62"
)

func enc() *base62.Base62Encoder { return base62.NewEncoder() }

func TestEncode_Zero(t *testing.T) {
	s, err := enc().Encode(0)
	require.NoError(t, err)
	assert.Equal(t, "0", s)
}

func TestEncode_Negative_ReturnsError(t *testing.T) {
	_, err := enc().Encode(-1)
	assert.Error(t, err)
}

func TestEncode_KnownValues(t *testing.T) {
	cases := []struct {
		id   int64
		want string
	}{
		{1, "1"}, {9, "9"}, {10, "A"}, {35, "Z"},
		{36, "a"}, {61, "z"}, {62, "10"}, {3844, "100"},
	}
	for _, tc := range cases {
		got, err := enc().Encode(tc.id)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got, "Encode(%d)", tc.id)
	}
}

func TestDecode_EmptyString_ReturnsError(t *testing.T) {
	_, err := enc().Decode("")
	assert.Error(t, err)
}

func TestDecode_InvalidChar_ReturnsError(t *testing.T) {
	for _, s := range []string{"-1", "abc!", "привет", " "} {
		_, err := enc().Decode(s)
		assert.Error(t, err, "input %q must be rejected", s)
	}
}

func TestDecode_KnownValues(t *testing.T) {
	cases := []struct {
		str  string
		want int64
	}{
		{"0", 0}, {"1", 1}, {"A", 10}, {"Z", 35},
		{"a", 36}, {"z", 61}, {"10", 62}, {"100", 3844},
	}
	for _, tc := range cases {
		got, err := enc().Decode(tc.str)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got, "Decode(%q)", tc.str)
	}
}

func TestRoundtrip(t *testing.T) {
	cases := []int64{0, 1, 61, 62, 3843, 3844, 1_000_000, math.MaxInt64}
	e := enc()
	for _, id := range cases {
		s, err := e.Encode(id)
		require.NoError(t, err, "Encode(%d)", id)
		got, err := e.Decode(s)
		require.NoError(t, err, "Decode(%q)", s)
		assert.Equal(t, id, got, "roundtrip(%d)", id)
	}
}

func TestEncode_MaxLength(t *testing.T) {
	// math.MaxInt64 должен умещаться в 11 символов
	s, err := enc().Encode(math.MaxInt64)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(s), 11)
}

func BenchmarkEncode(b *testing.B) {
	e := base62.NewEncoder()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = e.Encode(7816251636736237568)
	}
}
