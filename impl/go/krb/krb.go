package krb

import "encoding/binary"

// KRB Spec Version expected by this reader
const (
	SpecVersionMajor = 0
	SpecVersionMinor = 3
	ExpectedVersion  = uint16(SpecVersionMinor<<8 | SpecVersionMajor)
)

// Magic Number
var MagicNumber = [4]byte{'K', 'R', 'B', '1'}

// Header Flags
const (
	FlagHasStyles     uint16 = 1 << 0
	FlagHasAnimations uint16 = 1 << 1
	FlagHasResources  uint16 = 1 << 2
	FlagCompressed    uint16 = 1 << 3 // Not currently used
	FlagFixedPoint    uint16 = 1 << 4
	FlagExtendedColor uint16 = 1 << 5
	FlagHasApp        uint16 = 1 << 6
	// Bits 7-15 Reserved
)

// Element Types (iota can simplify sequential constants)
type ElementType uint8

const (
	ElemTypeApp         ElementType = 0x00
	ElemTypeContainer   ElementType = 0x01
	ElemTypeText        ElementType = 0x02
	ElemTypeImage       ElementType = 0x03
	ElemTypeCanvas      ElementType = 0x04
	// 0x05 - 0x0F Reserved
	ElemTypeButton      ElementType = 0x10
	ElemTypeInput       ElementType = 0x11
	// 0x12 - 0x1F Reserved
	ElemTypeList        ElementType = 0x20
	ElemTypeGrid        ElementType = 0x21
	ElemTypeScrollable  ElementType = 0x22
	// 0x23 - 0x2F Reserved
	ElemTypeVideo       ElementType = 0x30
	// 0x31 - 0xFF Custom/Specialized
	ElemTypeCustomStart ElementType = 0x31
)

// Property IDs
type PropertyID uint8

const (
	PropIDInvalid        PropertyID = 0x00
	PropIDBgColor        PropertyID = 0x01
	PropIDFgColor        PropertyID = 0x02
	PropIDBorderColor    PropertyID = 0x03
	PropIDBorderWidth    PropertyID = 0x04
	PropIDBorderRadius   PropertyID = 0x05
	PropIDPadding        PropertyID = 0x06
	PropIDMargin         PropertyID = 0x07
	PropIDTextContent    PropertyID = 0x08
	PropIDFontSize       PropertyID = 0x09
	PropIDFontWeight     PropertyID = 0x0A
	PropIDTextAlignment  PropertyID = 0x0B
	PropIDImageSource    PropertyID = 0x0C
	PropIDOpacity        PropertyID = 0x0D
	PropIDZIndex         PropertyID = 0x0E
	PropIDVisibility     PropertyID = 0x0F
	PropIDGap            PropertyID = 0x10
	PropIDMinWidth       PropertyID = 0x11
	PropIDMinHeight      PropertyID = 0x12
	PropIDMaxWidth       PropertyID = 0x13
	PropIDMaxHeight      PropertyID = 0x14
	PropIDAspectRatio    PropertyID = 0x15
	PropIDTransform      PropertyID = 0x16
	PropIDShadow         PropertyID = 0x17
	PropIDOverflow       PropertyID = 0x18
	PropIDCustomDataBlob PropertyID = 0x19 // Renamed for clarity
	PropIDLayoutFlags    PropertyID = 0x1A
	// App-Specific (0x20-0x28)
	PropIDWindowWidth  PropertyID = 0x20
	PropIDWindowHeight PropertyID = 0x21
	PropIDWindowTitle  PropertyID = 0x22
	PropIDResizable    PropertyID = 0x23
	PropIDKeepAspect   PropertyID = 0x24
	PropIDScaleFactor  PropertyID = 0x25
	PropIDIcon         PropertyID = 0x26
	PropIDVersion      PropertyID = 0x27
	PropIDAuthor       PropertyID = 0x28
	// 0x29 - 0xFF Reserved
)

// Value Types
type ValueType uint8

