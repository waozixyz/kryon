// krb/types.go

package krb

// KRB Spec Version expected by this reader
const (
	SpecVersionMajor = 0
	SpecVersionMinor = 4
	ExpectedVersion  = uint16(SpecVersionMinor<<8 | SpecVersionMajor)
)

var MagicNumber = [4]byte{'K', 'R', 'B', '1'}

const (
	FlagHasStyles        uint16 = 1 << 0
	FlagHasComponentDefs uint16 = 1 << 1
	FlagHasAnimations    uint16 = 1 << 2
	FlagHasResources     uint16 = 1 << 3
	FlagCompressed       uint16 = 1 << 4
	FlagFixedPoint       uint16 = 1 << 5
	FlagExtendedColor    uint16 = 1 << 6
	FlagHasApp           uint16 = 1 << 7
)

type ElementType uint8

const (
	ElemTypeApp         ElementType = 0x00
	ElemTypeContainer   ElementType = 0x01
	ElemTypeText        ElementType = 0x02
	ElemTypeImage       ElementType = 0x03
	ElemTypeCanvas      ElementType = 0x04
	ElemTypeButton      ElementType = 0x10
	ElemTypeInput       ElementType = 0x11
	ElemTypeList        ElementType = 0x20
	ElemTypeGrid        ElementType = 0x21
	ElemTypeScrollable  ElementType = 0x22
	ElemTypeVideo       ElementType = 0x30
	ElemTypeCustomStart ElementType = 0x31
)

type PropertyID uint8

const (
	PropIDInvalid           PropertyID = 0x00
	PropIDBgColor           PropertyID = 0x01
	PropIDFgColor           PropertyID = 0x02
	PropIDBorderColor       PropertyID = 0x03
	PropIDBorderWidth       PropertyID = 0x04
	PropIDBorderRadius      PropertyID = 0x05
	PropIDPadding           PropertyID = 0x06
	PropIDMargin            PropertyID = 0x07
	PropIDTextContent       PropertyID = 0x08
	PropIDFontSize          PropertyID = 0x09
	PropIDFontWeight        PropertyID = 0x0A
	PropIDTextAlignment     PropertyID = 0x0B
	PropIDImageSource       PropertyID = 0x0C
	PropIDOpacity           PropertyID = 0x0D
	PropIDZIndex            PropertyID = 0x0E
	PropIDVisibility        PropertyID = 0x0F
	PropIDGap               PropertyID = 0x10
	PropIDMinWidth          PropertyID = 0x11
	PropIDMinHeight         PropertyID = 0x12
	PropIDMaxWidth          PropertyID = 0x13
	PropIDMaxHeight         PropertyID = 0x14
	PropIDAspectRatio       PropertyID = 0x15
	PropIDTransform         PropertyID = 0x16
	PropIDShadow            PropertyID = 0x17
	PropIDOverflow          PropertyID = 0x18
	PropIDCustomDataBlob    PropertyID = 0x19
	PropIDLayoutFlags       PropertyID = 0x1A
	PropIDWindowWidth       PropertyID = 0x20
	PropIDWindowHeight      PropertyID = 0x21
	PropIDWindowTitle       PropertyID = 0x22
	PropIDResizable         PropertyID = 0x23
	PropIDKeepAspect        PropertyID = 0x24
	PropIDScaleFactor       PropertyID = 0x25
	PropIDIcon              PropertyID = 0x26
	PropIDVersion           PropertyID = 0x27
	PropIDAuthor            PropertyID = 0x28
)

type ValueType uint8

const (
	ValTypeNone       ValueType = 0x00
	ValTypeByte       ValueType = 0x01
	ValTypeShort      ValueType = 0x02
	ValTypeColor      ValueType = 0x03
	ValTypeString     ValueType = 0x04
	ValTypeResource   ValueType = 0x05
	ValTypePercentage ValueType = 0x06
	ValTypeRect       ValueType = 0x07
	ValTypeEdgeInsets ValueType = 0x08
	ValTypeEnum       ValueType = 0x09
	ValTypeVector     ValueType = 0x0A
	ValTypeCustom     ValueType = 0x0B
)

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
)

const (
	LayoutDirectionMask   uint8 = 0x03
	LayoutAlignmentMask   uint8 = 0x0C
	LayoutWrapBit         uint8 = 1 << 4
	LayoutGrowBit         uint8 = 1 << 5
	LayoutAbsoluteBit     uint8 = 1 << 6
)

const (
	LayoutDirRow           uint8 = 0x00
	LayoutDirColumn        uint8 = 0x01
	LayoutDirRowReverse    uint8 = 0x02
	LayoutDirColumnReverse uint8 = 0x03
)

