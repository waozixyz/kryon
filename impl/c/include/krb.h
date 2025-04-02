#ifndef KRB_H
#define KRB_H

#include <stdint.h>
#include <stdio.h>   // For FILE*
#include <stdbool.h> // For bool type

// Define MAX_ELEMENTS if not defined elsewhere
#ifndef MAX_ELEMENTS
#define MAX_ELEMENTS 256
#endif

// --- Constants from KRB v0.2 Specification ---

#define KRB_SPEC_VERSION_MAJOR 0
#define KRB_SPEC_VERSION_MINOR 3

// Header Flags
#define FLAG_HAS_STYLES     (1 << 0)
#define FLAG_HAS_ANIMATIONS (1 << 1)
#define FLAG_HAS_RESOURCES  (1 << 2)
#define FLAG_COMPRESSED     (1 << 3)
#define FLAG_FIXED_POINT    (1 << 4)
#define FLAG_EXTENDED_COLOR (1 << 5)
#define FLAG_HAS_APP        (1 << 6)
// Bits 7-15 Reserved

// Element Types
#define ELEM_TYPE_APP         0x00
#define ELEM_TYPE_CONTAINER   0x01
#define ELEM_TYPE_TEXT        0x02
#define ELEM_TYPE_IMAGE       0x03
#define ELEM_TYPE_CANVAS      0x04
#define ELEM_TYPE_BUTTON      0x10
#define ELEM_TYPE_INPUT       0x11
#define ELEM_TYPE_LIST        0x20
#define ELEM_TYPE_GRID        0x21
#define ELEM_TYPE_SCROLLABLE  0x22
#define ELEM_TYPE_VIDEO       0x30
// 0x31-0xFF Custom/Specialized

// Property IDs
#define PROP_ID_INVALID         0x00
#define PROP_ID_BG_COLOR        0x01
#define PROP_ID_FG_COLOR        0x02
#define PROP_ID_BORDER_COLOR    0x03
#define PROP_ID_BORDER_WIDTH    0x04
#define PROP_ID_BORDER_RADIUS   0x05
#define PROP_ID_PADDING         0x06
#define PROP_ID_MARGIN          0x07
#define PROP_ID_TEXT_CONTENT    0x08
#define PROP_ID_FONT_SIZE       0x09
#define PROP_ID_FONT_WEIGHT     0x0A
#define PROP_ID_TEXT_ALIGNMENT  0x0B
#define PROP_ID_IMAGE_SOURCE    0x0C
#define PROP_ID_OPACITY         0x0D
#define PROP_ID_ZINDEX          0x0E
#define PROP_ID_VISIBILITY      0x0F
#define PROP_ID_GAP             0x10
#define PROP_ID_MIN_WIDTH       0x11
#define PROP_ID_MIN_HEIGHT      0x12
#define PROP_ID_MAX_WIDTH       0x13
#define PROP_ID_MAX_HEIGHT      0x14
#define PROP_ID_ASPECT_RATIO    0x15
#define PROP_ID_TRANSFORM       0x16
#define PROP_ID_SHADOW          0x17
#define PROP_ID_OVERFLOW        0x18
#define PROP_ID_CUSTOM          0x19
#define PROP_ID_LAYOUT_FLAGS    0x1A
#define PROP_ID_WINDOW_WIDTH    0x20
#define PROP_ID_WINDOW_HEIGHT   0x21
#define PROP_ID_WINDOW_TITLE    0x22
#define PROP_ID_RESIZABLE       0x23
#define PROP_ID_KEEP_ASPECT     0x24
#define PROP_ID_SCALE_FACTOR    0x25
#define PROP_ID_ICON            0x26
#define PROP_ID_VERSION         0x27
#define PROP_ID_AUTHOR          0x28
// 0x29 - 0xFF Reserved

// Value Types
#define VAL_TYPE_NONE       0x00
#define VAL_TYPE_BYTE       0x01
#define VAL_TYPE_SHORT      0x02
#define VAL_TYPE_COLOR      0x03
#define VAL_TYPE_STRING     0x04
#define VAL_TYPE_RESOURCE   0x05
#define VAL_TYPE_PERCENTAGE 0x06
#define VAL_TYPE_RECT       0x07
#define VAL_TYPE_EDGEINSETS 0x08
#define VAL_TYPE_ENUM       0x09
#define VAL_TYPE_VECTOR     0x0A
#define VAL_TYPE_CUSTOM     0x0B
// 0x0C - 0xFF Reserved

