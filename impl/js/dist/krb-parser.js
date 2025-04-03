// impl/js/krb-parser.js
import * as C from './krb-constants.js';

// Simple wrapper for DataView with offset tracking
class BufferReader {
    constructor(arrayBuffer) {
        this.buffer = arrayBuffer;
        this.view = new DataView(arrayBuffer);
        this.offset = 0;
        this.littleEndian = true; // KRB spec is little-endian
    }

    seek(newOffset) {
        if (newOffset < 0 || newOffset > this.buffer.byteLength) {
             throw new Error(`Seek offset ${newOffset} out of bounds (0-${this.buffer.byteLength})`);
        }
        this.offset = newOffset;
    }

    skip(bytes) {
        const newOffset = this.offset + bytes;
         if (newOffset < 0 || newOffset > this.buffer.byteLength) {
             throw new Error(`Skip(${bytes}) from offset ${this.offset} goes out of bounds (0-${this.buffer.byteLength})`);
         }
        this.offset = newOffset;
    }

    tell() {
        return this.offset;
    }

    readBytes(numBytes) {
        if (numBytes < 0) throw new Error(`Invalid number of bytes to read: ${numBytes}`);
        if (this.offset + numBytes > this.buffer.byteLength) {
            throw new Error(`Read past end of buffer: trying to read ${numBytes} bytes at offset ${this.offset} (buffer size: ${this.buffer.byteLength})`);
        }
        // Use slice().buffer to get ArrayBuffer, needed for DataView on rawValue later
        const slice = this.buffer.slice(this.offset, this.offset + numBytes);
        this.offset += numBytes;
        return slice;
    }

    readU8() {
        if (this.offset >= this.buffer.byteLength) throw new Error(`Read past end of buffer (u8) at offset ${this.offset}`);
        const value = this.view.getUint8(this.offset);
        this.offset += 1;
        return value;
    }

    readU16() {
        if (this.offset + 1 >= this.buffer.byteLength) throw new Error(`Read past end of buffer (u16) at offset ${this.offset}`);
        const value = this.view.getUint16(this.offset, this.littleEndian);
        this.offset += 2;
        return value;
    }

     readU32() {
        if (this.offset + 3 >= this.buffer.byteLength) throw new Error(`Read past end of buffer (u32) at offset ${this.offset}`);
        const value = this.view.getUint32(this.offset, this.littleEndian);
        this.offset += 4;
        return value;
    }

    readString(length) {
        if (length === 0) return "";
        const bytes = new Uint8Array(this.readBytes(length)); // readBytes handles length/bounds checks
        return new TextDecoder('utf-8').decode(bytes); // KRB uses UTF-8
    }

    readLengthPrefixedString() {
        const length = this.readU8();
        return this.readString(length);
    }
}

export class KrbParser {
    constructor() {
        this.doc = null; // Will hold the parsed KrbDocument structure
    }

