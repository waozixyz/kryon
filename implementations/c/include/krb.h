#ifndef KRB_H
#define KRB_H

#include <stdint.h>

#define MAX_ELEMENTS 256

#pragma pack(push, 1)

typedef struct {
    char magic[4];           // "KRB1"
    uint16_t version;        // 0x0001 for v1.0
    uint16_t flags;          // Bit 0: Has styles, Bit 1: Has animations, Bit 2: Has resources, 
                             // Bit 3: Compressed, Bit 4: Fixed-point, Bit 5: Extended color, 
                             // Bit 6: Has App element, Bit 7-15: Reserved
    uint16_t element_count;
    uint16_t style_count;
    uint16_t animation_count;
    uint16_t string_count;
    uint16_t resource_count;
    uint32_t element_offset;
    uint32_t style_offset;
    uint32_t animation_offset;
    uint32_t string_offset;
    uint32_t total_size; 
} KrbHeader;

typedef struct {
    uint8_t type;            // Element type (e.g., 0x00=App, 0x01=Container, 0x02=Text, 0x30=Video)
    uint8_t id;              // String table index or 0
    uint16_t pos_x;
    uint16_t pos_y;
    uint16_t width;
    uint16_t height;
    uint8_t layout;          // Layout properties bitfield
    uint8_t style_id;        // Style reference or 0; for App, cascades to children unless overridden
    uint8_t property_count;
    uint8_t child_count;
    uint8_t event_count;
    uint8_t animation_count;
} KrbElementHeader;

typedef struct {
    uint8_t property_id;     // e.g., 0x01=BackgroundColor, 0x20=WindowWidth
    uint8_t value_type;      // e.g., 0x01=Byte, 0x02=Short, 0x03=Color, 0x04=String, 0x06=Percentage
    uint8_t size;            // Size of value in bytes
    void* value;             // Pointer to value data
} KrbProperty;

typedef struct {
    uint8_t id;              // Style identifier
    uint8_t name_index;      // String table index for style name
    uint8_t property_count;
    KrbProperty* properties; // Array of properties
} KrbStyle;

typedef struct {
    KrbHeader header;
    KrbElementHeader* elements;  // Array of element headers
    KrbProperty** properties;    // Array of property arrays, indexed by element
    KrbStyle* styles;            // Array of styles
    char** strings;              // Array of string pointers
} KrbDocument;

#pragma pack(pop)

int read_header(FILE* file, KrbHeader* header);
void read_element_header(FILE* file, KrbElementHeader* element);
void read_property(FILE* file, KrbProperty* prop);
int krb_read_document(FILE* file, KrbDocument* doc);
void krb_free_document(KrbDocument* doc);

#endif