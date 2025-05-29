// krb/reader.go

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
	headerBuf := make([]byte, HeaderSize) // HeaderSize is now 48 for v0.4
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("krb read: failed to seek to header: %w", err)
	}
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, fmt.Errorf("krb read: failed to read header: %w", err)
	}

	// Parse header fields according to KRB v0.4
	copy(doc.Header.Magic[:], headerBuf[0:4])
	doc.Header.Version = readU16LE(headerBuf[4:6])
	doc.Header.Flags = readU16LE(headerBuf[6:8])
	doc.Header.ElementCount = readU16LE(headerBuf[8:10])
	doc.Header.StyleCount = readU16LE(headerBuf[10:12])
	doc.Header.ComponentDefCount = readU16LE(headerBuf[12:14]) // New in v0.4
	doc.Header.AnimationCount = readU16LE(headerBuf[14:16])
	doc.Header.StringCount = readU16LE(headerBuf[16:18])
	doc.Header.ResourceCount = readU16LE(headerBuf[18:20])
	doc.Header.ElementOffset = readU32LE(headerBuf[20:24])
	doc.Header.StyleOffset = readU32LE(headerBuf[24:28])
	doc.Header.ComponentDefOffset = readU32LE(headerBuf[28:32]) // New in v0.4
	doc.Header.AnimationOffset = readU32LE(headerBuf[32:36])
	doc.Header.StringOffset = readU32LE(headerBuf[36:40])
	doc.Header.ResourceOffset = readU32LE(headerBuf[40:44])
	doc.Header.TotalSize = readU32LE(headerBuf[44:48])

	// Validate Magic Number
	if !bytes.Equal(doc.Header.Magic[:], MagicNumber[:]) {
		return nil, fmt.Errorf("krb read: invalid magic number %v", doc.Header.Magic)
	}

	// Store parsed version
	doc.VersionMajor = uint8(doc.Header.Version & 0x00FF)
	doc.VersionMinor = uint8(doc.Header.Version >> 8)

	// Optional: Version check
	if doc.Header.Version != ExpectedVersion { // ExpectedVersion should be 0.4
		fmt.Printf("Warning: KRB version mismatch. File is %d.%d, reader expects %d.%d. Parsing continues...\n",
			doc.VersionMajor, doc.VersionMinor, SpecVersionMajor, SpecVersionMinor)
	}

	// Basic Offset Sanity Checks (can be expanded)
	if doc.Header.ElementCount > 0 && doc.Header.ElementOffset < HeaderSize {
		return nil, errors.New("krb read: element offset overlaps header")
	}
	if doc.Header.StyleCount > 0 && doc.Header.StyleOffset < HeaderSize {
		return nil, errors.New("krb read: style offset overlaps header")
	}
	if doc.Header.ComponentDefCount > 0 && doc.Header.ComponentDefOffset < HeaderSize {
		return nil, errors.New("krb read: component definition offset overlaps header")
	}

	// --- 2. Read Element Blocks (Main UI Tree) ---
	if doc.Header.ElementCount > 0 {
		doc.Elements = make([]ElementHeader, doc.Header.ElementCount)
		doc.ElementStartOffsets = make([]uint32, doc.Header.ElementCount)
		doc.Properties = make([][]Property, doc.Header.ElementCount)
		doc.CustomProperties = make([][]CustomProperty, doc.Header.ElementCount)
		doc.Events = make([][]EventFileEntry, doc.Header.ElementCount)
		doc.AnimationRefs = make([][]AnimationRef, doc.Header.ElementCount)
		doc.ChildRefs = make([][]ChildRef, doc.Header.ElementCount)

		if _, err := r.Seek(int64(doc.Header.ElementOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to elements offset %d: %w", doc.Header.ElementOffset, err)
		}

		elementHeaderBuf := make([]byte, ElementHeaderSize)
		propertyHeaderBuf := make([]byte, 3)
		customPropertyHeaderBuf := make([]byte, 3)

		for i := uint16(0); i < doc.Header.ElementCount; i++ {
			currentPos, err := r.Seek(0, io.SeekCurrent)
			if err != nil {
				return nil, fmt.Errorf("krb read: failed to get current position for element header %d: %w", i, err)
			}
			if i < uint16(len(doc.ElementStartOffsets)) {
				doc.ElementStartOffsets[i] = uint32(currentPos)
			} else {
				return nil, fmt.Errorf("krb read: element index %d out of bounds for ElementStartOffsets (len %d)", i, len(doc.ElementStartOffsets))
			}

			if _, err := io.ReadFull(r, elementHeaderBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read element header %d at offset %d: %w", i, currentPos, err)
			}

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
				CustomPropCount: elementHeaderBuf[16],
			}
			elemHdr := &doc.Elements[i]

			if elemHdr.PropertyCount > 0 {
				doc.Properties[i] = make([]Property, elemHdr.PropertyCount)
				for j := uint8(0); j < elemHdr.PropertyCount; j++ {
					if _, err := io.ReadFull(r, propertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read property header (%d/%d) for element %d: %w", j+1, elemHdr.PropertyCount, i, err)
					}
					prop := &doc.Properties[i][j]
					prop.ID = PropertyID(propertyHeaderBuf[0])
					prop.ValueType = ValueType(propertyHeaderBuf[1])
					prop.Size = propertyHeaderBuf[2]
					if prop.Size > 0 {
						prop.Value = make([]byte, prop.Size)
						if _, err := io.ReadFull(r, prop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read property value (size %d) for element %d, prop %d: %w", prop.Size, i, j, err)
						}
					}
				}
			}

			if elemHdr.CustomPropCount > 0 {
				doc.CustomProperties[i] = make([]CustomProperty, elemHdr.CustomPropCount)
				for j := uint8(0); j < elemHdr.CustomPropCount; j++ {
					if _, err := io.ReadFull(r, customPropertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read custom property header (%d/%d) for element %d: %w", j+1, elemHdr.CustomPropCount, i, err)
					}
					cprop := &doc.CustomProperties[i][j]
					cprop.KeyIndex = customPropertyHeaderBuf[0]
					cprop.ValueType = ValueType(customPropertyHeaderBuf[1])
					cprop.Size = customPropertyHeaderBuf[2]
					if cprop.Size > 0 {
						cprop.Value = make([]byte, cprop.Size)
						if _, err := io.ReadFull(r, cprop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read custom property value (size %d) for element %d, cprop %d: %w", cprop.Size, i, j, err)
						}
					}
				}
			}

			if elemHdr.EventCount > 0 {
				doc.Events[i] = make([]EventFileEntry, elemHdr.EventCount)
				eventDataSize := int(elemHdr.EventCount) * EventFileEntrySize
				eventBuf := make([]byte, eventDataSize)
				if _, err := io.ReadFull(r, eventBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read events block for element %d: %w", i, err)
				}
				for j := uint8(0); j < elemHdr.EventCount; j++ {
					offset := int(j) * EventFileEntrySize
					doc.Events[i][j] = EventFileEntry{
						EventType:  EventType(eventBuf[offset]),
						CallbackID: eventBuf[offset+1],
					}
				}
			}

			if elemHdr.AnimationCount > 0 {
				doc.AnimationRefs[i] = make([]AnimationRef, elemHdr.AnimationCount)
				animRefDataSize := int(elemHdr.AnimationCount) * AnimationRefSize
				animRefBuf := make([]byte, animRefDataSize)
				if _, err := io.ReadFull(r, animRefBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read anim refs block for element %d: %w", i, err)
				}
				for j := uint8(0); j < elemHdr.AnimationCount; j++ {
					offset := int(j) * AnimationRefSize
					doc.AnimationRefs[i][j] = AnimationRef{
						AnimationIndex: animRefBuf[offset],
						Trigger:        animRefBuf[offset+1],
					}
				}
			}

			if elemHdr.ChildCount > 0 {
				doc.ChildRefs[i] = make([]ChildRef, elemHdr.ChildCount)
				childRefDataSize := int(elemHdr.ChildCount) * ChildRefSize
				childRefBuf := make([]byte, childRefDataSize)
				if _, err := io.ReadFull(r, childRefBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read child refs block for element %d: %w", i, err)
				}
				for j := uint8(0); j < elemHdr.ChildCount; j++ {
					offset := int(j) * ChildRefSize
					doc.ChildRefs[i][j] = ChildRef{
						ChildOffset: readU16LE(childRefBuf[offset : offset+ChildRefSize]),
					}
				}
			}
		}
	}

	// --- 3. Read Style Blocks ---
	if doc.Header.StyleCount > 0 {
		doc.Styles = make([]Style, doc.Header.StyleCount)
		if _, err := r.Seek(int64(doc.Header.StyleOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to styles offset %d: %w", doc.Header.StyleOffset, err)
		}
		styleHeaderBuf := make([]byte, 3)
		propertyHeaderBuf := make([]byte, 3)
		for i := uint16(0); i < doc.Header.StyleCount; i++ {
			if _, err := io.ReadFull(r, styleHeaderBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read style header %d: %w", i, err)
			}
			style := &doc.Styles[i]
			style.ID = styleHeaderBuf[0]
			style.NameIndex = styleHeaderBuf[1]
			style.PropertyCount = styleHeaderBuf[2]
			if style.PropertyCount > 0 {
				style.Properties = make([]Property, style.PropertyCount)
				for j := uint8(0); j < style.PropertyCount; j++ {
					if _, err := io.ReadFull(r, propertyHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read property header for style %d, prop %d: %w", i, j, err)
					}
					prop := &style.Properties[j]
					prop.ID = PropertyID(propertyHeaderBuf[0])
					prop.ValueType = ValueType(propertyHeaderBuf[1])
					prop.Size = propertyHeaderBuf[2]
					if prop.Size > 0 {
						prop.Value = make([]byte, prop.Size)
						if _, err := io.ReadFull(r, prop.Value); err != nil {
							return nil, fmt.Errorf("krb read: failed to read property value for style %d, prop %d: %w", i, j, err)
						}
					}
				}
			}
		}
	}

	// --- 4. Read Component Definition Table (KRB v0.4) ---
	if (doc.Header.Flags&FlagHasComponentDefs) != 0 && doc.Header.ComponentDefCount > 0 {
		doc.ComponentDefinitions = make([]KrbComponentDefinition, doc.Header.ComponentDefCount)
		if _, err := r.Seek(int64(doc.Header.ComponentDefOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to component definitions offset %d: %w", doc.Header.ComponentDefOffset, err)
		}

		compDefEntryHeaderBuf := make([]byte, 2) // NameIndex (1) + PropertyDefCount (1)
		propDefHeaderBuf := make([]byte, 3)      // NameIndex (1) + ValueTypeHint (1) + DefaultValueSize (1)

		// This value needs to be determined by the sum of all component def entry sizes,
		// or by knowing the offset of the next section.
		// This is a simplification assuming component defs are the last structural block before animations/strings.
		var endOfComponentDefsSection uint32
		if doc.Header.AnimationCount > 0 {
			endOfComponentDefsSection = doc.Header.AnimationOffset
		} else if doc.Header.StringCount > 0 {
			endOfComponentDefsSection = doc.Header.StringOffset
		} else if doc.Header.ResourceCount > 0 {
			endOfComponentDefsSection = doc.Header.ResourceOffset
		} else {
			endOfComponentDefsSection = doc.Header.TotalSize
		}


		for i := uint16(0); i < doc.Header.ComponentDefCount; i++ {
			compDef := &doc.ComponentDefinitions[i]

			if _, err := io.ReadFull(r, compDefEntryHeaderBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read component definition entry header %d: %w", i, err)
			}
			compDef.NameIndex = compDefEntryHeaderBuf[0]
			compDef.PropertyDefCount = compDefEntryHeaderBuf[1]

			if compDef.PropertyDefCount > 0 {
				compDef.PropertyDefinitions = make([]KrbPropertyDefinition, compDef.PropertyDefCount)
				for j := uint8(0); j < compDef.PropertyDefCount; j++ {
					propDefEntry := &compDef.PropertyDefinitions[j]
					if _, err := io.ReadFull(r, propDefHeaderBuf); err != nil {
						return nil, fmt.Errorf("krb read: failed to read property definition header for comp_def %d, prop_def %d: %w", i, j, err)
					}
					propDefEntry.NameIndex = propDefHeaderBuf[0]
					propDefEntry.ValueTypeHint = ValueType(propDefHeaderBuf[1])
					propDefEntry.DefaultValueSize = propDefHeaderBuf[2]
					if propDefEntry.DefaultValueSize > 0 {
						propDefEntry.DefaultValueData = make([]byte, propDefEntry.DefaultValueSize)
						if _, err := io.ReadFull(r, propDefEntry.DefaultValueData); err != nil {
							return nil, fmt.Errorf("krb read: failed to read property definition default value for comp_def %d, prop_def %d: %w", i, j, err)
						}
					}
				}
			}

            // Read RootElementTemplateData
            // Calculate the size of the template data for *this specific* component definition.
            // This is the most complex part without explicit size markers per definition.
            currentPosAfterPropDefs, _ := r.Seek(0, io.SeekCurrent)
            var sizeOfTemplateData uint32

            if (i + 1) < doc.Header.ComponentDefCount {
                // This is difficult: we need to know where the *next* component definition starts.
                // The KRB spec does not store individual offsets for component defs, only the table start.
                // A robust reader would need to fully parse the current template to know its size,
                // or the KRB format would need to store the size of each comp def entry.

                // For this implementation, we will make a simplifying assumption:
                // We *cannot* reliably determine the end of the current template if there are more defs,
                // unless we fully parse it, which this basic RootElementTemplateData []byte approach avoids.
                // This part needs significant enhancement for multiple component definitions.
                // For a single component definition, the calculation below is okay.
                // We'll log an error and potentially read garbage if there are multiple.
                if doc.Header.ComponentDefCount > 1 {
                    log.Printf("Warning: KRB reader - Multiple component definitions found. Size calculation for RootElementTemplateData for component definition %d (of %d) might be incorrect. Only the last definition's template size can be reliably determined with current logic.", i, doc.Header.ComponentDefCount)
                    // As a fallback, try to read just one ElementHeader and assume no complex children for now.
                    // This is a HACK for demonstration.
                    // tempElementHeaderForSize := make([]byte, ElementHeaderSize)
                    // if _, err := r.Read(tempElementHeaderForSize); err == nil {
                    //     sizeOfTemplateData = uint32(ElementHeaderSize) // Gross oversimplification
                    //     // We need to seek back because we consumed bytes
                    //     if _, seekErr := r.Seek(currentPosAfterPropDefs, io.SeekStart); seekErr != nil {
                    //         return nil, fmt.Errorf("krb read: failed to seek back after peeking template header for comp_def %d: %w", i, seekErr)
                    //     }
                    // } else {
                    //      return nil, fmt.Errorf("krb read: failed to peek template header for comp_def %d: %w", i, err)
                    // }
                    // A better HACK: if not the last, don't read template data for now to avoid corruption.
                    sizeOfTemplateData = 0 // Avoid reading if not last and logic is complex
                     log.Printf("Warning: Skipping RootElementTemplateData for component definition %d as it's not the last one and size determination is complex.", i)

                } else { // This is the only/last component definition
                     sizeOfTemplateData = endOfComponentDefsSection - uint32(currentPosAfterPropDefs)
                }

            } else { // This is the only/last component definition
                sizeOfTemplateData = endOfComponentDefsSection - uint32(currentPosAfterPropDefs)
            }

            if int64(sizeOfTemplateData) < 0 { // Check for negative size
                 return nil, fmt.Errorf("krb read: calculated negative template size (%d) for component definition %d ('%s'). Offsets/logic error.", int64(sizeOfTemplateData), i, doc.Strings[compDef.NameIndex])
            }

			if sizeOfTemplateData > 0 {
				compDef.RootElementTemplateData = make([]byte, sizeOfTemplateData)
				if _, err := io.ReadFull(r, compDef.RootElementTemplateData); err != nil {
					return nil, fmt.Errorf("krb read: failed to read root element template data (size %d) for comp_def %d: %w", sizeOfTemplateData, i, err)
				}
			} else {
                // This can happen if sizeOfTemplateData was 0 due to the multi-def hack or actual zero size
                compDef.RootElementTemplateData = nil
                if sizeOfTemplateData == 0 && !(doc.Header.ComponentDefCount > 1 && i < doc.Header.ComponentDefCount -1) { // Don't log for the multi-def hack skip
				    log.Printf("Warning: RootElementTemplateData for component definition %d ('%s') is empty (size %d).", i, doc.Strings[compDef.NameIndex], sizeOfTemplateData)
                }
			}
		}
	}

	// --- 5. Read Animation Table ---
	// (Positioned after Component Defs in KRB v0.4 spec, but reading order depends on seeks)
	if doc.Header.AnimationCount > 0 {
		if _, err := r.Seek(int64(doc.Header.AnimationOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to animation offset %d: %w", doc.Header.AnimationOffset, err)
		}
		log.Println("Warning: KRB Animation Table found but parsing is not yet implemented.")
		// Placeholder: Skip reading animation data for now.
		// To correctly skip, you would need to know the total size of the animation section.
		// For now, subsequent seeks will handle positioning.
	}

	// --- 6. Read String Table ---
	if doc.Header.StringCount > 0 {
		doc.Strings = make([]string, doc.Header.StringCount)
		if _, err := r.Seek(int64(doc.Header.StringOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to strings offset %d: %w", doc.Header.StringOffset, err)
		}
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read string table count: %w", err)
		}
		tableCount := readU16LE(countBuf)
		if tableCount != doc.Header.StringCount {
			log.Printf("Warning: KRB String Table count mismatch. Header: %d, Table: %d. Using header count.", doc.Header.StringCount, tableCount)
		}
		lenBuf := make([]byte, 1)
		for i := uint16(0); i < doc.Header.StringCount; i++ {
			if _, err := io.ReadFull(r, lenBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read string length for index %d: %w", i, err)
			}
			length := uint8(lenBuf[0])
			if length > 0 {
				strBuf := make([]byte, length)
				if _, err := io.ReadFull(r, strBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read string data (len %d) for index %d: %w", length, i, err)
				}
				doc.Strings[i] = string(strBuf)
			} else {
				doc.Strings[i] = ""
			}
		}
	}

	// --- 7. Read Resource Table ---
	if doc.Header.ResourceCount > 0 {
		doc.Resources = make([]Resource, doc.Header.ResourceCount)
		if _, err := r.Seek(int64(doc.Header.ResourceOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to resources offset %d: %w", doc.Header.ResourceOffset, err)
		}
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read resource table count: %w", err)
		}
		tableCount := readU16LE(countBuf)
		if tableCount != doc.Header.ResourceCount {
			log.Printf("Warning: KRB Resource Table count mismatch. Header: %d, Table: %d. Using header count.", doc.Header.ResourceCount, tableCount)
		}
		resEntryCommonBuf := make([]byte, 3)
		resExternalDataBuf := make([]byte, 1)
		resInlineSizeBuf := make([]byte, 2)
		for i := uint16(0); i < doc.Header.ResourceCount; i++ {
			res := &doc.Resources[i]
			if _, err := io.ReadFull(r, resEntryCommonBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read resource entry common part %d: %w", i, err)
			}
			res.Type = ResourceType(resEntryCommonBuf[0])
			res.NameIndex = resEntryCommonBuf[1]
			res.Format = ResourceFormat(resEntryCommonBuf[2])
			switch res.Format {
			case ResFormatExternal:
				if _, err := io.ReadFull(r, resExternalDataBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read external resource data index %d: %w", i, err)
				}
				res.DataStringIndex = resExternalDataBuf[0]
			case ResFormatInline:
				if _, err := io.ReadFull(r, resInlineSizeBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read inline resource size %d: %w", i, err)
				}
				res.InlineDataSize = readU16LE(resInlineSizeBuf)
				if res.InlineDataSize > 0 {
					res.InlineData = make([]byte, res.InlineDataSize)
					if _, err := io.ReadFull(r, res.InlineData); err != nil {
						return nil, fmt.Errorf("krb read: failed to read inline resource data (size %d) for index %d: %w", res.InlineDataSize, i, err)
					}
				}
			default:
				return nil, fmt.Errorf("krb read: unknown resource format 0x%02X for resource %d", res.Format, i)
			}
		}
	}

	// --- Final Check ---
	// This check might be misleading if sections like Animations or ComponentDefs with complex internal structures
	// are not fully read/skipped by their exact byte count.
	// For now, let's comment it out to avoid false positives until all sections are robustly read.
	/*
	finalPos, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		log.Printf("Warning: Could not verify final read position after parsing: %v", err)
	} else {
		if uint32(finalPos) != doc.Header.TotalSize {
			log.Printf("Warning: Final read position %d does not match header TotalSize %d. File may be corrupt or reader incomplete.", finalPos, doc.Header.TotalSize)
		}
	}
	*/

	return doc, nil
}