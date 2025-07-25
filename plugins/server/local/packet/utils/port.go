package utils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PortRange represents a port range [start, end]
type PortRange struct {
	Start int
	End   int
}

// rangePortMatcher implements PortMatcher using sorted ranges
type rangePortMatcher struct {
	ranges []PortRange
}

// bitmapPortMatcher implements PortMatcher using bitmap
type bitmapPortMatcher struct {
	bitmap [65536]bool // ports 0-65535
}

type PortMatcher interface {
	IsMatch(port int) bool
	Merge(other PortMatcher) (PortMatcher, error)
	IsOverlapped(other PortMatcher) bool
}

// NewRangePortMatcher creates a range-based port matcher
func NewRangePortMatcher(ranges []PortRange) PortMatcher {
	// Sort and merge overlapping ranges
	if len(ranges) == 0 {
		return &rangePortMatcher{ranges: []PortRange{}}
	}

	// Sort by start port
	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Start < ranges[j].Start
	})

	// Merge overlapping ranges
	merged := []PortRange{ranges[0]}
	for i := 1; i < len(ranges); i++ {
		last := &merged[len(merged)-1]
		current := ranges[i]

		if current.Start <= last.End+1 {
			// Overlapping or adjacent, merge
			if current.End > last.End {
				last.End = current.End
			}
		} else {
			// No overlap, add new range
			merged = append(merged, current)
		}
	}

	return &rangePortMatcher{ranges: merged}
}

// NewBitmapPortMatcher creates a bitmap-based port matcher
func NewBitmapPortMatcher(ranges []PortRange) PortMatcher {
	pm := &bitmapPortMatcher{}
	for _, r := range ranges {
		for port := r.Start; port <= r.End; port++ {
			if port >= 0 && port < 65536 {
				pm.bitmap[port] = true
			}
		}
	}
	return pm
}

// Range-based implementation
func (rpm *rangePortMatcher) IsMatch(port int) bool {
	for _, r := range rpm.ranges {
		if port >= r.Start && port <= r.End {
			return true
		}
		if port < r.Start {
			break // ranges are sorted
		}
	}
	return false
}

func (rpm *rangePortMatcher) Merge(other PortMatcher) (PortMatcher, error) {
	otherRange, ok := other.(*rangePortMatcher)
	if !ok {
		return nil, fmt.Errorf("cannot merge different PortMatcher types")
	}

	allRanges := append(rpm.ranges, otherRange.ranges...)
	return NewRangePortMatcher(allRanges), nil
}

func (rpm *rangePortMatcher) IsOverlapped(other PortMatcher) bool {
	otherRange, ok := other.(*rangePortMatcher)
	if !ok {
		return false
	}

	for _, r1 := range rpm.ranges {
		for _, r2 := range otherRange.ranges {
			if r1.Start <= r2.End && r2.Start <= r1.End {
				return true
			}
		}
	}
	return false
}

// Bitmap-based implementation
func (bpm *bitmapPortMatcher) IsMatch(port int) bool {
	if port < 0 || port >= 65536 {
		return false
	}
	return bpm.bitmap[port]
}

func (bpm *bitmapPortMatcher) Merge(other PortMatcher) (PortMatcher, error) {
	otherBitmap, ok := other.(*bitmapPortMatcher)
	if !ok {
		return nil, fmt.Errorf("cannot merge different PortMatcher types")
	}

	result := &bitmapPortMatcher{}
	for i := 0; i < 65536; i++ {
		result.bitmap[i] = bpm.bitmap[i] || otherBitmap.bitmap[i]
	}
	return result, nil
}

func (bpm *bitmapPortMatcher) IsOverlapped(other PortMatcher) bool {
	otherBitmap, ok := other.(*bitmapPortMatcher)
	if !ok {
		return false
	}

	for i := 0; i < 65536; i++ {
		if bpm.bitmap[i] && otherBitmap.bitmap[i] {
			return true
		}
	}
	return false
}

func NewPortMatcher(ports string, useBitmap bool) (PortMatcher, error) {
	// Parse the port string into PortMatcher
	// while parsing, we can also handle the case of single port like "8080" or "9090-9095" or both in a single string
	// In a real implementation, you would parse the string properly.
	// besides, we need validate the ports and ranges. it should be in the range of 0-65535.

	if ports == "" {
		return nil, fmt.Errorf("ports string cannot be empty")
	}

	ranges, err := parsePortString(ports)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ports: %w", err)
	}

	if len(ranges) == 0 {
		return nil, fmt.Errorf("no valid ports found in: %s", ports)
	}

	if useBitmap {
		return NewBitmapPortMatcher(ranges), nil
	}
	return NewRangePortMatcher(ranges), nil
}

// parsePortString parses a port string like "8080,9090-9095,3000"
func parsePortString(ports string) ([]PortRange, error) {
	var portRanges []PortRange
	portSpecs := strings.Split(ports, ",")

	for _, spec := range portSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		if strings.Contains(spec, "-") {
			// Handle port range like "9090-9095"
			parts := strings.Split(spec, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid port range format: %s", spec)
			}

			start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start port in range %s: %w", spec, err)
			}

			end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end port in range %s: %w", spec, err)
			}

			if err := validatePortRange(start, end); err != nil {
				return nil, fmt.Errorf("invalid port range %s: %w", spec, err)
			}

			portRanges = append(portRanges, PortRange{Start: start, End: end})
		} else {
			// Handle single port like "8080"
			port, err := strconv.Atoi(spec)
			if err != nil {
				return nil, fmt.Errorf("invalid port number %s: %w", spec, err)
			}

			if err := validatePort(port); err != nil {
				return nil, fmt.Errorf("invalid port %s: %w", spec, err)
			}

			portRanges = append(portRanges, PortRange{Start: port, End: port})
		}
	}

	return portRanges, nil
}

// validatePort validates a single port number
func validatePort(port int) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("port number must be between 0 and 65535, got %d", port)
	}
	return nil
}

// validatePortRange validates a port range
func validatePortRange(start, end int) error {
	if err := validatePort(start); err != nil {
		return fmt.Errorf("start port: %w", err)
	}
	if err := validatePort(end); err != nil {
		return fmt.Errorf("end port: %w", err)
	}
	if start > end {
		return fmt.Errorf("start port (%d) cannot be greater than end port (%d)", start, end)
	}
	return nil
}