const (
	LayoutAlignStart        uint8 = 0x00
	LayoutAlignCenter       uint8 = 0x01
	LayoutAlignEnd          uint8 = 0x02
	LayoutAlignSpaceBetween uint8 = 0x03
	LayoutAlignStretch      uint8 = 0x04 // Conceptual, for cross-axis
)

type ResourceType uint8

const (
	ResTypeNone   ResourceType = 0x00
	ResTypeImage  ResourceType = 0x01
	ResTypeFont   ResourceType = 0x02
	ResTypeSound  ResourceType = 0x03
	ResTypeVideo  ResourceType = 0x04
	ResTypeCustom ResourceType = 0x05
)

type ResourceFormat uint8

const (
	ResFormatExternal ResourceFormat = 0x00
	ResFormatInline   ResourceFormat = 0x01
)

type Header struct {
	Magic             [4]byte
	Version           uint16
	Flags             uint16
	ElementCount      uint16
	StyleCount        uint16
	ComponentDefCount uint16
	AnimationCount    uint16
	StringCount       uint16
	ResourceCount     uint16
	ElementOffset     uint32
	StyleOffset       uint32
	ComponentDefOffset uint32
	AnimationOffset   uint32
	StringOffset      uint32
	ResourceOffset    uint32
	TotalSize         uint32
}

const HeaderSize = 48

type ElementHeader struct {
	Type            ElementType
	ID              uint8
	PosX            uint16
	PosY            uint16
	Width           uint16
	Height          uint16
	Layout          uint8
	StyleID         uint8
	PropertyCount   uint8
	ChildCount      uint8
	EventCount      uint8
	AnimationCount  uint8
	CustomPropCount uint8
}

const ElementHeaderSize = 17

type Property struct {
	ID        PropertyID
	ValueType ValueType
	Size      uint8
	Value     []byte
}

type CustomProperty struct {
	KeyIndex  uint8
	ValueType ValueType
	Size      uint8
	Value     []byte
}

type EventFileEntry struct {
	EventType  EventType
	CallbackID uint8
}

const EventFileEntrySize = 2

type AnimationRef struct {
	AnimationIndex uint8
	Trigger        uint8
}

const AnimationRefSize = 2

type ChildRef struct {
	ChildOffset uint16
}

const ChildRefSize = 2

type Style struct {
	ID            uint8
	NameIndex     uint8
	PropertyCount uint8
	Properties    []Property
}

type Resource struct {
	Type            ResourceType
	NameIndex       uint8
	Format          ResourceFormat
	DataStringIndex uint8
	InlineDataSize  uint16
	InlineData      []byte
}

type KrbPropertyDefinition struct {
	NameIndex        uint8
	ValueTypeHint    ValueType
	DefaultValueSize uint8
	DefaultValueData []byte
}

type KrbComponentDefinition struct {
	NameIndex               uint8
	PropertyDefCount        uint8
	PropertyDefinitions     []KrbPropertyDefinition
	RootElementTemplateData []byte
}

type Document struct {
	Header               Header
	VersionMajor         uint8
	VersionMinor         uint8
	Elements             []ElementHeader
	ElementStartOffsets  []uint32
	Properties           [][]Property
	CustomProperties     [][]CustomProperty
	Events               [][]EventFileEntry
	ComponentDefinitions []KrbComponentDefinition
	Styles               []Style
	Animations           []byte
	Strings              []string
	Resources            []Resource
	ChildRefs            [][]ChildRef
	AnimationRefs        [][]AnimationRef
}

func (eh *ElementHeader) LayoutDirection() uint8 {
	return eh.Layout & LayoutDirectionMask
}

func (eh *ElementHeader) LayoutAlignment() uint8 {
	return (eh.Layout & LayoutAlignmentMask) >> 2
}

func (eh *ElementHeader) LayoutCrossAlignment() uint8 {
	mainAxisAlignment := eh.LayoutAlignment()
	if mainAxisAlignment == LayoutAlignSpaceBetween {
		// Default to Start for cross-axis if main is SpaceBetween.
		// Could also be LayoutAlignStretch if that's implemented in layout logic.
		return LayoutAlignStart
	}
	// Otherwise, cross-axis alignment mirrors main-axis alignment.
	return mainAxisAlignment
}

func (eh *ElementHeader) LayoutWrap() bool {
	return (eh.Layout & LayoutWrapBit) != 0
}

func (eh *ElementHeader) LayoutGrow() bool {
	return (eh.Layout & LayoutGrowBit) != 0
}

func (eh *ElementHeader) LayoutAbsolute() bool {
	return (eh.Layout & LayoutAbsoluteBit) != 0
}