    parse(arrayBuffer) {
        console.log(`Parsing KRB file (${arrayBuffer.byteLength} bytes)`);
        const reader = new BufferReader(arrayBuffer);
        this.doc = {
            header: null,
            versionMajor: 0,
            versionMinor: 0,
            // Added customProperties to element structure for v0.3
            elements: [], // Holds { header: KrbElementHeader, properties: [], customProperties: [], events: [], animationRefs: [], _fileOffset }
            styles: [],   // Holds { id, nameIndex, properties: [] }
            strings: [],
            resources: [], // Holds { type, nameIndex, format, dataStringIndex/inlineData }
            animations: [] // TODO
        };

        try {
            this.parseHeader(reader);
            // Validate version *after* parsing header
            if (this.doc.versionMajor !== C.KRB_SPEC_VERSION_MAJOR || this.doc.versionMinor !== C.KRB_SPEC_VERSION_MINOR) {
                 console.warn(`KRB version mismatch. File is ${this.doc.versionMajor}.${this.doc.versionMinor}, Parser expects ${C.KRB_SPEC_VERSION_MAJOR}.${C.KRB_SPEC_VERSION_MINOR}. Parsing may fail or produce incorrect results.`);
                 // Decide if parsing should continue or stop based on version compatibility needs
                 // For now, we'll attempt to continue.
            }

            // Sections can be parsed in any order using offsets, but string table is often needed first
            if (this.doc.header.stringCount > 0) this.parseStringTable(reader);
            if (this.doc.header.resourceCount > 0 && (this.doc.header.flags & C.FLAG_HAS_RESOURCES)) this.parseResourceTable(reader);
            if (this.doc.header.styleCount > 0 && (this.doc.header.flags & C.FLAG_HAS_STYLES)) this.parseStyles(reader);
            if (this.doc.header.elementCount > 0) this.parseElements(reader);
            // TODO: Parse Animations if FLAG_HAS_ANIMATIONS
            if (this.doc.header.animationCount > 0 && (this.doc.header.flags & C.FLAG_HAS_ANIMATIONS)) this.parseAnimations(reader); // Added placeholder

        } catch (error) {
            console.error("KRB Parsing Error:", error);
            // Provide more context if available
            if (error.message.includes("offset")) {
                 console.error(`   Occurred near offset ${reader?.tell()} / ${arrayBuffer?.byteLength}`);
            }
            alert(`KRB Parsing Error: ${error.message}`); // Simple feedback
            return null; // Indicate failure
        }

        console.log("KRB Parsing Complete.", this.doc);
        return this.doc;
    }

    parseHeader(reader) {
        reader.seek(0); // Ensure starting at 0
        const expectedHeaderSize = 42; // v0.3 header size
        if (reader.buffer.byteLength < expectedHeaderSize) {
            throw new Error(`Invalid KRB file: Size (${reader.buffer.byteLength}) is less than header size (${expectedHeaderSize}).`);
        }

        const header = {};
        const magicBytes = new Uint8Array(reader.readBytes(4));
        header.magic = new TextDecoder('ascii').decode(magicBytes);
        if (header.magic !== "KRB1") {
            throw new Error(`Invalid magic number. Expected 'KRB1', got '${header.magic}'`);
        }

        header.versionRaw = reader.readU16(); // Offset 4
        header.flags = reader.readU16(); // Offset 6
        header.elementCount = reader.readU16(); // Offset 8
        header.styleCount = reader.readU16(); // Offset 10
        header.animationCount = reader.readU16(); // Offset 12
        header.stringCount = reader.readU16(); // Offset 14
        header.resourceCount = reader.readU16(); // Offset 16
        header.elementOffset = reader.readU32(); // Offset 18
        header.styleOffset = reader.readU32(); // Offset 22
        header.animationOffset = reader.readU32(); // Offset 26
        header.stringOffset = reader.readU32(); // Offset 30
        header.resourceOffset = reader.readU32(); // Offset 34
        header.totalSize = reader.readU32(); // Offset 38

        this.doc.header = header;
        this.doc.versionMajor = header.versionRaw & 0x00FF;
        this.doc.versionMinor = header.versionRaw >> 8;

        // Basic sanity check on total size vs actual buffer size
         if (header.totalSize !== reader.buffer.byteLength) {
            console.warn(`Header totalSize (${header.totalSize}) does not match actual buffer size (${reader.buffer.byteLength}). File might be truncated or header incorrect.`);
        }
        // Check if offsets are within file bounds (basic check)
        const maxOffset = Math.max(header.elementOffset, header.styleOffset, header.animationOffset, header.stringOffset, header.resourceOffset);
        if (maxOffset > header.totalSize || maxOffset > reader.buffer.byteLength) {
             console.warn(`One or more section offsets point beyond the reported total size or actual buffer size. Offsets:`, { elem: header.elementOffset, style: header.styleOffset, anim: header.animationOffset, str: header.stringOffset, res: header.resourceOffset }, `Total/Actual: ${header.totalSize}/${reader.buffer.byteLength}`);
        }


        console.log("Parsed Header (v0.3):", header);
    }

