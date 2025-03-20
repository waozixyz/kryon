#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "krb.h"

int read_header(FILE* file, KrbHeader* header) {
    // Read the full 38-byte header
    unsigned char buffer[38];
    
    // Get file size
    fseek(file, 0, SEEK_END);
    long file_size = ftell(file);
    fseek(file, 0, SEEK_SET);
    
    // Read only what's available, up to 38 bytes
    size_t bytes_read = fread(buffer, 1, 38, file);
    if (bytes_read < 38) {
        printf("Error: Failed to read header (only %zu bytes available)\n", bytes_read);
        return 0;
    }

    // Extract fields with explicit little-endian handling
    memcpy(header->magic, buffer, 4);
    header->version = (uint16_t)(buffer[4] | (buffer[5] << 8));
    header->flags = (uint16_t)(buffer[6] | (buffer[7] << 8));
    header->element_count = (uint16_t)(buffer[8] | (buffer[9] << 8));
    header->style_count = (uint16_t)(buffer[10] | (buffer[11] << 8));
    header->animation_count = (uint16_t)(buffer[12] | (buffer[13] << 8));
    header->string_count = (uint16_t)(buffer[14] | (buffer[15] << 8));
    header->resource_count = (uint16_t)(buffer[16] | (buffer[17] << 8));
    header->element_offset = (uint32_t)(buffer[18] | (buffer[19] << 8) | (buffer[20] << 16) | (buffer[21] << 24));
    header->style_offset = (uint32_t)(buffer[22] | (buffer[23] << 8) | (buffer[24] << 16) | (buffer[25] << 24));
    header->animation_offset = (uint32_t)(buffer[26] | (buffer[27] << 8) | (buffer[28] << 16) | (buffer[29] << 24));
    header->string_offset = (uint32_t)(buffer[30] | (buffer[31] << 8) | (buffer[32] << 16) | (buffer[33] << 24));
    header->resource_offset = (uint32_t)(buffer[34] | (buffer[35] << 8) | (buffer[36] << 16) | (buffer[37] << 24));
    
    // Use file_size as total_size
    header->total_size = file_size;

    if (memcmp(header->magic, "KRB1", 4) != 0) {
        printf("Error: Invalid magic number\n");
        return 0;
    }
    if (header->version != 0x0001) {
        printf("Error: Unsupported version (0x%04X)\n", header->version);
        return 0;
    }

    if (header->string_offset >= file_size) {
        printf("DEBUG: String table is at the end of file or missing\n");
        header->string_count = 0;
    } else {
        // Validate the string offset makes sense
        if (header->string_offset < 32) {
            printf("Error: Invalid string offset detected (offset: %u)\n", header->string_offset);
            return 0;
        }
    }
    
    printf("DEBUG: Read header - Element offset: %u, String offset: %u\n", 
           header->element_offset, header->string_offset);
    return 1;
}

void read_element_header(FILE* file, KrbElementHeader* element) {
    // Reset the element first
    memset(element, 0, sizeof(KrbElementHeader));
    
    unsigned char buffer[16];
    size_t bytes_read = fread(buffer, 1, 16, file);
    if (bytes_read != 16) {
        printf("Error: Failed to read element header (got %zu bytes)\n", bytes_read);
        return;
    }

    printf("DEBUG: Raw element header bytes: ");
    for (int i = 0; i < 16; i++) {
        printf("%02X ", buffer[i]);
    }
    printf("\n");

    // Based on the hex dump, the structure seems to be:
    element->type = buffer[0];
    element->id = buffer[1];
    element->pos_x = (uint16_t)(buffer[2] | (buffer[3] << 8));
    element->pos_y = (uint16_t)(buffer[4] | (buffer[5] << 8));
    element->width = (uint16_t)(buffer[6] | (buffer[7] << 8));
    element->height = (uint16_t)(buffer[8] | (buffer[9] << 8));
    element->layout = buffer[10];
    element->style_id = buffer[11];
    element->property_count = buffer[12];
    element->child_count = buffer[13];
    element->event_count = buffer[14];
    element->animation_count = buffer[15];
}
void read_property(FILE* file, KrbProperty* prop) {
    unsigned char buffer[3];
    size_t read_bytes = fread(buffer, 1, 3, file);
    if (read_bytes != 3) {
        printf("Error: Failed to read property fields (only read %zu bytes)\n", read_bytes);
        prop->property_id = 0;
        prop->value_type = 0;
        prop->size = 0;
        prop->value = NULL;
        return;
    }

    prop->property_id = buffer[0];
    prop->value_type = buffer[1];
    prop->size = buffer[2];
    
    // Better validation for property values
    if (prop->property_id > 0x20 || prop->value_type > 0x10 || prop->size > 64) {
        printf("Warning: Suspicious property values detected (ID: 0x%02X, Type: 0x%02X, Size: %d)\n", 
               prop->property_id, prop->value_type, prop->size);
        
        // Try to recover - treat this as a simple property with no data
        prop->size = 0;
        prop->value = NULL;
        return;
    }

    printf("DEBUG: Property ID: 0x%02X, Type: 0x%02X, Size: %d\n", 
           prop->property_id, prop->value_type, prop->size);

    if (prop->size > 0) {
        prop->value = malloc(prop->size);
        if (!prop->value) {
            printf("Error: Failed to allocate memory for property value\n");
            return;
        }
        
        size_t bytes_read = fread(prop->value, 1, prop->size, file);
        if (bytes_read != prop->size) {
            printf("Error: Failed to read property value (expected %d bytes, got %zu)\n", 
                   prop->size, bytes_read);
            free(prop->value);
            prop->value = NULL;
        }
    } else {
        prop->value = NULL;
    }
}

