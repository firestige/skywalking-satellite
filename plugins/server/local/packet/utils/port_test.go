package utils

import (
	"testing"
)

func TestNewPortMatcher(t *testing.T) {
	tests := []struct {
		name        string
		ports       string
		useBitmap   bool
		expectError bool
		expected    []PortRange
	}{
		// Basic valid cases
		{
			name:      "single port",
			ports:     "8080",
			useBitmap: false,
			expected:  []PortRange{{Start: 8080, End: 8080}},
		},
		{
			name:      "multiple single ports",
			ports:     "80,443,8080",
			useBitmap: false,
			expected:  []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}, {Start: 8080, End: 8080}},
		},
		{
			name:      "port range",
			ports:     "8000-8100",
			useBitmap: false,
			expected:  []PortRange{{Start: 8000, End: 8100}},
		},
		{
			name:      "mixed ports and ranges",
			ports:     "80,8000-8100,9000",
			useBitmap: false,
			expected:  []PortRange{{Start: 80, End: 80}, {Start: 8000, End: 8100}, {Start: 9000, End: 9000}},
		},

		// Robustness tests - whitespace handling
		{
			name:      "spaces around commas",
			ports:     "80 , 443 , 8080",
			useBitmap: false,
			expected:  []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}, {Start: 8080, End: 8080}},
		},
		{
			name:      "spaces around dashes",
			ports:     "8000 - 8100",
			useBitmap: false,
			expected:  []PortRange{{Start: 8000, End: 8100}},
		},
		{
			name:      "mixed whitespace",
			ports:     " 80 , 8000 - 8100 , 9000 ",
			useBitmap: false,
			expected:  []PortRange{{Start: 80, End: 80}, {Start: 8000, End: 8100}, {Start: 9000, End: 9000}},
		},
		{
			name:      "tabs and multiple spaces",
			ports:     "80,\t8000\t-\t8100,  9000  ",
			useBitmap: false,
			expected:  []PortRange{{Start: 80, End: 80}, {Start: 8000, End: 8100}, {Start: 9000, End: 9000}},
		},

		// Empty segments handling
		{
			name:      "empty segments between commas",
			ports:     "80,,443,",
			useBitmap: false,
			expected:  []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}},
		},
		{
			name:      "leading and trailing commas",
			ports:     ",80,443,",
			useBitmap: false,
			expected:  []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}},
		},

		// Boundary values
		{
			name:      "port 0",
			ports:     "0",
			useBitmap: false,
			expected:  []PortRange{{Start: 0, End: 0}},
		},
		{
			name:      "port 65535",
			ports:     "65535",
			useBitmap: false,
			expected:  []PortRange{{Start: 65535, End: 65535}},
		},
		{
			name:      "full range",
			ports:     "0-65535",
			useBitmap: false,
			expected:  []PortRange{{Start: 0, End: 65535}},
		},

		// Range direction tests
		{
			name:        "reverse range should fail",
			ports:       "8100-8000",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:      "same start and end",
			ports:     "8080-8080",
			useBitmap: false,
			expected:  []PortRange{{Start: 8080, End: 8080}},
		},

		// Error cases
		{
			name:        "empty string",
			ports:       "",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "only whitespace",
			ports:       "   ",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "only commas",
			ports:       ",,,",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "invalid port number",
			ports:       "abc",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "negative port",
			ports:       "-1",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "port too large",
			ports:       "65536",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "invalid range format - too many dashes",
			ports:       "8000-8050-8100",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "invalid range format - empty start",
			ports:       "-8100",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "invalid range format - empty end",
			ports:       "8000-",
			useBitmap:   false,
			expectError: true,
		},
		{
			name:        "mixed valid and invalid",
			ports:       "80,abc,443",
			useBitmap:   false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := NewPortMatcher(tt.ports, tt.useBitmap)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if matcher == nil {
				t.Error("matcher should not be nil")
				return
			}

			// Test that the matcher works correctly
			testPortMatching(t, matcher, tt.expected)
		})
	}
}

