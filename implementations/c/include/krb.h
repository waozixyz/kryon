#ifndef KRB_H
#define KRB_H

#include <stdint.h>
#include <stdio.h>   // For FILE*
#include <stdbool.h> // For bool type

// Define MAX_ELEMENTS if not defined elsewhere, used for array sizing in renderers
#ifndef MAX_ELEMENTS
#define MAX_ELEMENTS 256
#endif

// --- Constants from Specification ---

// Header Flags
#define FLAG_HAS_STYLES     (1 << 0)
#define FLAG_HAS_ANIMATIONS (1 << 1)
#define FLAG_HAS_RESOURCES  (1 << 2)
#define FLAG_COMPRESSED     (1 << 3) // Not implemented by current parser/compiler
#define FLAG_FIXED_POINT    (1 << 4) // Indicates usage of fixed-point values
#define FLAG_EXTENDED_COLOR (1 << 5) // Indicates usage of RGBA colors (vs palette)
#define FLAG_HAS_APP        (1 << 6) // Indicates App element is present as element 0
// Bits 7-15 Reserved

// Property IDs (Matching Specification)
#define PROP_ID_INVALID         0x00
#define PROP_ID_BG_COLOR        0x01
#define PROP_ID_FG_COLOR        0x02
#define PROP_ID_BORDER_COLOR    0x03
#define PROP_ID_BORDER_WIDTH    0x04 // Can be Byte or EdgeInsets
#define PROP_ID_BORDER_RADIUS   0x05
#define PROP_ID_PADDING         0x06 // Can be Short or EdgeInsets
#define PROP_ID_MARGIN          0x07 // Can be Short or EdgeInsets
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
#define PROP_ID_CUSTOM          0x19 // Uses string table ref for name
// App Specific
#define PROP_ID_WINDOW_WIDTH    0x20
#define PROP_ID_WINDOW_HEIGHT   0x21
#define PROP_ID_WINDOW_TITLE    0x22
#define PROP_ID_RESIZABLE       0x23
#define PROP_ID_KEEP_ASPECT     0x24
#define PROP_ID_SCALE_FACTOR    0x25
#define PROP_ID_ICON            0x26 // Resource index (or string index for path)
#define PROP_ID_VERSION         0x27
#define PROP_ID_AUTHOR          0x28
// 0x29 - 0xFF Reserved

// Value Types (Matching Specification)
#define VAL_TYPE_NONE       0x00
#define VAL_TYPE_BYTE       0x01
#define VAL_TYPE_SHORT      0x02
#define VAL_TYPE_COLOR      0x03 // RGBA or palette index
#define VAL_TYPE_STRING     0x04 // Index to string table (1 byte)
#define VAL_TYPE_RESOURCE   0x05 // Index to resource table (1 byte)
#define VAL_TYPE_PERCENTAGE 0x06 // Fixed-point (e.g., 8.8) - size depends on flag
#define VAL_TYPE_RECT       0x07 // x,y,w,h (e.g., 4 shorts = 8 bytes)
#define VAL_TYPE_EDGEINSETS 0x08 // top,right,bottom,left (e.g., 4 bytes)
#define VAL_TYPE_ENUM       0x09 // Predefined options (1 byte usually)
#define VAL_TYPE_VECTOR     0x0A // x,y coords (e.g., 2 shorts = 4 bytes)
#define VAL_TYPE_CUSTOM     0x0B // Depends on context
// 0x0C - 0xFF Reserved

// Layout Byte Bits (Matching Specification)
#define LAYOUT_DIRECTION_MASK 0x03 // Bits 0-1: 00=Row, 01=Col, 10=RowRev, 11=ColRev
#define LAYOUT_ALIGNMENT_MASK 0x0C // Bits 2-3: 00=Start, 01=Center, 10=End, 11=SpaceBetween
#define LAYOUT_WRAP_BIT       (1 << 4) // Bit 4: 0=NoWrap, 1=Wrap
#define LAYOUT_GROW_BIT       (1 << 5) // Bit 5: 0=Fixed, 1=Grow
#define LAYOUT_ABSOLUTE_BIT   (1 << 6) // Bit 6: 0=Flow, 1=Absolute
// Bit 7 Reserved

// --- Data Structures ---

// Ensure structs that directly map to file format are packed
#pragma pack(push, 1)

typedef struct {
    char magic[4];           // "KRB1"
    uint16_t version;        // 0x0001 for v1.0 (Little Endian: 01 00)
    uint16_t flags;          // Bitfield using FLAG_* constants
    uint16_t element_count;
    uint16_t style_count;
    uint16_t animation_count;
    uint16_t string_count;
    uint16_t resource_count;
    uint32_t element_offset;
    uint32_t style_offset;
    uint32_t animation_offset;
    uint32_t string_offset;
    uint32_t total_size;    // Spec v1.0 puts this here (Offset 34)
    // uint32_t resource_offset; // Not in Spec v1.0 header
} KrbHeader;

typedef struct {
    uint8_t type;            // Element type (e.g., 0x00=App, 0x01=Container, 0x02=Text)
    uint8_t id;              // String table index or 0 (For Element ID name)
    uint16_t pos_x;
    uint16_t pos_y;
    uint16_t width;
    uint16_t height;
    uint8_t layout;          // Layout properties bitfield (Uses LAYOUT_* constants)
    uint8_t style_id;        // Style reference (1-based) or 0
    uint8_t property_count;
    uint8_t child_count;
    uint8_t event_count;
    uint8_t animation_count;
} KrbElementHeader;

#pragma pack(pop)

// Structures representing data parsed into memory (don't need packing)
typedef struct {
    uint8_t property_id;     // e.g., PROP_ID_BG_COLOR, PROP_ID_WINDOW_WIDTH
    uint8_t value_type;      // e.g., VAL_TYPE_BYTE, VAL_TYPE_COLOR, VAL_TYPE_STRING
    uint8_t size;            // Size of value data in bytes
    void* value;             // Pointer to allocated memory holding the raw property value
} KrbProperty;

typedef struct {
    uint8_t id;              // Style identifier (1-based), corresponds to index+1 in doc->styles
    uint8_t name_index;      // String table index for style name
    uint8_t property_count;
    KrbProperty* properties; // Dynamically allocated array of KrbProperty for this style
} KrbStyle;

// Represents the entire parsed KRB document data in memory
typedef struct {
    KrbHeader header;

    // Flat array of element headers read sequentially from the file
    KrbElementHeader* elements;

    // Array of pointers to property arrays. Indexed same as 'elements'.
    // doc.properties[i] is a pointer to an array of KrbProperty structs for element i.
    // If element i has no properties, doc.properties[i] will be NULL.
    KrbProperty** properties;

    // Array of styles read from the file
    KrbStyle* styles;

    // Array of pointers to null-terminated strings read from the file
    char** strings;

    // TODO: Add fields for animations, resources if implemented
    // KrbAnimation* animations;
    // KrbResource* resources;

} KrbDocument;


// --- Function Prototypes for krb_reader.c ---

// Reads the entire KRB document structure into memory from the given file.
// Allocates memory for elements, properties, styles, and strings.
// Returns 1 on success, 0 on failure. Caller must call krb_free_document on success or failure.
int krb_read_document(FILE* file, KrbDocument* doc);

// Frees all memory dynamically allocated by krb_read_document within the KrbDocument structure.
// Safe to call even if krb_read_document failed partway through.
void krb_free_document(KrbDocument* doc);

// (Optional: If you want read_header to be public, uncomment)
// Reads only the main file header. Returns 1 on success, 0 on failure.
// int read_header(FILE* file, KrbHeader* header);


#endif // KRB_H