    parseStringTable(reader) {
        if (!this.doc.header.stringOffset || this.doc.header.stringCount === 0) return;
        console.log(`Parsing String Table (Count: ${this.doc.header.stringCount}) at offset ${this.doc.header.stringOffset}`);
        reader.seek(this.doc.header.stringOffset);

        const tableCount = reader.readU16(); // Read count from table itself (2 bytes)
        if (tableCount !== this.doc.header.stringCount) {
            console.warn(`Header string count (${this.doc.header.stringCount}) != table count (${tableCount}). Using header count.`);
        }
        const countToRead = this.doc.header.stringCount; // Use header count reliably

        this.doc.strings = [];
        for (let i = 0; i < countToRead; i++) {
            try {
                // Spec: [Length (1 byte)] [UTF-8 Bytes (Variable)]
                const str = reader.readLengthPrefixedString();
                this.doc.strings.push(str);
            } catch (e) {
                 throw new Error(`Error reading string index ${i}: ${e.message}`);
            }
        }
        // console.log("Parsed Strings:", this.doc.strings);
    }

     parseResourceTable(reader) {
        if (!this.doc.header.resourceOffset || this.doc.header.resourceCount === 0 || !(this.doc.header.flags & C.FLAG_HAS_RESOURCES)) return;
        console.log(`Parsing Resource Table (Count: ${this.doc.header.resourceCount}) at offset ${this.doc.header.resourceOffset}`);
        reader.seek(this.doc.header.resourceOffset);

        const tableCount = reader.readU16(); // Header (2 bytes)
        if (tableCount !== this.doc.header.resourceCount) {
             console.warn(`Header resource count (${this.doc.header.resourceCount}) != table count (${tableCount}). Using header count.`);
        }
         const countToRead = this.doc.header.resourceCount;

        this.doc.resources = [];
        for (let i = 0; i < countToRead; i++) {
             try {
                // Spec: [Type (1)] [Name Index (1)] [Format (1)] [Data (Variable)]
                const resource = {};
                resource.type = reader.readU8();
                resource.nameIndex = reader.readU8(); // 0-based index into string table
                resource.format = reader.readU8();

                if (resource.format === C.RES_FORMAT_EXTERNAL) {
                    // Data is 1 byte: string table index for path/URL
                    resource.dataStringIndex = reader.readU8();
                    resource.inlineData = null;
                } else if (resource.format === C.RES_FORMAT_INLINE) {
                    // Data is [Size (2 bytes)] [Raw Bytes (Variable)]
                    const inlineSize = reader.readU16();
                    resource.inlineData = reader.readBytes(inlineSize); // readBytes checks bounds
                    resource.dataStringIndex = null;
                    // console.log(`Parsed Inline resource ${i} (Type ${resource.type}), Size ${inlineSize} bytes`);
                } else {
                    throw new Error(`Unknown resource format 0x${resource.format.toString(16)} for resource index ${i}`);
                }
                this.doc.resources.push(resource);
            } catch(e) {
                throw new Error(`Error reading resource index ${i}: ${e.message}`);
            }
        }
        // console.log("Parsed Resources:", this.doc.resources);
    }

     parseStyles(reader) {
        if (!this.doc.header.styleOffset || this.doc.header.styleCount === 0 || !(this.doc.header.flags & C.FLAG_HAS_STYLES)) return;
        console.log(`Parsing Styles (Count: ${this.doc.header.styleCount}) at offset ${this.doc.header.styleOffset}`);
        reader.seek(this.doc.header.styleOffset);

        const countToRead = this.doc.header.styleCount;
        this.doc.styles = [];
        for (let i = 0; i < countToRead; i++) {
             let currentStyleId = -1; // For error reporting
            try {
                 // Spec: [ID (1, 1-based)] [Name Index (1)] [Property Count (1)] [Properties...]
                const style = {};
                style.id = reader.readU8(); // 1-based ID from file
                currentStyleId = style.id;
                style.nameIndex = reader.readU8(); // 0-based string index
                style.propertyCount = reader.readU8();
                style.properties = [];

                for (let j = 0; j < style.propertyCount; j++) {
                    // Use parseStandardProperty helper
                    style.properties.push(this.parseStandardProperty(reader, `Style ${style.id} Prop ${j}`));
                }
                this.doc.styles.push(style);
            } catch (e) {
                 throw new Error(`Error reading style index ${i} (Style ID ${currentStyleId}): ${e.message}`);
            }
        }
        // console.log("Parsed Styles:", this.doc.styles);
    }

