#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "krb.h"

int read_header(FILE* file, KrbHeader* header) {
    unsigned char buffer[38];
    
    fseek(file, 0, SEEK_END);
    long file_size = ftell(file);
    fseek(file, 0, SEEK_SET);
    
    size_t bytes_read = fread(buffer, 1, 38, file);
    if (bytes_read < 38) return 0;

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
    header->total_size = (uint32_t)(buffer[34] | (buffer[35] << 8) | (buffer[36] << 16) | (buffer[37] << 24));
    
    if (memcmp(header->magic, "KRB1", 4) != 0 || header->version != 0x0001) return 0;

    // Optional: Validate that if Bit 6 is set, the first element is App (type 0x00)
    // We'll check this after reading elements to keep this function simple

    return 1;
}

void read_element_header(FILE* file, KrbElementHeader* element) {
    unsigned char buffer[16];
    fread(buffer, 1, 16, file);
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
    fread(buffer, 1, 3, file);
    prop->property_id = buffer[0];
    prop->value_type = buffer[1];
    prop->size = buffer[2];
    
    if (prop->size > 0) {
        prop->value = malloc(prop->size);
        fread(prop->value, 1, prop->size, file);

        switch (prop->value_type) {
            case 0x02: // Short (16-bit)
                if (prop->size == 2) {
                    uint8_t* bytes = (uint8_t*)prop->value;
                    *(uint16_t*)prop->value = (uint16_t)(bytes[0] | (bytes[1] << 8));
                }
                break;
            case 0x03: // Color (RGBA) - No reordering, keep as written
                break;
            case 0x06: // Percentage (8.8 fixed-point)
                if (prop->size == 2) {
                    uint8_t* bytes = (uint8_t*)prop->value;
                    *(uint16_t*)prop->value = (uint16_t)(bytes[0] | (bytes[1] << 8));
                }
                break;
            case 0x08: // EdgeInsets
                break;
            default:
                break;
        }
    } else {
        prop->value = NULL;
    }
}

int krb_read_document(FILE* file, KrbDocument* doc) {
    if (!read_header(file, &doc->header)) return 0;

    // Validate App element if Flag Bit 6 is set
    if (doc->header.flags & 0x0040) {
        fseek(file, doc->header.element_offset, SEEK_SET);
        unsigned char first_type;
        fread(&first_type, 1, 1, file);
        if (first_type != 0x00) {
            printf("Error: Flag indicates App element, but first element is not type 0x00\n");
            return 0;
        }
        fseek(file, doc->header.element_offset, SEEK_SET); // Reset position
    }

    if (doc->header.style_count > 0 && doc->header.style_offset >= 38) {
        doc->styles = calloc(doc->header.style_count, sizeof(KrbStyle));
        fseek(file, doc->header.style_offset, SEEK_SET);
        for (int i = 0; i < doc->header.style_count; i++) {
            unsigned char buffer[3];
            fread(buffer, 1, 3, file);
            doc->styles[i].id = buffer[0];
            doc->styles[i].name_index = buffer[1];
            doc->styles[i].property_count = buffer[2];
            if (doc->styles[i].property_count > 0) {
                doc->styles[i].properties = calloc(doc->styles[i].property_count, sizeof(KrbProperty));
                for (int j = 0; j < doc->styles[i].property_count; j++) {
                    read_property(file, &doc->styles[i].properties[j]);
                }
            }
        }
    }

    if (doc->header.string_count > 0 && doc->header.string_offset >= 38) {
        doc->strings = calloc(doc->header.string_count, sizeof(char*));
        fseek(file, doc->header.string_offset, SEEK_SET);
        uint16_t string_table_count;
        fread(&string_table_count, 2, 1, file);
        string_table_count = (uint16_t)(string_table_count & 0xFF) | ((string_table_count >> 8) & 0xFF);
        for (int i = 0; i < string_table_count; i++) {
            uint8_t length;
            fread(&length, 1, 1, file);
            doc->strings[i] = malloc(length + 1);
            fread(doc->strings[i], 1, length, file);
            doc->strings[i][length] = '\0';
        }
    }

    if (doc->header.element_count > 0 && doc->header.element_offset >= 38) {
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
    if (doc->styles) {
        for (int i = 0; i < doc->header.style_count; i++) {
            if (doc->styles[i].properties) {
                for (int j = 0; j < doc->styles[i].property_count; j++) {
                    if (doc->styles[i].properties[j].value) free(doc->styles[i].properties[j].value);
                }
                free(doc->styles[i].properties);
            }
        }
        free(doc->styles);
    }
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