func testPortMatching(t *testing.T, matcher PortMatcher, expected []PortRange) {
	// Test ports that should match
	for _, r := range expected {
		// Test start, middle, and end of each range
		if !matcher.IsMatch(r.Start) {
			t.Errorf("port %d should match", r.Start)
		}
		if !matcher.IsMatch(r.End) {
			t.Errorf("port %d should match", r.End)
		}
		if r.Start < r.End {
			middle := (r.Start + r.End) / 2
			if !matcher.IsMatch(middle) {
				t.Errorf("port %d should match", middle)
			}
		}
	}

	// Test some ports that should not match
	nonMatchingPorts := []int{1, 2, 22, 999, 10000, 20000, 30000, 40000, 50000, 60000}
	for _, port := range nonMatchingPorts {
		shouldMatch := false
		for _, r := range expected {
			if port >= r.Start && port <= r.End {
				shouldMatch = true
				break
			}
		}
		if matcher.IsMatch(port) != shouldMatch {
			if shouldMatch {
				t.Errorf("port %d should match but doesn't", port)
			} else {
				t.Errorf("port %d should not match but does", port)
			}
		}
	}
}

func TestPortMatcherBothImplementations(t *testing.T) {
	testCases := []string{
		"80,443",
		"8000-8100",
		"80,443,8000-8100,9000",
		"0,65535",
		"1000-2000,3000-4000",
	}

	for _, ports := range testCases {
		t.Run("comparing_implementations_"+ports, func(t *testing.T) {
			rangeMatcher, err1 := NewPortMatcher(ports, false)
			bitmapMatcher, err2 := NewPortMatcher(ports, true)

			if err1 != nil || err2 != nil {
				t.Fatalf("unexpected errors: range=%v, bitmap=%v", err1, err2)
			}

			// Test that both implementations give the same results
			testPorts := []int{0, 79, 80, 81, 442, 443, 444, 7999, 8000, 8050, 8100, 8101, 8999, 9000, 9001, 65534, 65535}
			for _, port := range testPorts {
				rangeResult := rangeMatcher.IsMatch(port)
				bitmapResult := bitmapMatcher.IsMatch(port)
				if rangeResult != bitmapResult {
					t.Errorf("port %d: range=%v, bitmap=%v", port, rangeResult, bitmapResult)
				}
			}
		})
	}
}

func TestParsePortStringEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		expected    []PortRange
	}{
		{
			name:     "unicode spaces",
			input:    "80\u00A0,\u2000443\u2001", // non-breaking space, em space, en space
			expected: []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}},
		},
		{
			name:        "decimal point",
			input:       "80.5",
			expectError: true,
		},
		{
			name:        "scientific notation",
			input:       "8e3",
			expectError: true,
		},
		{
			name:        "hexadecimal",
			input:       "0x50",
			expectError: true,
		},
		{
			name:     "leading zeros",
			input:    "0080,00443",
			expected: []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}},
		},
		{
			name:        "very large number",
			input:       "999999999",
			expectError: true,
		},
		{
			name:        "multiple dashes in sequence",
			input:       "8000--8100",
			expectError: true,
		},
		{
			name:        "range with spaces but no numbers",
			input:       " - ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges, err := parsePortString(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(ranges) != len(tt.expected) {
				t.Errorf("expected %d ranges, got %d", len(tt.expected), len(ranges))
				return
			}

			for i, expected := range tt.expected {
				if ranges[i].Start != expected.Start || ranges[i].End != expected.End {
					t.Errorf("range %d: expected %v, got %v", i, expected, ranges[i])
				}
			}
		})
	}
}

// Existing benchmark and test functions remain the same...
// (keeping the previous benchmark tests)

func BenchmarkRangePortMatcher_IsMatch(b *testing.B) {
	ranges := []PortRange{
		{80, 90},
		{443, 443},
		{8000, 8100},
		{9000, 9050},
		{3000, 3010},
	}
	matcher := NewRangePortMatcher(ranges)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.IsMatch(8080)
	}
}