     parseElements(reader) {
        if (!this.doc.header.elementOffset || this.doc.header.elementCount === 0) return;
        console.log(`Parsing Elements (Count: ${this.doc.header.elementCount}) at offset ${this.doc.header.elementOffset}`);
        reader.seek(this.doc.header.elementOffset);

        this.doc.elements = [];
        for (let i = 0; i < this.doc.header.elementCount; i++) {
            const elementStartOffset = reader.tell();
            try {
                // --- Element Header (17 bytes in v0.3) ---
                const header = {};
                // Read all header fields sequentially according to v0.3 spec
                header.type = reader.readU8();           // Offset 0
                header.idIndex = reader.readU8();        // Offset 1 (0-based string index, 0=no ID)
                header.posX = reader.readU16();          // Offset 2
                header.posY = reader.readU16();          // Offset 4
                header.width = reader.readU16();         // Offset 6
                header.height = reader.readU16();        // Offset 8
                header.layout = reader.readU8();         // Offset 10
                header.styleId = reader.readU8();        // Offset 11 (1-based style ID, 0=no style)
                header.propertyCount = reader.readU8();  // Offset 12 (Standard properties)
                header.childCount = reader.readU8();     // Offset 13
                header.eventCount = reader.readU8();     // Offset 14
                header.animationCount = reader.readU8(); // Offset 15
                // v0.3 Change: Added Custom Prop Count
                header.customPropCount = reader.readU8(); // Offset 16

                // --- Sanity Check: App Element ---
                 if (i === 0 && (this.doc.header.flags & C.FLAG_HAS_APP) && header.type !== C.ELEM_TYPE_APP) {
                    console.warn(`FLAG_HAS_APP is set, but first element (index 0) has type 0x${header.type.toString(16)}, not ELEM_TYPE_APP (0x00).`);
                 }

                const elementData = {
                    header: header,
                    properties: [],       // Standard properties
                    customProperties: [], // Added v0.3
                    events: [],
                    animationRefs: [],
                    _fileOffset: elementStartOffset // Store for debugging/linking if needed
                };

                // --- Read Standard Properties ---
                // console.log(`   Elem ${i} @${elementStartOffset}: Reading ${header.propertyCount} std props from offset ${reader.tell()}`);
                for (let j = 0; j < header.propertyCount; j++) {
                    elementData.properties.push(this.parseStandardProperty(reader, `Elem ${i} StdProp ${j}`));
                }

                // --- Read Custom Properties (New in v0.3) ---
                 // console.log(`   Elem ${i}: Reading ${header.customPropCount} custom props from offset ${reader.tell()}`);
                for (let j = 0; j < header.customPropCount; j++) {
                     elementData.customProperties.push(this.parseCustomProperty(reader, `Elem ${i} CustProp ${j}`));
                }

                // --- Read Events ---
                 // console.log(`   Elem ${i}: Reading ${header.eventCount} events from offset ${reader.tell()}`);
                for (let j = 0; j < header.eventCount; j++) {
                     // Spec: [Event Type (1)] [Callback ID Index (1)]
                    const eventData = {
                        eventType: reader.readU8(),
                        callbackIdIndex: reader.readU8() // 0-based string index
                    };
                    elementData.events.push(eventData);
                }

                // --- Read Animation References ---
                 // console.log(`   Elem ${i}: Reading ${header.animationCount} anim refs from offset ${reader.tell()}`);
                for (let j = 0; j < header.animationCount; j++) {
                      // Spec: [Animation Index (1, 0-based)] [Trigger Type (1)]
                     const animRef = {
                         animationIndex: reader.readU8(), // 0-based index into Animation Table
                         trigger: reader.readU8()
                     };
                    elementData.animationRefs.push(animRef);
                }

                // --- Read (and skip) Child References ---
                 const childRefBytes = header.childCount * 2; // Each ref is 2 bytes (offset)
                 // console.log(`   Elem ${i}: Skipping ${header.childCount} child refs (${childRefBytes} bytes) from offset ${reader.tell()}`);
                 if (childRefBytes > 0) {
                    reader.skip(childRefBytes);
                 }
                 // console.log(`   Elem ${i} End Offset: ${reader.tell()}`);


                this.doc.elements.push(elementData);

            } catch(e) {
                 // Include element index and offset in error message
                 throw new Error(`Error reading element index ${i} starting at offset ${elementStartOffset}: ${e.message}`);
            }
        }
        // console.log("Parsed Elements:", this.doc.elements);
    }