int krb_read_document(FILE* file, KrbDocument* doc) {
    if (!read_header(file, &doc->header)) {
        return 0;
    }

    // Read string table
    if (doc->header.string_count > 0 && doc->header.string_offset >= 32 && doc->header.string_offset < doc->header.total_size) {
        doc->strings = calloc(doc->header.string_count, sizeof(char*));
        fseek(file, doc->header.string_offset, SEEK_SET);
        
        // Debugging the file position
        printf("DEBUG: String table position before reading: %ld\n", ftell(file));
        
        // Read the string table count
        uint16_t string_table_count;
        if (fread(&string_table_count, 2, 1, file) != 1) {
            printf("Error: Failed to read string table count\n");
            free(doc->strings);
            doc->strings = NULL;
            return 0;
        }
        
        printf("DEBUG: String table position after reading count: %ld\n", ftell(file));
        
        // Read and display a few bytes to verify the next data
        unsigned char probe[4];
        long pos = ftell(file);
        fread(probe, 1, 4, file);
        printf("DEBUG: Next 4 bytes at position %ld: %02X %02X %02X %02X\n", 
               pos, probe[0], probe[1], probe[2], probe[3]);
        fseek(file, pos, SEEK_SET);  // Reset position
        
        // Convert from little-endian if needed
        string_table_count = (uint16_t)(string_table_count & 0xFF) | ((string_table_count & 0xFF00) >> 8);
        
        printf("DEBUG: String count from table: %u\n", string_table_count);
        
        if (string_table_count != doc->header.string_count) {
            printf("Warning: String count mismatch (header: %u, table: %u)\n", 
                   doc->header.string_count, string_table_count);
        }
        
        // Read each string from the string table
        for (int i = 0; i < string_table_count; i++) {
            // Get the current position for debugging
            long pos_before = ftell(file);
            
            // Read the length byte
            uint8_t length;
            if (fread(&length, 1, 1, file) != 1) {
                printf("Error: Failed to read string length for string %d\n", i);
                break;
            }
            
            printf("DEBUG: Read length byte at position %ld: %u\n", pos_before, length);
            printf("DEBUG: Reading string %d with length %u at file position %ld\n", 
                   i, length, ftell(file));
            
            // Validate the length
            if (length > 100 || ftell(file) + length > doc->header.total_size) {
                printf("Error: Invalid string length: %u for string %d\n", length, i);
                break;
            }
            
            // Allocate memory for the string (plus null terminator)
            doc->strings[i] = malloc(length + 1);
            if (!doc->strings[i]) {
                printf("Error: Failed to allocate memory for string %d\n", i);
                break;
            }
            
            // Read the string data
            size_t bytes_read = fread(doc->strings[i], 1, length, file);
            if (bytes_read != length) {
                printf("Error: Failed to read string data for string %d (expected %u bytes, got %zu)\n", 
                       i, length, bytes_read);
                free(doc->strings[i]);
                doc->strings[i] = NULL;
                break;
            }
            
            // Null-terminate the string
            doc->strings[i][length] = '\0';
            printf("DEBUG: String %d: '%s' (length: %u)\n", i, doc->strings[i], length);
        }
    }

    // Read elements
    if (doc->header.element_count > 0 && doc->header.element_offset >= 32 && doc->header.element_offset < doc->header.total_size) {
        doc->elements = calloc(doc->header.element_count, sizeof(KrbElementHeader));
        doc->properties = calloc(doc->header.element_count, sizeof(KrbProperty*));
        fseek(file, doc->header.element_offset, SEEK_SET);

        for (int i = 0; i < doc->header.element_count; i++) {
            read_element_header(file, &doc->elements[i]);
            doc->properties[i] = calloc(doc->elements[i].property_count, sizeof(KrbProperty));
            for (int j = 0; j < doc->elements[i].property_count; j++) {
                read_property(file, &doc->properties[i][j]);
            }
        }
    }

    return 1;
}

void krb_free_document(KrbDocument* doc) {
    if (doc->strings) {
        for (int i = 0; i < doc->header.string_count; i++) {
            if (doc->strings[i]) free(doc->strings[i]);
        }
        free(doc->strings);
    }
    if (doc->properties) {
        for (int i = 0; i < doc->header.element_count; i++) {
            if (doc->properties[i]) {
                for (int j = 0; j < doc->elements[i].property_count; j++) {
                    if (doc->properties[i][j].value) free(doc->properties[i][j].value);
                }
                free(doc->properties[i]);
            }
        }
        free(doc->properties);
    }
    if (doc->elements) free(doc->elements);
}