// Event Types
#define EVENT_TYPE_NONE     0x00
#define EVENT_TYPE_CLICK    0x01
#define EVENT_TYPE_PRESS    0x02
#define EVENT_TYPE_RELEASE  0x03
#define EVENT_TYPE_LONGPRESS 0x04
#define EVENT_TYPE_HOVER    0x05
#define EVENT_TYPE_FOCUS    0x06
#define EVENT_TYPE_BLUR     0x07
#define EVENT_TYPE_CHANGE   0x08
#define EVENT_TYPE_SUBMIT   0x09
#define EVENT_TYPE_CUSTOM   0x0A
// 0x0B-0xFF Reserved

// Layout Byte Bits
#define LAYOUT_DIRECTION_MASK 0x03
#define LAYOUT_ALIGNMENT_MASK 0x0C
#define LAYOUT_WRAP_BIT       (1 << 4)
#define LAYOUT_GROW_BIT       (1 << 5)
#define LAYOUT_ABSOLUTE_BIT   (1 << 6)
// Bit 7 Reserved

// Resource Types
#define RES_TYPE_NONE       0x00
#define RES_TYPE_IMAGE      0x01
#define RES_TYPE_FONT       0x02
#define RES_TYPE_SOUND      0x03
#define RES_TYPE_VIDEO      0x04
#define RES_TYPE_CUSTOM     0x05
// 0x06 - 0xFF Reserved

// Resource Formats
#define RES_FORMAT_EXTERNAL 0x00
#define RES_FORMAT_INLINE   0x01


// --- Data Structures ---

#pragma pack(push, 1)

// KRB v0.2 Header Structure (42 bytes) - Matches file format exactly
typedef struct {
    char magic[4];           // "KRB1"
    uint16_t version;        // Minor << 8 | Major (e.g., 0x0002 for 0.2)
    uint16_t flags;
    uint16_t element_count;
    uint16_t style_count;
    uint16_t animation_count;
    uint16_t string_count;
    uint16_t resource_count;
    uint32_t element_offset;
    uint32_t style_offset;
    uint32_t animation_offset;
    uint32_t string_offset;
    uint32_t resource_offset; // Added for v0.2
    uint32_t total_size;
} KrbHeader;

// Element Header (17 bytes)
typedef struct {
    uint8_t type;
    uint8_t id;              // 0-based string index
    uint16_t pos_x;
    uint16_t pos_y;
    uint16_t width;
    uint16_t height;
    uint8_t layout;
    uint8_t style_id;        // 1-based style ID
    uint8_t property_count;
    uint8_t child_count;
    uint8_t event_count;
    uint8_t animation_count;
    uint8_t custom_prop_count; 
} KrbElementHeader;

// Event structure (as stored in file - 2 bytes)
typedef struct {
    uint8_t event_type;
    uint8_t callback_id;   // 0-based string index
} KrbEventFileEntry;

#pragma pack(pop)


// Structures representing data parsed into memory (don't need packing)

typedef struct {
    uint8_t property_id;
    uint8_t value_type;
    uint8_t size;
    void* value;
} KrbProperty;

typedef struct {
    uint8_t id;
    uint8_t name_index;
    uint8_t property_count;
    KrbProperty* properties;
} KrbStyle;

typedef struct {
    uint8_t type;
    uint8_t name_index;
    uint8_t format;
    uint8_t data_string_index; // Only if format is External
    // void* inline_data;    // TODO for inline
    // size_t inline_data_size; // TODO for inline
} KrbResource;

// Represents the entire parsed KRB document data in memory
typedef struct {
    KrbHeader header; // Copy of the raw header data

    // Parsed version for convenience
    uint8_t version_major; // <-- Moved here
    uint8_t version_minor; // <-- Moved here

    // Arrays holding parsed data
    KrbElementHeader* elements;
    KrbProperty** properties;
    KrbEventFileEntry** events;
    KrbStyle* styles;
    char** strings;
    KrbResource* resources;
    // KrbAnimation* animations; // TODO

} KrbDocument;


// --- Function Prototypes for krb_reader.c ---

// Reads the entire KRB document structure into memory.
bool krb_read_document(FILE* file, KrbDocument* doc);

// Frees all memory dynamically allocated by krb_read_document.
void krb_free_document(KrbDocument* doc);

// Helpers for reading little-endian values
uint16_t krb_read_u16_le(const void* data);
uint32_t krb_read_u32_le(const void* data);


#endif // KRB_H