const (
	ValTypeNone       ValueType = 0x00
	ValTypeByte       ValueType = 0x01
	ValTypeShort      ValueType = 0x02
	ValTypeColor      ValueType = 0x03
	ValTypeString     ValueType = 0x04 // Index
	ValTypeResource   ValueType = 0x05 // Index
	ValTypePercentage ValueType = 0x06 // 8.8 fixed point (uint16)
	ValTypeRect       ValueType = 0x07 // 4 shorts (x,y,w,h) -> 8 bytes
	ValTypeEdgeInsets ValueType = 0x08 // 4 bytes/shorts (t,r,b,l) - Check spec/impl for exact size
	ValTypeEnum       ValueType = 0x09
	ValTypeVector     ValueType = 0x0A // 2 shorts (x,y) -> 4 bytes
	ValTypeCustom     ValueType = 0x0B
	// 0x0C - 0xFF Reserved
)

// Event Types
type EventType uint8

const (
	EventTypeNone      EventType = 0x00
	EventTypeClick     EventType = 0x01
	EventTypePress     EventType = 0x02
	EventTypeRelease   EventType = 0x03
	EventTypeLongPress EventType = 0x04
	EventTypeHover     EventType = 0x05
	EventTypeFocus     EventType = 0x06
	EventTypeBlur      EventType = 0x07
	EventTypeChange    EventType = 0x08
	EventTypeSubmit    EventType = 0x09
	EventTypeCustom    EventType = 0x0A
	// 0x0B - 0xFF Reserved
)

// Layout Byte Bits
const (
	LayoutDirectionMask uint8 = 0x03 // Bits 0-1
	LayoutAlignmentMask uint8 = 0x0C // Bits 2-3
	LayoutWrapBit       uint8 = 1 << 4
	LayoutGrowBit       uint8 = 1 << 5
	LayoutAbsoluteBit   uint8 = 1 << 6
	// Bit 7 Reserved
)

// Layout Direction Values (extracted from mask)
const (
	LayoutDirRow          uint8 = 0x00
	LayoutDirColumn       uint8 = 0x01
	LayoutDirRowReverse   uint8 = 0x02
	LayoutDirColumnReverse uint8 = 0x03
)

// Layout Alignment Values (extracted from mask, shifted)
const (
	LayoutAlignStart        uint8 = 0x00 // (00 << 2) >> 2
	LayoutAlignCenter      uint8 = 0x01 // (01 << 2) >> 2
	LayoutAlignEnd          uint8 = 0x02 // (10 << 2) >> 2
	LayoutAlignSpaceBetween uint8 = 0x03 // (11 << 2) >> 2
)

// Resource Types
type ResourceType uint8

const (
	ResTypeNone   ResourceType = 0x00
	ResTypeImage  ResourceType = 0x01
	ResTypeFont   ResourceType = 0x02
	ResTypeSound  ResourceType = 0x03
	ResTypeVideo  ResourceType = 0x04
	ResTypeCustom ResourceType = 0x05
	// 0x06 - 0xFF Reserved
)

// Resource Formats
type ResourceFormat uint8

const (
	ResFormatExternal ResourceFormat = 0x00
	ResFormatInline   ResourceFormat = 0x01
)

// --- Data Structures ---
// Note: Go structs don't map 1:1 with C packed structs for direct reading.
// We'll read fields individually using encoding/binary.

// Header represents the KRB file header.
type Header struct {
	Magic          [4]byte
	Version        uint16 // Raw version from file
	Flags          uint16
	ElementCount   uint16
	StyleCount     uint16
	AnimationCount uint16
	StringCount    uint16
	ResourceCount  uint16
	ElementOffset  uint32
	StyleOffset    uint32
	AnimationOffset uint32
	StringOffset   uint32
	ResourceOffset uint32
	TotalSize      uint32
}

const HeaderSize = 42 // Size of KRB v0.3 header

