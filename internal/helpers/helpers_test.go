package helpers_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/jroosing/hydradns/internal/helpers"
	"github.com/stretchr/testify/assert"
)

func TestClampIntToUint16(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want uint16
	}{
		{name: "negative", in: -1, want: 0},
		{name: "zero", in: 0, want: 0},
		{name: "one", in: 1, want: 1},
		{name: "max", in: int(math.MaxUint16), want: math.MaxUint16},
		{name: "above-max", in: int(math.MaxUint16) + 1, want: math.MaxUint16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, helpers.ClampIntToUint16(tt.in))
		})
	}
}

func TestClampInt(t *testing.T) {
	tests := []struct {
		name       string
		v          int
		lowerLimit int
		upperLimit int
		want       int
	}{
		{name: "below", v: 0, lowerLimit: 10, upperLimit: 20, want: 10},
		{name: "inside", v: 15, lowerLimit: 10, upperLimit: 20, want: 15},
		{name: "above", v: 25, lowerLimit: 10, upperLimit: 20, want: 20},
		{name: "at-lower", v: 10, lowerLimit: 10, upperLimit: 20, want: 10},
		{name: "at-upper", v: 20, lowerLimit: 10, upperLimit: 20, want: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, helpers.ClampInt(tt.v, tt.lowerLimit, tt.upperLimit))
		})
	}
}

func TestClampIntToUint32(t *testing.T) {
	assert.Equal(t, uint32(0), helpers.ClampIntToUint32(-1))
	assert.Equal(t, uint32(0), helpers.ClampIntToUint32(0))
	assert.Equal(t, uint32(1), helpers.ClampIntToUint32(1))

	if strconv.IntSize == 64 {
		assert.Equal(t, uint32(math.MaxUint32), helpers.ClampIntToUint32(int(math.MaxUint32)))
		assert.Equal(t, uint32(math.MaxUint32), helpers.ClampIntToUint32(int(math.MaxUint32)+1))
	}
}

func TestClampUint32ToUint8(t *testing.T) {
	tests := []struct {
		name string
		in   uint32
		want uint8
	}{
		{name: "zero", in: 0, want: 0},
		{name: "one", in: 1, want: 1},
		{name: "max", in: uint32(math.MaxUint8), want: math.MaxUint8},
		{name: "above-max", in: uint32(math.MaxUint8) + 1, want: math.MaxUint8},
		{name: "way-above-max", in: math.MaxUint32, want: math.MaxUint8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, helpers.ClampUint32ToUint8(tt.in))
		})
	}
}
