package krb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log" // Import log for potential debug messages
)

// ReadDocument parses a KRB file from the given reader into a Document struct.
// The reader must also implement io.Seeker for random access.
func ReadDocument(r io.ReadSeeker) (*Document, error) {
	doc := &Document{}

	// --- 1. Read Header ---
	headerBuf := make([]byte, HeaderSize)
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("krb read: failed to seek to header: %w", err)
	}
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, fmt.Errorf("krb read: failed to read header: %w", err)
	}

	// Parse header fields
	copy(doc.Header.Magic[:], headerBuf[0:4])
	doc.Header.Version = readU16LE(headerBuf[4:6])
	doc.Header.Flags = readU16LE(headerBuf[6:8])
	doc.Header.ElementCount = readU16LE(headerBuf[8:10])
	doc.Header.StyleCount = readU16LE(headerBuf[10:12])
	doc.Header.AnimationCount = readU16LE(headerBuf[12:14])
	doc.Header.StringCount = readU16LE(headerBuf[14:16])
	doc.Header.ResourceCount = readU16LE(headerBuf[16:18])
	doc.Header.ElementOffset = readU32LE(headerBuf[18:22])
	doc.Header.StyleOffset = readU32LE(headerBuf[22:26])
	doc.Header.AnimationOffset = readU32LE(headerBuf[26:30])
	doc.Header.StringOffset = readU32LE(headerBuf[30:34])
	doc.Header.ResourceOffset = readU32LE(headerBuf[34:38])
	doc.Header.TotalSize = readU32LE(headerBuf[38:42])

	// Validate Magic Number
	if !bytes.Equal(doc.Header.Magic[:], MagicNumber[:]) {
		return nil, fmt.Errorf("krb read: invalid magic number %v", doc.Header.Magic)
	}

	// Store parsed version
	doc.VersionMajor = uint8(doc.Header.Version & 0x00FF)
	doc.VersionMinor = uint8(doc.Header.Version >> 8)

	// Optional: Version check (maybe just warn)
	if doc.Header.Version != ExpectedVersion {
		fmt.Printf("Warning: KRB version mismatch. File is %d.%d, reader expects %d.%d. Parsing continues...\n",
			doc.VersionMajor, doc.VersionMinor, SpecVersionMajor, SpecVersionMinor)
	}

	// Basic Offset Sanity Checks
	if doc.Header.ElementCount > 0 && doc.Header.ElementOffset < HeaderSize {
		return nil, errors.New("krb read: element offset overlaps header")
	}
	if doc.Header.StyleCount > 0 && doc.Header.StyleOffset < HeaderSize {
		return nil, errors.New("krb read: style offset overlaps header")
	}
	// ... add checks for other offsets if needed

	// --- 2. Read Element Blocks ---
	if doc.Header.ElementCount > 0 {
		// *** MODIFICATION START: Initialize slices ***
		doc.Elements = make([]ElementHeader, doc.Header.ElementCount)
		doc.ElementStartOffsets = make([]uint32, doc.Header.ElementCount) // Initialize offset slice
		doc.Properties = make([][]Property, doc.Header.ElementCount)
		doc.CustomProperties = make([][]CustomProperty, doc.Header.ElementCount)
		doc.Events = make([][]EventFileEntry, doc.Header.ElementCount)
		doc.AnimationRefs = make([][]AnimationRef, doc.Header.ElementCount)
		doc.ChildRefs = make([][]ChildRef, doc.Header.ElementCount)
		// *** MODIFICATION END: Initialize slices ***

		// Seek to the start of the element blocks section
		if _, err := r.Seek(int64(doc.Header.ElementOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to elements offset %d: %w", doc.Header.ElementOffset, err)
		}

		// Prepare reusable buffers
		elementHeaderBuf := make([]byte, ElementHeaderSize)
		propertyHeaderBuf := make([]byte, 3)        // ID(1)+Type(1)+Size(1)
		customPropertyHeaderBuf := make([]byte, 3) // KeyIdx(1)+Type(1)+Size(1)

		// Loop through each element defined in the header
		for i := uint16(0); i < doc.Header.ElementCount; i++ {

			// *** MODIFICATION START: Capture element header start offset ***
			currentPos, err := r.Seek(0, io.SeekCurrent) // Get current position
			if err != nil {
				return nil, fmt.Errorf("krb read: failed to get current position before reading element header %d: %w", i, err)
			}
			// Ensure index is within bounds before assigning
			if i < uint16(len(doc.ElementStartOffsets)) {
				doc.ElementStartOffsets[i] = uint32(currentPos)
				// Optional Debug Log:
				// log.Printf("Debug krb read: Element %d header starts at offset %d (0x%X)", i, currentPos, currentPos)
			} else {
				// This should not happen if initialization was correct, but safeguard anyway
				return nil, fmt.Errorf("krb read: element index %d is out of bounds for ElementStartOffsets (len %d)", i, len(doc.ElementStartOffsets))
			}
			// *** MODIFICATION END: Capture element header start offset ***

			// Read Element Header Data
			if _, err := io.ReadFull(r, elementHeaderBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read element header %d (expected size %d) at offset %d: %w", i, ElementHeaderSize, currentPos, err)
			}

			// Parse Element Header Fields
			doc.Elements[i] = ElementHeader{
				Type:            ElementType(elementHeaderBuf[0]),
				ID:              elementHeaderBuf[1],
				PosX:            readU16LE(elementHeaderBuf[2:4]),
				PosY:            readU16LE(elementHeaderBuf[4:6]),
				Width:           readU16LE(elementHeaderBuf[6:8]),
				Height:          readU16LE(elementHeaderBuf[8:10]),
				Layout:          elementHeaderBuf[10],
				StyleID:         elementHeaderBuf[11],
				PropertyCount:   elementHeaderBuf[12],
				ChildCount:      elementHeaderBuf[13],
				EventCount:      elementHeaderBuf[14],
				AnimationCount:  elementHeaderBuf[15],
				CustomPropCount: elementHeaderBuf[16], // v0.3
			}
			elemHdr := &doc.Elements[i] // Pointer for convenience

			// --- Read Standard Properties ---
			if elemHdr.PropertyCount > 0 {
				// Initialize the properties slice for this element
				doc.Properties[i] = make([]Property, elemHdr.PropertyCount)
				// Read each property
				for j := uint8(0); j < elemHdr.PropertyCount; j++ {
					// Read property header (ID, Type, Size)
					if _, err := io.ReadFull(r, propertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read property header (%d/%d) for element %d: %w", j+1, elemHdr.PropertyCount, i, err)
					}
					prop := &doc.Properties[i][j]
					prop.ID = PropertyID(propertyHeaderBuf[0])
					prop.ValueType = ValueType(propertyHeaderBuf[1])
					prop.Size = propertyHeaderBuf[2]
					// Read property value if size > 0
					if prop.Size > 0 {
						prop.Value = make([]byte, prop.Size)
						if _, err := io.ReadFull(r, prop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read property value (size %d) (%d/%d) for element %d: %w", prop.Size, j+1, elemHdr.PropertyCount, i, err)
						}
					}
				}
			}

			// --- Read Custom Properties (v0.3) ---
			if elemHdr.CustomPropCount > 0 {
				// Initialize the custom properties slice for this element
				doc.CustomProperties[i] = make([]CustomProperty, elemHdr.CustomPropCount)
				// Read each custom property
				for j := uint8(0); j < elemHdr.CustomPropCount; j++ {
					// Read custom property header (KeyIndex, Type, Size)
					if _, err := io.ReadFull(r, customPropertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read custom property header (%d/%d) for element %d: %w", j+1, elemHdr.CustomPropCount, i, err)
					}
					cprop := &doc.CustomProperties[i][j]
					cprop.KeyIndex = customPropertyHeaderBuf[0]
					cprop.ValueType = ValueType(customPropertyHeaderBuf[1])
					cprop.Size = customPropertyHeaderBuf[2]
					// Read custom property value if size > 0
					if cprop.Size > 0 {
						cprop.Value = make([]byte, cprop.Size)
						if _, err := io.ReadFull(r, cprop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read custom property value (size %d) (%d/%d) for element %d: %w", cprop.Size, j+1, elemHdr.CustomPropCount, i, err)
						}
					}
				}
			}

			// --- Read Events ---
			if elemHdr.EventCount > 0 {
				doc.Events[i] = make([]EventFileEntry, elemHdr.EventCount)
				eventDataSize := int(elemHdr.EventCount) * EventFileEntrySize
				eventBuf := make([]byte, eventDataSize)
				if _, err := io.ReadFull(r, eventBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read events block (count %d, size %d) for element %d: %w", elemHdr.EventCount, eventDataSize, i, err)
				}
				// Parse individual event entries from the buffer
				for j := uint8(0); j < elemHdr.EventCount; j++ {
					offset := int(j) * EventFileEntrySize
					// Boundary check for safety, though ReadFull should guarantee size
					if offset+EventFileEntrySize <= len(eventBuf) {
						doc.Events[i][j] = EventFileEntry{
							EventType:  EventType(eventBuf[offset]),
							CallbackID: eventBuf[offset+1],
						}
					} else {
						return nil, fmt.Errorf("krb read: buffer underrun reading event %d/%d for element %d", j+1, elemHdr.EventCount, i)
					}
				}
			}

			// --- Read Animation References ---
			if elemHdr.AnimationCount > 0 {
				doc.AnimationRefs[i] = make([]AnimationRef, elemHdr.AnimationCount)
				animRefDataSize := int(elemHdr.AnimationCount) * AnimationRefSize
				animRefBuf := make([]byte, animRefDataSize)
				if _, err := io.ReadFull(r, animRefBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read anim refs block (count %d, size %d) for element %d: %w", elemHdr.AnimationCount, animRefDataSize, i, err)
				}
				// Parse individual animation references
				for j := uint8(0); j < elemHdr.AnimationCount; j++ {
					offset := int(j) * AnimationRefSize
					if offset+AnimationRefSize <= len(animRefBuf) {
						doc.AnimationRefs[i][j] = AnimationRef{
							AnimationIndex: animRefBuf[offset],
							Trigger:        animRefBuf[offset+1],
						}
					} else {
						return nil, fmt.Errorf("krb read: buffer underrun reading anim ref %d/%d for element %d", j+1, elemHdr.AnimationCount, i)
					}
				}
			}

			// --- Read Child References ---
			if elemHdr.ChildCount > 0 {
				doc.ChildRefs[i] = make([]ChildRef, elemHdr.ChildCount)
				childRefDataSize := int(elemHdr.ChildCount) * ChildRefSize
				childRefBuf := make([]byte, childRefDataSize)
				if _, err := io.ReadFull(r, childRefBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read child refs block (count %d, size %d) for element %d: %w", elemHdr.ChildCount, childRefDataSize, i, err)
				}
				// Parse individual child references
				for j := uint8(0); j < elemHdr.ChildCount; j++ {
					offset := int(j) * ChildRefSize
					if offset+ChildRefSize <= len(childRefBuf) {
						doc.ChildRefs[i][j] = ChildRef{
							ChildOffset: readU16LE(childRefBuf[offset : offset+ChildRefSize]),
						}
						// Optional Debug Log:
						// log.Printf("Debug krb read: Elem %d ChildRef %d: Offset %d", i, j, doc.ChildRefs[i][j].ChildOffset)
					} else {
						return nil, fmt.Errorf("krb read: buffer underrun reading child ref %d/%d for element %d", j+1, elemHdr.ChildCount, i)
					}
				}
			}
		} // End loop through elements
	} // End if ElementCount > 0

	// --- 3. Read Style Blocks ---
	if doc.Header.StyleCount > 0 {
		doc.Styles = make([]Style, doc.Header.StyleCount)
		if _, err := r.Seek(int64(doc.Header.StyleOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to styles offset %d: %w", doc.Header.StyleOffset, err)
		}

		styleHeaderBuf := make([]byte, 3) // ID(1)+NameIdx(1)+PropCount(1)
		propertyHeaderBuf := make([]byte, 3) // Shared buffer

		for i := uint16(0); i < doc.Header.StyleCount; i++ {
			// Read style header
			if _, err := io.ReadFull(r, styleHeaderBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read style header %d: %w", i, err)
			}
			style := &doc.Styles[i]
			style.ID = styleHeaderBuf[0]            // 1-based
			style.NameIndex = styleHeaderBuf[1]     // 0-based
			style.PropertyCount = styleHeaderBuf[2]

			// Read style properties
			if style.PropertyCount > 0 {
				style.Properties = make([]Property, style.PropertyCount)
				for j := uint8(0); j < style.PropertyCount; j++ {
					if _, err := io.ReadFull(r, propertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read property header (%d/%d) for style %d (ID %d): %w", j+1, style.PropertyCount, i, style.ID, err)
					}
					prop := &style.Properties[j]
					prop.ID = PropertyID(propertyHeaderBuf[0])
					prop.ValueType = ValueType(propertyHeaderBuf[1])
					prop.Size = propertyHeaderBuf[2]
					if prop.Size > 0 {
						prop.Value = make([]byte, prop.Size)
						if _, err := io.ReadFull(r, prop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read property value (size %d) (%d/%d) for style %d (ID %d): %w", prop.Size, j+1, style.PropertyCount, i, style.ID, err)
						}
					}
				}
			}
		}
	}

	// --- 4. Read Animation Table (Placeholder) ---
	if doc.Header.AnimationCount > 0 {
		// Seek to doc.Header.AnimationOffset
		// Read Animation entries based on their type (Transition/Keyframe)
		// Store in doc.Animations (define appropriate structs)
		// For now, just log a warning if animations exist but aren't read
		log.Println("Warning: KRB Animation Table found but parsing is not yet implemented.")
		// Ensure we skip past the animation table if present but unread
		// This requires knowing the size of the animation table, which isn't stored directly.
		// For now, rely on subsequent seeks to String/Resource tables being correct.
	}

	// --- 5. Read String Table ---
	if doc.Header.StringCount > 0 {
		doc.Strings = make([]string, doc.Header.StringCount)
		if _, err := r.Seek(int64(doc.Header.StringOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to strings offset %d: %w", doc.Header.StringOffset, err)
		}

		// Read table header (count)
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read string table count: %w", err)
		}
		tableCount := readU16LE(countBuf)
		if tableCount != doc.Header.StringCount {
			log.Printf("Warning: KRB String Table count mismatch. Header: %d, Table: %d. Using header count.", doc.Header.StringCount, tableCount)
			// Potentially problematic, proceed with header count but log warning.
		}

		lenBuf := make([]byte, 1) // Buffer for length prefix
		for i := uint16(0); i < doc.Header.StringCount; i++ {
			// Read length prefix (1 byte)
			if _, err := io.ReadFull(r, lenBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read string length prefix for index %d: %w", i, err)
			}
			length := uint8(lenBuf[0])

			// Read string data if length > 0
			if length > 0 {
				strBuf := make([]byte, length)
				if _, err := io.ReadFull(r, strBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read string data (len %d) for index %d: %w", length, i, err)
				}
				doc.Strings[i] = string(strBuf) // Assumes UTF-8 encoding
			} else {
				doc.Strings[i] = "" // Empty string if length is 0
			}
		}
	}

	// --- 6. Read Resource Table ---
	if doc.Header.ResourceCount > 0 {
		doc.Resources = make([]Resource, doc.Header.ResourceCount)
		if _, err := r.Seek(int64(doc.Header.ResourceOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to resources offset %d: %w", doc.Header.ResourceOffset, err)
		}

		// Read table header (count)
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read resource table count: %w", err)
		}
		tableCount := readU16LE(countBuf)
		if tableCount != doc.Header.ResourceCount {
			log.Printf("Warning: KRB Resource Table count mismatch. Header: %d, Table: %d. Using header count.", doc.Header.ResourceCount, tableCount)
		}

		// Prepare reusable buffers for resource entry parts
		resEntryCommonBuf := make([]byte, 3) // Type(1), NameIdx(1), Format(1)
		resExternalDataBuf := make([]byte, 1)  // DataStringIndex(1) for External
		resInlineSizeBuf := make([]byte, 2)   // InlineDataSize(2) for Inline

		for i := uint16(0); i < doc.Header.ResourceCount; i++ {
			res := &doc.Resources[i]

			// Read Type, NameIdx, Format
			if _, err := io.ReadFull(r, resEntryCommonBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read resource entry common part %d: %w", i, err)
			}
			res.Type = ResourceType(resEntryCommonBuf[0])
			res.NameIndex = resEntryCommonBuf[1]
			res.Format = ResourceFormat(resEntryCommonBuf[2])

			// Read format-specific data
			switch res.Format {
			case ResFormatExternal:
				if _, err := io.ReadFull(r, resExternalDataBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read external resource data index for index %d: %w", i, err)
				}
				res.DataStringIndex = resExternalDataBuf[0]
				res.InlineData = nil // Ensure nil for external
				res.InlineDataSize = 0

			case ResFormatInline:
				// Read inline data size
				if _, err := io.ReadFull(r, resInlineSizeBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read inline resource size for index %d: %w", i, err)
				}
				res.InlineDataSize = readU16LE(resInlineSizeBuf)
				res.DataStringIndex = 0 // Ensure 0 for inline

				// Read inline data bytes if size > 0
				if res.InlineDataSize > 0 {
					res.InlineData = make([]byte, res.InlineDataSize)
					if _, err := io.ReadFull(r, res.InlineData); err != nil {
						return nil, fmt.Errorf("krb read: failed to read inline resource data (size %d) for index %d: %w", res.InlineDataSize, i, err)
					}
				} else {
					res.InlineData = nil // Ensure nil if size is 0
				}
				// log.Printf("Debug: KRB Inline Resource %d found (Size: %d)", i, res.InlineDataSize)

			default:
				// Unknown format, treat as error or skip? Error is safer.
				return nil, fmt.Errorf("krb read: unknown resource format 0x%02X for resource index %d", res.Format, i)
			}
		}
	}

	// --- Final Check (Optional) ---
	// Verify if the current position matches the TotalSize from the header
	finalPos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		log.Printf("Warning: Could not verify final read position after parsing: %v", err)
	} else {
		// If animations are skipped, finalPos might be less than TotalSize.
		// Only warn if we read *more* than expected.
		if uint32(finalPos) > doc.Header.TotalSize {
			log.Printf("Warning: Read %d bytes, exceeding header TotalSize %d. Potential read error or incorrect TotalSize in KRB.", finalPos, doc.Header.TotalSize)
		} else if doc.Header.AnimationCount == 0 && uint32(finalPos) < doc.Header.TotalSize {
			// If no animations were expected and we still haven't reached TotalSize, something might be wrong
			log.Printf("Warning: Read %d bytes, but header TotalSize is %d (and no animations were declared). Potential unread data or incorrect TotalSize.", finalPos, doc.Header.TotalSize)
		}
	}

	return doc, nil
}