// ElementHeader represents the header for each element block.
type ElementHeader struct {
	Type           ElementType
	ID             uint8 // String table index (0-based), 0 if no ID
	PosX           uint16
	PosY           uint16
	Width          uint16
	Height         uint16
	Layout         uint8 // Effective layout flags
	StyleID        uint8 // 1-based index into Style Blocks, 0 for none
	PropertyCount  uint8 // Standard properties
	ChildCount     uint8
	EventCount     uint8
	AnimationCount uint8
	CustomPropCount uint8 // v0.3
}

const ElementHeaderSize = 17 // Size of KRB v0.3 element header

// Property represents a standard property entry.
type Property struct {
	ID        PropertyID
	ValueType ValueType
	Size      uint8 // Size of Value in bytes
	Value     []byte // Raw byte value, interpretation depends on ValueType
}

// CustomProperty represents a custom key-value property entry (v0.3).
type CustomProperty struct {
	KeyIndex  uint8     // String table index for key name
	ValueType ValueType
	Size      uint8   // Size of Value in bytes
	Value     []byte  // Raw byte value
}

// EventFileEntry represents an event entry as stored in the file.
type EventFileEntry struct {
	EventType EventType
	CallbackID uint8 // String table index for callback name
}

const EventFileEntrySize = 2

// AnimationRef represents an animation reference in the element block.
type AnimationRef struct {
    AnimationIndex uint8 // 0-based index into Animation Table
    Trigger        uint8 // TRIGGER_TYPE_*
}
const AnimationRefSize = 2

// ChildRef represents a child reference in the element block.
type ChildRef struct {
    ChildOffset uint16 // Byte offset from parent's header start to child header
}
const ChildRefSize = 2


// Style represents a reusable style block.
type Style struct {
	ID            uint8 // 1-based ID from file
	NameIndex     uint8 // 0-based string index
	PropertyCount uint8
	Properties    []Property // Parsed properties
}

// Resource represents an entry in the resource table.
type Resource struct {
	Type            ResourceType
	NameIndex       uint8 // 0-based string index for name/path
	Format          ResourceFormat
	DataStringIndex uint8  // Only if format is External (0-based string index)
	InlineDataSize  uint16 // Only if format is Inline
	InlineData      []byte // Only if format is Inline (TODO: Implement reading)
}

// Document holds the entire parsed KRB data in memory.
type Document struct {
    Header           Header
    VersionMajor     uint8
    VersionMinor     uint8
    Elements         []ElementHeader
    ElementStartOffsets []uint32 // <-- ADD THIS FIELD: Start byte offset of each ElementHeader in the file
    Properties       [][]Property
    CustomProperties [][]CustomProperty
    Events           [][]EventFileEntry
    Styles           []Style
    Animations       []byte
    Strings          []string
    Resources        []Resource
    ChildRefs        [][]ChildRef
    AnimationRefs    [][]AnimationRef
}

// Helper to get layout direction
func (eh *ElementHeader) LayoutDirection() uint8 {
	return eh.Layout & LayoutDirectionMask
}

// Helper to get layout alignment
func (eh *ElementHeader) LayoutAlignment() uint8 {
	return (eh.Layout & LayoutAlignmentMask) >> 2
}

// Helper to check layout wrap
func (eh *ElementHeader) LayoutWrap() bool {
	return (eh.Layout & LayoutWrapBit) != 0
}

// Helper to check layout grow
func (eh *ElementHeader) LayoutGrow() bool {
	return (eh.Layout & LayoutGrowBit) != 0
}

// Helper to check layout absolute positioning
func (eh *ElementHeader) LayoutAbsolute() bool {
	return (eh.Layout & LayoutAbsoluteBit) != 0
}


// Helper to read little-endian uint16
func readU16LE(data []byte) uint16 {
	if len(data) < 2 { return 0 }
	return binary.LittleEndian.Uint16(data)
}

// Helper to read little-endian uint32
func readU32LE(data []byte) uint32 {
	if len(data) < 4 { return 0 }
	return binary.LittleEndian.Uint32(data)
}