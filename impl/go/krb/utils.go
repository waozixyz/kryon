// krb/utils.go
package krb

import (
	"encoding/binary"
	"math"
)

// ReadU16LE reads a little-endian uint16 from a byte slice.
// It's a common utility for parsing KRB data.
func ReadU16LE(data []byte) uint16 {
	if len(data) < 2 {
		// Consider logging an error or returning an error
		return 0
	}
	return binary.LittleEndian.Uint16(data)
}

// ReadU32LE reads a little-endian uint32 from a byte slice.
func ReadU32LE(data []byte) uint32 {
	if len(data) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}


// --- Math and Scaling Helpers ---
func scaledF32(value uint8, scale float32) float32 {
	return float32(value) * scale
}

func scaledI32(value uint8, scale float32) int32 {
	// Round to nearest integer for pixel values
	return int32(math.Round(float64(value) * float64(scale)))
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func MuxFloat32(cond bool, valTrue, valFalse float32) float32 {
	if cond {
		return valTrue
	}
	return valFalse
}

func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}