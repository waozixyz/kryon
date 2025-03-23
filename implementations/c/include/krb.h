#ifndef KRB_H
#define KRB_H

#include <stdint.h>

#define MAX_ELEMENTS 256

#pragma pack(push, 1)

typedef struct {
    char magic[4];
    uint16_t version;
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
    uint32_t resource_offset;
    uint32_t total_size; 
} KrbHeader;

typedef struct {
    uint8_t type;
    uint8_t id;
    uint16_t pos_x;
    uint16_t pos_y;
    uint16_t width;
    uint16_t height;
    uint8_t layout;
    uint8_t style_id;
    uint8_t property_count;
    uint8_t child_count;
    uint8_t event_count;
    uint8_t animation_count;
} KrbElementHeader;

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
    KrbHeader header;
    KrbElementHeader* elements;
    KrbProperty** properties;
    KrbStyle* styles;
    char** strings;
} KrbDocument;

#pragma pack(pop)

int read_header(FILE* file, KrbHeader* header);
void read_element_header(FILE* file, KrbElementHeader* element);
void read_property(FILE* file, KrbProperty* prop);
int krb_read_document(FILE* file, KrbDocument* doc);
void krb_free_document(KrbDocument* doc);

#endif