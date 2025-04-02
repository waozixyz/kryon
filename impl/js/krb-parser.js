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
        this.offset = newOffset;
    }

    skip(bytes) {
        this.offset += bytes;
    }

    tell() {
        return this.offset;
    }

    readBytes(numBytes) {
        if (this.offset + numBytes > this.buffer.byteLength) {
            throw new Error(`Read past end of buffer: trying to read ${numBytes} bytes at offset ${this.offset} (buffer size: ${this.buffer.byteLength})`);
        }
        const slice = this.buffer.slice(this.offset, this.offset + numBytes);
        this.offset += numBytes;
        return slice;
    }

    readU8() {
        if (this.offset >= this.buffer.byteLength) throw new Error("Read past end of buffer (u8)");
        const value = this.view.getUint8(this.offset);
        this.offset += 1;
        return value;
    }

    readU16() {
        if (this.offset + 1 >= this.buffer.byteLength) throw new Error("Read past end of buffer (u16)");
        const value = this.view.getUint16(this.offset, this.littleEndian);
        this.offset += 2;
        return value;
    }

     readU32() {
        if (this.offset + 3 >= this.buffer.byteLength) throw new Error("Read past end of buffer (u32)");
        const value = this.view.getUint32(this.offset, this.littleEndian);
        this.offset += 4;
        return value;
    }

    readString(length) {
        const bytes = new Uint8Array(this.readBytes(length));
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
            elements: [], // Holds { header: KrbElementHeader, properties: [], events: [], childOffsets: [], animationRefs: [] }
            styles: [],   // Holds { id, nameIndex, properties: [] }
            strings: [],
            resources: [], // Holds { type, nameIndex, format, dataStringIndex }
            animations: [] // TODO
        };

        try {
            this.parseHeader(reader);
            // Sections can be parsed in any order using offsets, but string table is often needed first
            if (this.doc.header.stringCount > 0) this.parseStringTable(reader);
            if (this.doc.header.resourceCount > 0 && (this.doc.header.flags & C.FLAG_HAS_RESOURCES)) this.parseResourceTable(reader);
            if (this.doc.header.styleCount > 0 && (this.doc.header.flags & C.FLAG_HAS_STYLES)) this.parseStyles(reader);
            if (this.doc.header.elementCount > 0) this.parseElements(reader);
            // TODO: Parse Animations if FLAG_HAS_ANIMATIONS
        } catch (error) {
            console.error("KRB Parsing Error:", error);
            alert(`KRB Parsing Error: ${error.message}`); // Simple feedback
            return null; // Indicate failure
        }

        console.log("KRB Parsing Complete.", this.doc);
        return this.doc;
    }

    parseHeader(reader) {
        reader.seek(0);
        const header = {};
        const magicBytes = new Uint8Array(reader.readBytes(4));
        header.magic = new TextDecoder('ascii').decode(magicBytes);
        header.versionRaw = reader.readU16();
        header.flags = reader.readU16();
        header.elementCount = reader.readU16();
        header.styleCount = reader.readU16();
        header.animationCount = reader.readU16();
        header.stringCount = reader.readU16();
        header.resourceCount = reader.readU16();
        header.elementOffset = reader.readU32();
        header.styleOffset = reader.readU32();
        header.animationOffset = reader.readU32();
        header.stringOffset = reader.readU32();
        header.resourceOffset = reader.readU32();
        header.totalSize = reader.readU32();

        if (header.magic !== "KRB1") {
            throw new Error(`Invalid magic number. Expected 'KRB1', got '${header.magic}'`);
        }

        this.doc.header = header;
        this.doc.versionMajor = header.versionRaw & 0x00FF;
        this.doc.versionMinor = header.versionRaw >> 8;

        if (this.doc.versionMajor !== C.KRB_SPEC_VERSION_MAJOR || this.doc.versionMinor !== C.KRB_SPEC_VERSION_MINOR) {
             console.warn(`KRB version mismatch. File is ${this.doc.versionMajor}.${this.doc.versionMinor}, Parser expects ${C.KRB_SPEC_VERSION_MAJOR}.${C.KRB_SPEC_VERSION_MINOR}.`);
        }
         // Basic sanity check on total size
         if (header.totalSize !== reader.buffer.byteLength) {
            console.warn(`Header totalSize (${header.totalSize}) does not match actual buffer size (${reader.buffer.byteLength}).`);
        }
        console.log("Parsed Header:", header);
    }

    parseStringTable(reader) {
        if (!this.doc.header.stringOffset || this.doc.header.stringCount === 0) return;
        console.log(`Parsing String Table (Count: ${this.doc.header.stringCount}) at offset ${this.doc.header.stringOffset}`);
        reader.seek(this.doc.header.stringOffset);
        const tableCount = reader.readU16(); // Read count from table itself
        if (tableCount !== this.doc.header.stringCount) {
            console.warn(`Header string count (${this.doc.header.stringCount}) != table count (${tableCount}). Using header count.`);
        }

        this.doc.strings = []; // Reset just in case
        for (let i = 0; i < this.doc.header.stringCount; i++) {
            try {
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
        const tableCount = reader.readU16();
        if (tableCount !== this.doc.header.resourceCount) {
             console.warn(`Header resource count (${this.doc.header.resourceCount}) != table count (${tableCount}). Using header count.`);
        }

        this.doc.resources = [];
        for (let i = 0; i < this.doc.header.resourceCount; i++) {
             try {
                const resource = {};
                resource.type = reader.readU8();
                resource.nameIndex = reader.readU8(); // 0-based index into string table
                resource.format = reader.readU8();
                // resource.data = null; // Placeholder

                if (resource.format === C.RES_FORMAT_EXTERNAL) {
                    // Data is 1 byte: string table index for path/URL
                    resource.dataStringIndex = reader.readU8();
                } else if (resource.format === C.RES_FORMAT_INLINE) {
                    // Data is [Size (2 bytes)] [Raw Bytes (Variable)]
                    const inlineSize = reader.readU16();
                     console.warn(`Inline resource format (Res ${i}, Size ${inlineSize}) not fully implemented in parser. Skipping data.`);
                    reader.skip(inlineSize); // Skip the data for now
                    resource.inlineData = null; // Indicate not loaded
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

        this.doc.styles = [];
        for (let i = 0; i < this.doc.header.styleCount; i++) {
            try {
                const style = {};
                style.id = reader.readU8(); // 1-based ID from file
                style.nameIndex = reader.readU8(); // 0-based string index
                style.propertyCount = reader.readU8();
                style.properties = [];

                for (let j = 0; j < style.propertyCount; j++) {
                    style.properties.push(this.parseProperty(reader, `Style ${i} Prop ${j}`));
                }
                this.doc.styles.push(style);
            } catch (e) {
                 throw new Error(`Error reading style index ${i}: ${e.message}`);
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
            try {
                const elementStartOffset = reader.tell();
                const header = {};
                header.type = reader.readU8();
                header.idIndex = reader.readU8(); // 0-based string index
                header.posX = reader.readU16();
                header.posY = reader.readU16();
                header.width = reader.readU16();
                header.height = reader.readU16();
                header.layout = reader.readU8();
                header.styleId = reader.readU8(); // 1-based style ID
                header.propertyCount = reader.readU8();
                header.childCount = reader.readU8();
                header.eventCount = reader.readU8();
                header.animationCount = reader.readU8();

                // --- Sanity Check: App Element ---
                 if (i === 0 && (this.doc.header.flags & C.FLAG_HAS_APP) && header.type !== C.ELEM_TYPE_APP) {
                    console.warn(`FLAG_HAS_APP is set, but first element (index 0) has type 0x${header.type.toString(16)}, not ELEM_TYPE_APP (0x00).`);
                 }

                const elementData = {
                    header: header,
                    properties: [],
                    events: [],
                    animationRefs: [],
                    // We don't read child offsets here, assuming sequential layout for rendering pass
                    // childOffsets: []
                    _fileOffset: elementStartOffset // Store for debugging/linking if needed
                };

                // Read Properties
                for (let j = 0; j < header.propertyCount; j++) {
                    elementData.properties.push(this.parseProperty(reader, `Elem ${i} Prop ${j}`));
                }

                // Read Events
                for (let j = 0; j < header.eventCount; j++) {
                    const eventData = {
                        eventType: reader.readU8(),
                        callbackIdIndex: reader.readU8() // 0-based string index
                    };
                    elementData.events.push(eventData);
                }

                // Read Animation References
                for (let j = 0; j < header.animationCount; j++) {
                     const animRef = {
                         animationIndex: reader.readU8(), // 0-based index into Animation Table
                         trigger: reader.readU8()
                     };
                    elementData.animationRefs.push(animRef);
                     // TODO: Use these refs when animations are implemented
                }

                // Skip Child References (2 bytes each: offset)
                // The renderer will implicitly handle children assuming they follow sequentially
                const childRefBytes = header.childCount * 2;
                reader.skip(childRefBytes);

                this.doc.elements.push(elementData);

            } catch(e) {
                 throw new Error(`Error reading element index ${i}: ${e.message}`);
            }
        }
        // console.log("Parsed Elements:", this.doc.elements);
    }

    // Helper to parse a single property definition
    parseProperty(reader, debugContext = "") {
        const prop = {};
        prop.propertyId = reader.readU8();
        prop.valueType = reader.readU8();
        prop.size = reader.readU8();
        prop.rawValue = null; // Store raw bytes initially
        prop.value = null; // Parsed value (done during render or post-parse)

        if (prop.size > 0) {
            try {
                // Read the raw bytes for the value
                prop.rawValue = reader.readBytes(prop.size);

                // Optionally, parse simple types immediately
                 const valueView = new DataView(prop.rawValue);
                 switch (prop.valueType) {
                    case C.VAL_TYPE_BYTE:
                    case C.VAL_TYPE_ENUM:
                    case C.VAL_TYPE_STRING: // Store index
                    case C.VAL_TYPE_RESOURCE: // Store index
                         prop.value = valueView.getUint8(0);
                         break;
                    case C.VAL_TYPE_SHORT:
                         prop.value = valueView.getInt16(0, reader.littleEndian); // Assuming signed shorts might be useful
                         break;
                    case C.VAL_TYPE_COLOR: // Store raw bytes, renderer decides format
                         prop.value = prop.rawValue;
                         break;
                     case C.VAL_TYPE_PERCENTAGE: // Store raw fixed-point U16
                         prop.value = valueView.getUint16(0, reader.littleEndian);
                         break;
                    case C.VAL_TYPE_EDGEINSETS: // Store raw bytes
                         prop.value = prop.rawValue;
                         break;
                     // Add other types (Rect, Vector) if needed immediately
                     default:
                        // Keep rawValue for complex/custom types
                        break;
                 }

            } catch (e) {
                 throw new Error(`Error reading value for property ID 0x${prop.propertyId.toString(16)} (${debugContext}): ${e.message}`);
            }
        }
        return prop;
    }
}