func BenchmarkBitmapPortMatcher_IsMatch(b *testing.B) {
	ranges := []PortRange{
		{80, 90},
		{443, 443},
		{8000, 8100},
		{9000, 9050},
		{3000, 3010},
	}
	matcher := NewBitmapPortMatcher(ranges)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.IsMatch(8080)
	}
}

func BenchmarkRangePortMatcher_Merge(b *testing.B) {
	ranges1 := []PortRange{{80, 90}, {443, 443}, {8000, 8100}}
	ranges2 := []PortRange{{9000, 9050}, {3000, 3010}}
	matcher1 := NewRangePortMatcher(ranges1)
	matcher2 := NewRangePortMatcher(ranges2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher1.Merge(matcher2)
	}
}

func BenchmarkBitmapPortMatcher_Merge(b *testing.B) {
	ranges1 := []PortRange{{80, 90}, {443, 443}, {8000, 8100}}
	ranges2 := []PortRange{{9000, 9050}, {3000, 3010}}
	matcher1 := NewBitmapPortMatcher(ranges1)
	matcher2 := NewBitmapPortMatcher(ranges2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher1.Merge(matcher2)
	}
}

func BenchmarkRangePortMatcher_IsOverlapped(b *testing.B) {
	ranges1 := []PortRange{{80, 90}, {443, 443}, {8000, 8100}}
	ranges2 := []PortRange{{85, 95}, {9000, 9050}}
	matcher1 := NewRangePortMatcher(ranges1)
	matcher2 := NewRangePortMatcher(ranges2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher1.IsOverlapped(matcher2)
	}
}

func BenchmarkBitmapPortMatcher_IsOverlapped(b *testing.B) {
	ranges1 := []PortRange{{80, 90}, {443, 443}, {8000, 8100}}
	ranges2 := []PortRange{{85, 95}, {9000, 9050}}
	matcher1 := NewBitmapPortMatcher(ranges1)
	matcher2 := NewBitmapPortMatcher(ranges2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher1.IsOverlapped(matcher2)
	}
}

func BenchmarkPortMatchers(b *testing.B) {
	scenarios := []struct {
		name   string
		ranges []PortRange
	}{
		{"SmallRanges", []PortRange{{80, 90}, {443, 443}}},
		{"MediumRanges", []PortRange{{80, 90}, {443, 443}, {8000, 8100}, {9000, 9050}}},
		{"LargeRanges", []PortRange{{80, 180}, {443, 543}, {8000, 8100}, {9000, 9100}, {3000, 3100}}},
	}

	for _, scenario := range scenarios {
		b.Run("Range_"+scenario.name+"_IsMatch", func(b *testing.B) {
			matcher := NewRangePortMatcher(scenario.ranges)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				matcher.IsMatch(8080)
			}
		})

		b.Run("Bitmap_"+scenario.name+"_IsMatch", func(b *testing.B) {
			matcher := NewBitmapPortMatcher(scenario.ranges)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				matcher.IsMatch(8080)
			}
		})
	}
}

// Test correctness
func TestPortMatchers(t *testing.T) {
	ranges := []PortRange{{80, 90}, {443, 443}, {8000, 8100}}

	rangeMatcher := NewRangePortMatcher(ranges)
	bitmapMatcher := NewBitmapPortMatcher(ranges)

	testPorts := []struct {
		port     int
		expected bool
	}{
		{80, true},
		{85, true},
		{90, true},
		{91, false},
		{443, true},
		{444, false},
		{8050, true},
		{7999, false},
		{8101, false},
	}

	for _, test := range testPorts {
		if rangeMatcher.IsMatch(test.port) != test.expected {
			t.Errorf("Range matcher: port %d expected %v", test.port, test.expected)
		}
		if bitmapMatcher.IsMatch(test.port) != test.expected {
			t.Errorf("Bitmap matcher: port %d expected %v", test.port, test.expected)
		}
	}
}
