// krb/utils.go
package krb

import (
	"encoding/binary"
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
