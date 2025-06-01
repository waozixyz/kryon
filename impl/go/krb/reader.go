// krb/reader.go

package krb

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
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

	// Parse header fields according to KRB v0.4
	copy(doc.Header.Magic[:], headerBuf[0:4])
	doc.Header.Version = ReadU16LE(headerBuf[4:6])
	doc.Header.Flags = ReadU16LE(headerBuf[6:8])
	doc.Header.ElementCount = ReadU16LE(headerBuf[8:10])
	doc.Header.StyleCount = ReadU16LE(headerBuf[10:12])
	doc.Header.ComponentDefCount = ReadU16LE(headerBuf[12:14])
	doc.Header.AnimationCount = ReadU16LE(headerBuf[14:16])
	doc.Header.StringCount = ReadU16LE(headerBuf[16:18])
	doc.Header.ResourceCount = ReadU16LE(headerBuf[18:20])
	doc.Header.ElementOffset = ReadU32LE(headerBuf[20:24])
	doc.Header.StyleOffset = ReadU32LE(headerBuf[24:28])
	doc.Header.ComponentDefOffset = ReadU32LE(headerBuf[28:32])
	doc.Header.AnimationOffset = ReadU32LE(headerBuf[32:36])
	doc.Header.StringOffset = ReadU32LE(headerBuf[36:40])
	doc.Header.ResourceOffset = ReadU32LE(headerBuf[40:44])
	doc.Header.TotalSize = ReadU32LE(headerBuf[44:48])

	if !bytes.Equal(doc.Header.Magic[:], MagicNumber[:]) {
		return nil, fmt.Errorf("krb read: invalid magic number %v", doc.Header.Magic)
	}
	doc.VersionMajor = uint8(doc.Header.Version & 0x00FF)
	doc.VersionMinor = uint8(doc.Header.Version >> 8)
	if doc.Header.Version != ExpectedVersion {
		log.Printf("Warning: KRB version mismatch. File is %d.%d, reader expects %d.%d. Parsing continues...",
			doc.VersionMajor, doc.VersionMinor, SpecVersionMajor, SpecVersionMinor)
	}

	// Basic Offset Sanity Checks
	if doc.Header.ElementCount > 0 && doc.Header.ElementOffset < HeaderSize {
		return nil, errors.New("krb read: element offset overlaps header")
	}
	if doc.Header.StyleCount > 0 && doc.Header.StyleOffset < HeaderSize {
		return nil, errors.New("krb read: style offset overlaps header")
	}
	if doc.Header.ComponentDefCount > 0 && (doc.Header.Flags&FlagHasComponentDefs) != 0 && doc.Header.ComponentDefOffset < HeaderSize {
		return nil, errors.New("krb read: component definition offset overlaps header")
	}
	if doc.Header.AnimationCount > 0 && doc.Header.AnimationOffset < HeaderSize {
		return nil, errors.New("krb read: animation offset overlaps header")
	}
	if doc.Header.StringCount > 0 && doc.Header.StringOffset < HeaderSize {
		return nil, errors.New("krb read: string offset overlaps header")
	}
	if doc.Header.ResourceCount > 0 && doc.Header.ResourceOffset < HeaderSize {
		return nil, errors.New("krb read: resource offset overlaps header")
	}


	// --- Eagerly Read String Table ---
	// It's often needed by other sections (like ComponentDef names) for meaningful logging or early validation.
	if doc.Header.StringCount > 0 {
		doc.Strings = make([]string, doc.Header.StringCount)
		if _, err := r.Seek(int64(doc.Header.StringOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to strings offset %d: %w", doc.Header.StringOffset, err)
		}
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read string table count: %w", err)
		}
		tableCount := ReadU16LE(countBuf)
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
				PosX:            ReadU16LE(elementHeaderBuf[2:4]),
				PosY:            ReadU16LE(elementHeaderBuf[4:6]),
				Width:           ReadU16LE(elementHeaderBuf[6:8]),
				Height:          ReadU16LE(elementHeaderBuf[8:10]),
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
						ChildOffset: ReadU16LE(childRefBuf[offset : offset+ChildRefSize]),
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
			
			// The stream 'r' is now positioned at the start of RootElementTemplateData for component 'i'.
			// We use calculateAndReadKrbElementTree to parse this self-contained tree.
			// This function will read from 'r', determine the tree's size, and return the bytes.
			// 'r' will be advanced past the template data by this call.
			var compDefNameForLog string
			if int(compDef.NameIndex) < len(doc.Strings) {
				compDefNameForLog = doc.Strings[compDef.NameIndex]
			} else {
				compDefNameForLog = fmt.Sprintf("UnknownName(Index:%d)", compDef.NameIndex)
			}
			
			// log.Printf("Debug: Reading RootElementTemplateData for CompDef %d ('%s')", i, compDefNameForLog)
			_, templateDataBytes, err := calculateAndReadKrbElementTree(r)
			if err != nil {
				return nil, fmt.Errorf("krb read: comp_def '%s' (index %d), error processing RootElementTemplateData: %w", compDefNameForLog, i, err)
			}
			compDef.RootElementTemplateData = templateDataBytes
			// log.Printf("Debug: Read RootElementTemplateData for CompDef %d ('%s'), size %d bytes. Reader now at offset %d.", i, compDefNameForLog, templateSizeBytes, currentOffset)
		}
	}


	// --- 5. Read Animation Table ---
	if doc.Header.AnimationCount > 0 {
		if _, err := r.Seek(int64(doc.Header.AnimationOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to animation offset %d: %w", doc.Header.AnimationOffset, err)
		}
		
		var endOfAnimationSection uint32
		// Determine end of animation section by finding the start of the next known section,
		// or defaulting to TotalSize if it's the last one.
		// This requires careful ordering if sections are optional.
		nextSectionOffset := doc.Header.TotalSize // Default to end of file
		if doc.Header.StringCount > 0 && doc.Header.StringOffset > doc.Header.AnimationOffset && doc.Header.StringOffset < nextSectionOffset {
			nextSectionOffset = doc.Header.StringOffset
		}
		if doc.Header.ResourceCount > 0 && doc.Header.ResourceOffset > doc.Header.AnimationOffset && doc.Header.ResourceOffset < nextSectionOffset {
			nextSectionOffset = doc.Header.ResourceOffset
		}
		// If ComponentDefs are after Animations (unlikely by spec order, but for robustness):
        if doc.Header.ComponentDefCount > 0 && (doc.Header.Flags&FlagHasComponentDefs) != 0 && doc.Header.ComponentDefOffset > doc.Header.AnimationOffset && doc.Header.ComponentDefOffset < nextSectionOffset {
             nextSectionOffset = doc.Header.ComponentDefOffset
        }


		endOfAnimationSection = nextSectionOffset
		animationSectionSize := endOfAnimationSection - doc.Header.AnimationOffset

		if int64(animationSectionSize) < 0 { // Check for negative size
			return nil, fmt.Errorf("krb read: calculated negative animation section size (%d). Offsets/logic error.", int64(animationSectionSize))
		}

		if animationSectionSize > 0 {
			doc.Animations = make([]byte, animationSectionSize) // Store as raw blob for now
			if _, err := io.ReadFull(r, doc.Animations); err != nil {
				return nil, fmt.Errorf("krb read: failed to read animation table (size %d): %w", animationSectionSize, err)
			}
			log.Printf("Warning: KRB Animation Table found (%d animations, %d bytes) but detailed parsing is not yet implemented. Read as raw blob.", doc.Header.AnimationCount, animationSectionSize)
		} else if animationSectionSize == 0 && doc.Header.AnimationCount > 0 {
			log.Printf("Warning: KRB Animation Table header indicates %d animations, but calculated section size is 0.", doc.Header.AnimationCount)
		}
	}

	// --- 6. Read String Table (if not already read) ---
	// String table might have been read earlier if ComponentDefs needed it.
	if doc.Strings == nil && doc.Header.StringCount > 0 {
		doc.Strings = make([]string, doc.Header.StringCount)
		if _, err := r.Seek(int64(doc.Header.StringOffset), io.SeekStart); err != nil {
			return nil, fmt.Errorf("krb read: failed to seek to strings offset %d (fallback): %w", doc.Header.StringOffset, err)
		}
		countBuf := make([]byte, 2)
		if _, err := io.ReadFull(r, countBuf); err != nil {
			return nil, fmt.Errorf("krb read: failed to read string table count (fallback): %w", err)
		}
		tableCount := ReadU16LE(countBuf)
		if tableCount != doc.Header.StringCount {
			log.Printf("Warning: KRB String Table count mismatch (fallback). Header: %d, Table: %d. Using header count.", doc.Header.StringCount, tableCount)
		}
		lenBuf := make([]byte, 1)
		for i := uint16(0); i < doc.Header.StringCount; i++ {
			if _, err := io.ReadFull(r, lenBuf); err != nil {
				return nil, fmt.Errorf("krb read: failed to read string length for index %d (fallback): %w", i, err)
			}
			length := uint8(lenBuf[0])
			if length > 0 {
				strBuf := make([]byte, length)
				if _, err := io.ReadFull(r, strBuf); err != nil {
					return nil, fmt.Errorf("krb read: failed to read string data (len %d) for index %d (fallback): %w", length, i, err)
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
		tableCount := ReadU16LE(countBuf)
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
				res.InlineDataSize = ReadU16LE(resInlineSizeBuf)
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
	return doc, nil
}


// calculateAndReadKrbElementTree reads a self-contained KRB element tree from the stream.
// It determines the total size of this tree (root element + all its descendants within the tree)
// by parsing its structure, then reads the entire tree into a byte slice.
// The input stream 'r' is expected to be positioned at the start of the root element's header.
// After successful execution, 'r' will be positioned immediately after the parsed element tree.
func calculateAndReadKrbElementTree(r io.ReadSeeker) (totalTreeSize uint32, treeData []byte, err error) {
	startOffsetOfTree, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, nil, fmt.Errorf("calculateAndReadKrbElementTree: failed to get start offset: %w", err)
	}

	// This map stores the calculated size of each element block encountered within this tree.
	// Key: offset of the element's header *relative to startOffsetOfTree*.
	// Value: size of that element *block* (header, props, events, anims, childrefs).
	elementBlockSizes := make(map[uint32]uint32)

	// Queue of element offsets (relative to startOffsetOfTree) to process.
	// These offsets point to the headers of elements within the tree.
	processingQueue := []uint32{0} // Start with the root element at relative offset 0.
	
	// Tracks the maximum relative offset reached by the end of any processed element block.
	// This will determine the total size of the serialized tree.
	maxRelativeExtent := uint32(0)

	// Temp buffers
	headerBuf := make([]byte, ElementHeaderSize)
	propHeaderBuf := make([]byte, 3)
	childRefBufItem := make([]byte, ChildRefSize)

	for len(processingQueue) > 0 {
		currentElementRelativeOffset := processingQueue[0]
		processingQueue = processingQueue[1:]

		// If we've already calculated the size for this element block, skip.
		if _, visited := elementBlockSizes[currentElementRelativeOffset]; visited {
			continue
		}

		// Seek to the start of the current element's header within the tree.
		if _, err := r.Seek(startOffsetOfTree+int64(currentElementRelativeOffset), io.SeekStart); err != nil {
			return 0, nil, fmt.Errorf("calculateAndReadKrbElementTree: seek to element at rel_offset %d failed: %w", currentElementRelativeOffset, err)
		}

		var currentElementBlockSize uint32 = 0

		// Read Element Header
		bytesRead, err := io.ReadFull(r, headerBuf)
		if err != nil {
			// If this is the first element (root) and we get EOF, the tree is empty/invalid.
			if currentElementRelativeOffset == 0 && (err == io.EOF || err == io.ErrUnexpectedEOF) {
				return 0, nil, fmt.Errorf("calculateAndReadKrbElementTree: tree is empty or header read failed for root: %w", err)
			}
			// If it's not the root, an EOF here might mean a child offset pointed beyond valid data.
			return 0, nil, fmt.Errorf("calculateAndReadKrbElementTree: reading header at rel_offset %d failed: %w", currentElementRelativeOffset, err)
		}
		currentElementBlockSize += uint32(bytesRead)

		var elemHdr ElementHeader // Only need counts for size calculation
		elemHdr.PropertyCount = headerBuf[12]
		elemHdr.ChildCount = headerBuf[13]
		elemHdr.EventCount = headerBuf[14]
		elemHdr.AnimationCount = headerBuf[15]
		elemHdr.CustomPropCount = headerBuf[16]

		// Size of Standard Properties
		for j := uint8(0); j < elemHdr.PropertyCount; j++ {
			if _, err := io.ReadFull(r, propHeaderBuf); err != nil { return 0, nil, fmt.Errorf("calc: std_prop header read failed: %w", err) }
			currentElementBlockSize += 3
			propDataSize := propHeaderBuf[2]
			if propDataSize > 0 {
				if _, err := r.Seek(int64(propDataSize), io.SeekCurrent); err != nil { return 0, nil, fmt.Errorf("calc: std_prop seek data failed: %w", err) }
				currentElementBlockSize += uint32(propDataSize)
			}
		}
		// Size of Custom Properties
		for j := uint8(0); j < elemHdr.CustomPropCount; j++ {
			if _, err := io.ReadFull(r, propHeaderBuf); err != nil { return 0, nil, fmt.Errorf("calc: custom_prop header read failed: %w", err) }
			currentElementBlockSize += 3
			propDataSize := propHeaderBuf[2]
			if propDataSize > 0 {
				if _, err := r.Seek(int64(propDataSize), io.SeekCurrent); err != nil { return 0, nil, fmt.Errorf("calc: custom_prop seek data failed: %w", err) }
				currentElementBlockSize += uint32(propDataSize)
			}
		}
		// Size of Events
		eventsBlockSize := uint32(elemHdr.EventCount) * uint32(EventFileEntrySize)
		if _, err := r.Seek(int64(eventsBlockSize), io.SeekCurrent); err != nil { return 0, nil, fmt.Errorf("calc: events seek failed: %w", err) }
		currentElementBlockSize += eventsBlockSize
		// Size of Animation Refs
		animRefsBlockSize := uint32(elemHdr.AnimationCount) * uint32(AnimationRefSize)
		if _, err := r.Seek(int64(animRefsBlockSize), io.SeekCurrent); err != nil { return 0, nil, fmt.Errorf("calc: anim_refs seek failed: %w", err) }
		currentElementBlockSize += animRefsBlockSize

		// Add children from ChildRefs to the queue and include ChildRef block size
		if elemHdr.ChildCount > 0 {
			for j := uint8(0); j < elemHdr.ChildCount; j++ {
				if _, err := io.ReadFull(r, childRefBufItem); err != nil { return 0, nil, fmt.Errorf("calc: child_ref read failed: %w", err) }
				currentElementBlockSize += uint32(ChildRefSize) // Size of the ChildRef entry itself
				
				childRelOffsetFromParentHeader := ReadU16LE(childRefBufItem)
				// The child's offset relative to the *start of the entire tree*
				childActualTreeRelativeOffset := currentElementRelativeOffset + uint32(childRelOffsetFromParentHeader)
				
				// Add to queue only if not already processed (or scheduled)
				// This check isn't strictly necessary with the `elementBlockSizes` map check,
				// but good for clarity if queue could have duplicates from complex structures.
				if _, visited := elementBlockSizes[childActualTreeRelativeOffset]; !visited {
					// Ensure not already in queue to prevent redundant processing if graph-like refs (though KRB is tree-like)
					inQueue := false
					for _, off := range processingQueue {
						if off == childActualTreeRelativeOffset {
							inQueue = true
							break
						}
					}
					if !inQueue {
						processingQueue = append(processingQueue, childActualTreeRelativeOffset)
					}
				}
			}
		}
		
		elementBlockSizes[currentElementRelativeOffset] = currentElementBlockSize
		currentElementEndRelativeOffset := currentElementRelativeOffset + currentElementBlockSize
		if currentElementEndRelativeOffset > maxRelativeExtent {
			maxRelativeExtent = currentElementEndRelativeOffset
		}
	}

	totalTreeSize = maxRelativeExtent

	// Now that total size is known, read the data block.
	treeData = make([]byte, totalTreeSize)
	if _, err := r.Seek(startOffsetOfTree, io.SeekStart); err != nil {
		return 0, nil, fmt.Errorf("calculateAndReadKrbElementTree: final seek to re-read tree data failed: %w", err)
	}
	if _, err := io.ReadFull(r, treeData); err != nil {
		return 0, nil, fmt.Errorf("calculateAndReadKrbElementTree: final read of tree data (size %d) failed: %w", totalTreeSize, err)
	}
	
	// Critical: Ensure the main reader 'r' is positioned *after* this tree.
	if _, err := r.Seek(startOffsetOfTree+int64(totalTreeSize), io.SeekStart); err != nil {
		return 0, nil, fmt.Errorf("calculateAndReadKrbElementTree: final seek to position reader after tree failed: %w", err)
	}

	return totalTreeSize, treeData, nil
}
