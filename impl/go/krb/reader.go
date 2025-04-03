package krb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// ReadDocument parses a KRB file from the given reader into a Document struct.
// The reader must also implement io.Seeker and io.ReaderAt for random access.
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
		doc.Elements = make([]ElementHeader, doc.Header.ElementCount)
		doc.Properties = make([][]Property, doc.Header.ElementCount)
		doc.CustomProperties = make([][]CustomProperty, doc.Header.ElementCount) // v0.3
		doc.Events = make([][]EventFileEntry, doc.Header.ElementCount)
		doc.AnimationRefs = make([][]AnimationRef, doc.Header.ElementCount)
		doc.ChildRefs = make([][]ChildRef, doc.Header.ElementCount)

		if _, err := r.Seek(int64(doc.Header.ElementOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to elements: %w", err)
		}

		elementHeaderBuf := make([]byte, ElementHeaderSize)
		propertyHeaderBuf := make([]byte, 3) // ID(1)+Type(1)+Size(1)
		customPropertyHeaderBuf := make([]byte, 3) // KeyIdx(1)+Type(1)+Size(1) (v0.3)


		for i := uint16(0); i < doc.Header.ElementCount; i++ {
			// Read Element Header
			if _, err := io.ReadFull(r, elementHeaderBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read element header %d: %w", i, err)
			}
			doc.Elements[i] = ElementHeader{
				Type:           ElementType(elementHeaderBuf[0]),
				ID:             elementHeaderBuf[1],
				PosX:           readU16LE(elementHeaderBuf[2:4]),
				PosY:           readU16LE(elementHeaderBuf[4:6]),
				Width:          readU16LE(elementHeaderBuf[6:8]),
				Height:         readU16LE(elementHeaderBuf[8:10]),
				Layout:         elementHeaderBuf[10],
				StyleID:        elementHeaderBuf[11],
				PropertyCount:  elementHeaderBuf[12],
				ChildCount:     elementHeaderBuf[13],
				EventCount:     elementHeaderBuf[14],
				AnimationCount: elementHeaderBuf[15],
				CustomPropCount: elementHeaderBuf[16], // v0.3
			}
			elemHdr := &doc.Elements[i] // Pointer for convenience

			// Read Standard Properties
			if elemHdr.PropertyCount > 0 {
				doc.Properties[i] = make([]Property, elemHdr.PropertyCount)
				for j := uint8(0); j < elemHdr.PropertyCount; j++ {
					if _, err := io.ReadFull(r, propertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read property header (%d/%d) for element %d: %w", j, elemHdr.PropertyCount, i, err)
					}
					prop := &doc.Properties[i][j]
					prop.ID = PropertyID(propertyHeaderBuf[0])
					prop.ValueType = ValueType(propertyHeaderBuf[1])
					prop.Size = propertyHeaderBuf[2]
					if prop.Size > 0 {
						prop.Value = make([]byte, prop.Size)
						if _, err := io.ReadFull(r, prop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read property value (size %d) (%d/%d) for element %d: %w", prop.Size, j, elemHdr.PropertyCount, i, err)
						}
					}
				}
			}

			// Read Custom Properties (v0.3)
			if elemHdr.CustomPropCount > 0 {
				doc.CustomProperties[i] = make([]CustomProperty, elemHdr.CustomPropCount)
				for j := uint8(0); j < elemHdr.CustomPropCount; j++ {
					 if _, err := io.ReadFull(r, customPropertyHeaderBuf); err != nil {
                         return nil, fmt.Errorf("krb read: failed to read custom property header (%d/%d) for element %d: %w", j, elemHdr.CustomPropCount, i, err)
                     }
					 cprop := &doc.CustomProperties[i][j]
					 cprop.KeyIndex = customPropertyHeaderBuf[0]
					 cprop.ValueType = ValueType(customPropertyHeaderBuf[1])
					 cprop.Size = customPropertyHeaderBuf[2]
					 if cprop.Size > 0 {
						 cprop.Value = make([]byte, cprop.Size)
						 if _, err := io.ReadFull(r, cprop.Value); err != nil {
							 return nil, fmt.Errorf("krb read: failed to read custom property value (size %d) (%d/%d) for element %d: %w", cprop.Size, j, elemHdr.CustomPropCount, i, err)
						 }
					 }
				}
			}


			// Read Events
			if elemHdr.EventCount > 0 {
				doc.Events[i] = make([]EventFileEntry, elemHdr.EventCount)
				eventDataSize := int(elemHdr.EventCount) * EventFileEntrySize
				eventBuf := make([]byte, eventDataSize)
				if _, err := io.ReadFull(r, eventBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read events (count %d) for element %d: %w", elemHdr.EventCount, i, err)
				}
				for j := uint8(0); j < elemHdr.EventCount; j++ {
					offset := int(j) * EventFileEntrySize
					doc.Events[i][j] = EventFileEntry{
						EventType: EventType(eventBuf[offset]),
						CallbackID: eventBuf[offset+1],
					}
				}
			}

			// Read Animation References
			if elemHdr.AnimationCount > 0 {
				doc.AnimationRefs[i] = make([]AnimationRef, elemHdr.AnimationCount)
				animRefDataSize := int(elemHdr.AnimationCount) * AnimationRefSize
				animRefBuf := make([]byte, animRefDataSize)
				if _, err := io.ReadFull(r, animRefBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read anim refs (count %d) for element %d: %w", elemHdr.AnimationCount, i, err)
				}
				for j := uint8(0); j < elemHdr.AnimationCount; j++ {
					offset := int(j) * AnimationRefSize
					doc.AnimationRefs[i][j] = AnimationRef{
						AnimationIndex: animRefBuf[offset],
						Trigger:        animRefBuf[offset+1],
					}
				}
			}

			// Read Child References
			if elemHdr.ChildCount > 0 {
				doc.ChildRefs[i] = make([]ChildRef, elemHdr.ChildCount)
				childRefDataSize := int(elemHdr.ChildCount) * ChildRefSize
				childRefBuf := make([]byte, childRefDataSize)
				if _, err := io.ReadFull(r, childRefBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read child refs (count %d) for element %d: %w", elemHdr.ChildCount, i, err)
				}
				for j := uint8(0); j < elemHdr.ChildCount; j++ {
					offset := int(j) * ChildRefSize
					doc.ChildRefs[i][j] = ChildRef{
						ChildOffset: readU16LE(childRefBuf[offset : offset+2]),
					}
				}
			}
		}
	}

	// --- 3. Read Style Blocks ---
	if doc.Header.StyleCount > 0 {
		doc.Styles = make([]Style, doc.Header.StyleCount)
		if _, err := r.Seek(int64(doc.Header.StyleOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to styles: %w", err)
		}

		styleHeaderBuf := make([]byte, 3) // ID(1)+NameIdx(1)+PropCount(1)
		propertyHeaderBuf := make([]byte, 3) // Shared buffer

		for i := uint16(0); i < doc.Header.StyleCount; i++ {
			if _, err := io.ReadFull(r, styleHeaderBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read style header %d: %w", i, err)
			}
			style := &doc.Styles[i]
			style.ID = styleHeaderBuf[0]            // 1-based
			style.NameIndex = styleHeaderBuf[1]     // 0-based
			style.PropertyCount = styleHeaderBuf[2]

			if style.PropertyCount > 0 {
				style.Properties = make([]Property, style.PropertyCount)
				for j := uint8(0); j < style.PropertyCount; j++ {
					if _, err := io.ReadFull(r, propertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read property header (%d/%d) for style %d: %w", j, style.PropertyCount, i, err)
					}
					prop := &style.Properties[j]
					prop.ID = PropertyID(propertyHeaderBuf[0])
					prop.ValueType = ValueType(propertyHeaderBuf[1])
					prop.Size = propertyHeaderBuf[2]
					if prop.Size > 0 {
						prop.Value = make([]byte, prop.Size)
						if _, err := io.ReadFull(r, prop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read property value (size %d) (%d/%d) for style %d: %w", prop.Size, j, style.PropertyCount, i, err)
						}
					}
				}
			}
		}
	}

	// --- 4. Read Animation Table (TODO) ---
	if doc.Header.AnimationCount > 0 {
		// Seek to doc.Header.AnimationOffset
		// Read Animation entries based on their type (Transition/Keyframe)
		// Store in doc.Animations (define appropriate structs)
		// For now, just log a warning if animations exist but aren't read
		fmt.Println("Warning: KRB Animation Table found but parsing is not yet implemented.")
	}


	// --- 5. Read String Table ---
	if doc.Header.StringCount > 0 {
		doc.Strings = make([]string, doc.Header.StringCount)
		if _, err := r.Seek(int64(doc.Header.StringOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to strings: %w", err)
		}

		// Read table header (count)
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read string table count: %w", err)
		}
		tableCount := readU16LE(countBuf)
		if tableCount != doc.Header.StringCount {
			fmt.Printf("Warning: KRB String Table count mismatch. Header: %d, Table: %d\n", doc.Header.StringCount, tableCount)
			// Potentially problematic, decide whether to error or proceed with header count
		}

		lenBuf := make([]byte, 1)
		for i := uint16(0); i < doc.Header.StringCount; i++ {
			// Read length prefix
			if _, err := io.ReadFull(r, lenBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read string length for index %d: %w", i, err)
			}
			length := uint8(lenBuf[0])

			if length > 0 {
				strBuf := make([]byte, length)
				if _, err := io.ReadFull(r, strBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read string data (len %d) for index %d: %w", length, i, err)
				}
				doc.Strings[i] = string(strBuf) // Assumes UTF-8
			} else {
				doc.Strings[i] = "" // Empty string
			}
		}
	}

	// --- 6. Read Resource Table ---
	if doc.Header.ResourceCount > 0 {
		doc.Resources = make([]Resource, doc.Header.ResourceCount)
		if _, err := r.Seek(int64(doc.Header.ResourceOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to resources: %w", err)
		}

		// Read table header (count)
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read resource table count: %w", err)
		}
		tableCount := readU16LE(countBuf)
		if tableCount != doc.Header.ResourceCount {
			fmt.Printf("Warning: KRB Resource Table count mismatch. Header: %d, Table: %d\n", doc.Header.ResourceCount, tableCount)
		}

		// Buffer for the common part of the entry (Type, NameIdx, Format) = 3 bytes
		resEntryCommonBuf := make([]byte, 3)
		resExternalDataBuf := make([]byte, 1) // For RES_FORMAT_EXTERNAL Data Index
        resInlineSizeBuf := make([]byte, 2) // For RES_FORMAT_INLINE Size

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
                if _, err := io.ReadFull(r, resInlineSizeBuf); err != nil {
                    return nil, fmt.Errorf("krb read: failed to read inline resource size for index %d: %w", i, err)
                }
                res.InlineDataSize = readU16LE(resInlineSizeBuf)
                res.DataStringIndex = 0 // Ensure 0 for inline

                if res.InlineDataSize > 0 {
                    res.InlineData = make([]byte, res.InlineDataSize)
                    if _, err := io.ReadFull(r, res.InlineData); err != nil {
                        return nil, fmt.Errorf("krb read: failed to read inline resource data (size %d) for index %d: %w", res.InlineDataSize, i, err)
                    }
                } else {
                    res.InlineData = nil
                }
                fmt.Printf("Warning: KRB Inline Resource %d found (Size: %d), data loaded but renderer support may vary.\n", i, res.InlineDataSize)


			default:
				return nil, fmt.Errorf("krb read: unknown resource format 0x%02X for resource index %d", res.Format, i)
			}
		}
	}

	// Final check: Did we read roughly the expected total size?
	currentPos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		fmt.Printf("Warning: Could not verify final read position: %v\n", err)
	} else if uint32(currentPos) < doc.Header.TotalSize {
        // This isn't necessarily an error if animations weren't read, etc.
		// fmt.Printf("Warning: Read %d bytes, but header TotalSize is %d. Potential unread data.\n", currentPos, doc.Header.TotalSize)
	} else if uint32(currentPos) > doc.Header.TotalSize {
        // This is more likely an error
        fmt.Printf("Warning: Read %d bytes, exceeding header TotalSize %d. Potential read error.\n", currentPos, doc.Header.TotalSize)
    }


	return doc, nil
}

// --- Potentially add helper functions here to interpret property values ---
// e.g., func GetColorValue(prop *Property, flags uint16) (r,g,b,a uint8, ok bool)
// e.g., func GetStringValue(prop *Property, doc *Document) (string, bool)
// e.g., func GetFixedPointValue(prop *Property) (float32, bool)