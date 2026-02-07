// Package helpers provides utility functions for type conversions and numeric clamping.
//
// These helpers are used throughout HydraDNS for safe type conversions that may lose
// precision (e.g., int to uint16). They prevent overflow and underflow by clamping
// values to valid ranges for the target type.
package helpers

import "math"

// clampInt restricts v to the range [minVal, maxVal].
// Used internally for int-based clamping.
func clampInt(v, minVal, maxVal int) int {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// ClampInt restricts v to the range [lowerLimit, upperLimit].
func ClampInt(v, lowerLimit, upperLimit int) int {
	return clampInt(v, lowerLimit, upperLimit)
}

// ClampIntToUint16 converts v to uint16 with clamping.
// Values below 0 become 0; values above math.MaxUint16 become math.MaxUint16.
func ClampIntToUint16(v int) uint16 {
	clamped := clampInt(v, 0, math.MaxUint16)
	return uint16(clamped) //nolint:gosec // clamped to valid range
}

// ClampIntToUint32 converts v to uint32 with clamping.
// Values below 0 become 0; values above math.MaxUint32 become math.MaxUint32.
func ClampIntToUint32(v int) uint32 {
	clamped := clampInt(v, 0, math.MaxUint32)
	return uint32(clamped) //nolint:gosec // clamped to valid range
}

// ClampUint32ToUint8 converts v to uint8 with clamping.
// Values above math.MaxUint8 become math.MaxUint8.
func ClampUint32ToUint8(v uint32) uint8 {
	if v > math.MaxUint8 {
		return math.MaxUint8
	}
	return uint8(v)
}