    // Helper to parse a single STANDARD property definition
    parseStandardProperty(reader, debugContext = "") {
        const startOffset = reader.tell();
        const prop = {
            propertyId: 0,
            valueType: 0,
            size: 0,
            rawValue: null,
            value: null
        };

        try {
            // Spec: [Property ID (1)] [Value Type (1)] [Size (1)] [Value (Variable)]
            prop.propertyId = reader.readU8();
            prop.valueType = reader.readU8();
            prop.size = reader.readU8();

            // console.log(`      [${debugContext}] Read StdProp @${startOffset}: ID=0x${prop.propertyId.toString(16)}, Type=0x${prop.valueType.toString(16)}, Size=${prop.size}. Reading value from ${reader.tell()}`);

            if (prop.size > 0) {
                // Read the raw bytes for the value first
                prop.rawValue = reader.readBytes(prop.size); // Handles bounds check

                // Parse simple types immediately for easier use later
                 const valueView = new DataView(prop.rawValue);
                 switch (prop.valueType) {
                    case C.VAL_TYPE_BYTE:
                    case C.VAL_TYPE_ENUM:
                    case C.VAL_TYPE_STRING: // Store index
                    case C.VAL_TYPE_RESOURCE: // Store index
                         if (prop.size >= 1) prop.value = valueView.getUint8(0);
                         else console.warn(`[${debugContext}] Size mismatch for Byte/Enum/Index: Expected >=1, got ${prop.size}`);
                         break;
                    case C.VAL_TYPE_SHORT:
                         if (prop.size >= 2) prop.value = valueView.getInt16(0, reader.littleEndian);
                         else console.warn(`[${debugContext}] Size mismatch for Short: Expected >=2, got ${prop.size}`);
                         break;
                    case C.VAL_TYPE_COLOR: // Store raw ArrayBuffer, renderer decides format based on flags/size
                         prop.value = prop.rawValue; // Keep buffer
                         break;
                     case C.VAL_TYPE_PERCENTAGE: // Store raw fixed-point U16
                         if (prop.size >= 2) prop.value = valueView.getUint16(0, reader.littleEndian);
                          else console.warn(`[${debugContext}] Size mismatch for Percentage: Expected >=2, got ${prop.size}`);
                         break;
                    case C.VAL_TYPE_EDGEINSETS: // Store raw ArrayBuffer (expect 4 or 8 bytes?)
                    case C.VAL_TYPE_RECT:       // Store raw ArrayBuffer (expect 8 bytes?)
                    case C.VAL_TYPE_VECTOR:     // Store raw ArrayBuffer (expect 4 bytes?)
                         prop.value = prop.rawValue; // Keep buffer
                         break;
                     case C.VAL_TYPE_CUSTOM: // Often used with PROP_ID_CUSTOM_DATA_BLOB
                     default:
                        // Keep only rawValue for complex/custom types or unknown
                        prop.value = prop.rawValue; // Store buffer for potential later use
                        if (prop.valueType !== C.VAL_TYPE_CUSTOM) {
                             console.warn(`[${debugContext}] Unhandled standard VAL_TYPE: 0x${prop.valueType.toString(16)}`);
                        }
                        break;
                 }
            } else {
                 // Size is 0, value is effectively null or default
                 prop.value = null;
                 prop.rawValue = null;
            }
        } catch (e) {
             throw new Error(`Error reading value for std property (ID 0x${prop.propertyId.toString(16)}, Type 0x${prop.valueType.toString(16)}, Size ${prop.size}) starting @${startOffset} (${debugContext}): ${e.message}`);
        }
        return prop;
    }

