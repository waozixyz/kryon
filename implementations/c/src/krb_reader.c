#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "krb.h"

int read_header(FILE* file, KrbHeader* header) {
    // Read each field and handle endianness explicitly
    if (fread(header->magic, 1, 4, file) != 4) {
        printf("Error: Failed to read magic\n");
        return 0;
    }
    
    // Read 2-byte values
    uint16_t buffer16;
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->version = buffer16;
    
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->flags = buffer16;
    
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->element_count = buffer16;
    
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->style_count = buffer16;
    
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->animation_count = buffer16;
    
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->string_count = buffer16;
    
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->resource_count = buffer16;
    
    // Read 4-byte values
    uint32_t buffer32;
    uint8_t bytes[4];
    
    // Element offset
    if (fread(bytes, 1, 4, file) != 4) return 0;
    header->element_offset = (bytes[0]) | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24);
    
    // Style offset
    if (fread(bytes, 1, 4, file) != 4) return 0;
    header->style_offset = (bytes[0]) | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24);
    
    // Animation offset
    if (fread(bytes, 1, 4, file) != 4) return 0;
    header->animation_offset = (bytes[0]) | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24);
    
    // String offset
    if (fread(bytes, 1, 4, file) != 4) return 0;
    header->string_offset = (bytes[0]) | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24);
    
    // Resource offset
    if (fread(bytes, 1, 4, file) != 4) return 0;
    header->resource_offset = (bytes[0]) | (bytes[1] << 8) | (bytes[2] << 16) | (bytes[3] << 24);
    
    // Total size
    if (fread(&buffer16, 2, 1, file) != 1) return 0;
    header->total_size = buffer16;

    if (memcmp(header->magic, "KRB1", 4) != 0) {
        printf("Error: Invalid magic number\n");
        return 0;
    }

    printf("DEBUG: Read header - Element offset: %u, String offset: %u\n", 
           header->element_offset, header->string_offset);
    return 1;
}

void read_element_header(FILE* file, KrbElementHeader* element) {
    if (fread(element, sizeof(KrbElementHeader), 1, file) != 1) {
        printf("Error: Failed to read element header\n");
    }
}

void read_property(FILE* file, KrbProperty* prop) {
    if (fread(&prop->property_id, 1, 1, file) != 1 ||
        fread(&prop->value_type, 1, 1, file) != 1 ||
        fread(&prop->size, 1, 1, file) != 1) {
        printf("Error: Failed to read property fields\n");
        prop->value = NULL;
        return;
    }

    printf("DEBUG: Property ID: 0x%02X, Type: 0x%02X, Size: %d\n", 
           prop->property_id, prop->value_type, prop->size);

    if (prop->size > 0) {
        prop->value = malloc(prop->size);
        if (fread(prop->value, 1, prop->size, file) != prop->size) {
            printf("Error: Failed to read property value\n");
            free(prop->value);
            prop->value = NULL;
        }
    } else {
        prop->value = NULL;
    }
}