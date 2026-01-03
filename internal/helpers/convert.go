package helpers

import "math"

// ClampInt restricts v to the range [lowerLimit, upperLimit].
func ClampInt(v, lowerLimit, upperLimit int) int {
	if v < lowerLimit {
		return lowerLimit
	}
	if v > upperLimit {
		return upperLimit
	}
	return v
}

// ClampIntToUint16 converts v to uint16 with clamping.
// Values below 0 become 0; values above math.MaxUint16 become math.MaxUint16.
func ClampIntToUint16(v int) uint16 {
	if v < 0 {
		return 0
	}
	if v > math.MaxUint16 {
		return math.MaxUint16
	}
	return uint16(v)
}

// ClampIntToUint32 converts v to uint32 with clamping.
// Values below 0 become 0; values above math.MaxUint32 become math.MaxUint32.
func ClampIntToUint32(v int) uint32 {
	if v < 0 {
		return 0
	}
	if v > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v)
}

// ClampUint32ToUint8 converts v to uint8 with clamping.
// Values above math.MaxUint8 become math.MaxUint8.
func ClampUint32ToUint8(v uint32) uint8 {
	if v > math.MaxUint8 {
		return math.MaxUint8
	}
	return uint8(v)
}