    // Helper to parse a single CUSTOM property definition (New in v0.3)
    parseCustomProperty(reader, debugContext = "") {
         const startOffset = reader.tell();
         const prop = {
             keyIndex: 0,
             valueType: 0,
             size: 0,
             rawValue: null,
             value: null
         };

        try {
            // Spec: [Key Index (1)] [Value Type (1)] [Value Size (1)] [Value (Variable)]
            prop.keyIndex = reader.readU8(); // String table index for key
            prop.valueType = reader.readU8();
            prop.size = reader.readU8();

             // console.log(`      [${debugContext}] Read CustomProp @${startOffset}: KeyIdx=${prop.keyIndex}, Type=0x${prop.valueType.toString(16)}, Size=${prop.size}. Reading value from ${reader.tell()}`);

            if (prop.size > 0) {
                prop.rawValue = reader.readBytes(prop.size);
                // Optionally parse simple types like in parseStandardProperty
                const valueView = new DataView(prop.rawValue);
                 switch (prop.valueType) {
                    case C.VAL_TYPE_BYTE:
                    case C.VAL_TYPE_ENUM:
                    case C.VAL_TYPE_STRING:
                    case C.VAL_TYPE_RESOURCE:
                         if (prop.size >= 1) prop.value = valueView.getUint8(0);
                         break;
                    case C.VAL_TYPE_SHORT:
                         if (prop.size >= 2) prop.value = valueView.getInt16(0, reader.littleEndian);
                         break;
                     case C.VAL_TYPE_PERCENTAGE:
                         if (prop.size >= 2) prop.value = valueView.getUint16(0, reader.littleEndian);
                         break;
                    // Keep raw ArrayBuffer for COLOR, EDGEINSETS, RECT, VECTOR, CUSTOM etc.
                    default:
                        prop.value = prop.rawValue;
                        break;
                 }
            } else {
                 prop.value = null;
                 prop.rawValue = null;
            }
        } catch (e) {
             throw new Error(`Error reading value for custom property (KeyIdx ${prop.keyIndex}, Type 0x${prop.valueType.toString(16)}, Size ${prop.size}) starting @${startOffset} (${debugContext}): ${e.message}`);
        }
        return prop;
    }

    // Placeholder for Animation Table Parsing
    parseAnimations(reader) {
        if (!this.doc.header.animationOffset || this.doc.header.animationCount === 0 || !(this.doc.header.flags & C.FLAG_HAS_ANIMATIONS)) return;
        console.warn(`Animation Table parsing (Count: ${this.doc.header.animationCount}) at offset ${this.doc.header.animationOffset} is not implemented.`);
        // Seek to the offset just to show it's acknowledged
        try {
            reader.seek(this.doc.header.animationOffset);
            // TODO: Implement actual parsing based on spec for Transition/Keyframe
            // For now, just log and maybe skip based on a fixed size guess or total file size?
            // Skipping is dangerous without knowing the sizes.
            this.doc.animations = []; // Indicate empty/unparsed
        } catch (e) {
             console.error(`Error seeking to animation table: ${e.message}`);
        }
    }
}