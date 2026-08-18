package utils

import (
	"testing"
	"urlshortener/internal/idgenerator"
)

func TestToBase62_Zero(t *testing.T) {
	s, err := ToBase62(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "0" {
		t.Errorf("expected '0', got %q", s)
	}
}

func TestToBase62_Negative(t *testing.T) {
	_, err := ToBase62(-1)
	if err == nil {
		t.Error("expected error for negative input")
	}
}

func TestToBase62_KnownValues(t *testing.T) {
	cases := []struct {
		id  int64
		str string
	}{
		{1, "1"},
		{9, "9"},
		{10, "A"},
		{35, "Z"},
		{36, "a"},
		{61, "z"},
		{62, "10"},
		{3844, "100"}, // 62^2
	}
	for _, tc := range cases {
		got, err := ToBase62(tc.id)
		if err != nil {
			t.Errorf("ToBase62(%d) error: %v", tc.id, err)
			continue
		}
		if got != tc.str {
			t.Errorf("ToBase62(%d) = %q, want %q", tc.id, got, tc.str)
		}
	}
}

func TestFromBase62_KnownValues(t *testing.T) {
	cases := []struct {
		str string
		id  int64
	}{
		{"0", 0},
		{"1", 1},
		{"A", 10},
		{"Z", 35},
		{"a", 36},
		{"z", 61},
		{"10", 62},
		{"100", 3844},
	}
	for _, tc := range cases {
		got, err := FromBase62(tc.str)
		if err != nil {
			t.Errorf("FromBase62(%q) error: %v", tc.str, err)
			continue
		}
		if got != tc.id {
			t.Errorf("FromBase62(%q) = %d, want %d", tc.str, got, tc.id)
		}
	}
}

func TestFromBase62_InvalidChar(t *testing.T) {
	invalid := []string{"-1", "abc!", "привет", " "}
	for _, s := range invalid {
		_, err := FromBase62(s)
		if err == nil {
			t.Errorf("expected error for input %q", s)
		}
	}
}

func TestFromBase62_EmptyString(t *testing.T) {
	_, err := FromBase62("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

// Roundtrip: ToBase62 → FromBase62 должен вернуть исходное число
func TestRoundtrip(t *testing.T) {
	cases := []int64{
		0, 1, 61, 62, 3843, 3844,
		1_000_000,
		9_223_372_036_854_775_807, // math.MaxInt64
	}
	for _, id := range cases {
		s, err := ToBase62(id)
		if err != nil {
			t.Fatalf("ToBase62(%d) error: %v", id, err)
		}
		got, err := FromBase62(s)
		if err != nil {
			t.Fatalf("FromBase62(%q) error: %v", s, err)
		}
		if got != id {
			t.Errorf("roundtrip(%d): got %d via %q", id, got, s)
		}
	}
}

// Проверяем длину строки для реальных Snowflake ID
func TestToBase62_SnowflakeLength(t *testing.T) {
	g, _ := idgenerator.NewIDGenerator(idgenerator.Config{DatacenterID: 1, MachineID: 1})
	for i := 0; i < 100; i++ {
		id, _ := g.NextID()
		s, err := ToBase62(id)
		if err != nil {
			t.Fatalf("ToBase62 error: %v", err)
		}
		// Snowflake ID в Base62 занимает не более 11 символов
		if len(s) > 11 {
			t.Errorf("encoded length %d > 11 for id %d: %q", len(s), id, s)
		}
	}
}

func BenchmarkToBase62(b *testing.B) {
	g, _ := idgenerator.NewIDGenerator(idgenerator.Config{DatacenterID: 1, MachineID: 1})
	id, _ := g.NextID()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ToBase62(id)
	}
}
