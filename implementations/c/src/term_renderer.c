#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "krb.h"

typedef struct {
    KrbElementHeader header;
    char* text;
} KrbElement;

void dump_bytes(const void* data, size_t size) {
    const unsigned char* bytes = (const unsigned char*)data;
    for (size_t i = 0; i < size; i++) {
        printf("%02X ", bytes[i]);
        if ((i + 1) % 16 == 0) printf("\n");
    }
    printf("\n");
}

int main(int argc, char* argv[]) {
    if (argc != 2) {
        printf("Usage: %s <krb_file>\n", argv[0]);
        return 1;
    }

    FILE* file = fopen(argv[1], "rb");
    if (!file) {
        printf("Error: Could not open file %s\n", argv[1]);
        return 1;
    }

    // Get file size
    fseek(file, 0, SEEK_END);
    long file_size = ftell(file);
    fseek(file, 0, SEEK_SET);
    printf("DEBUG: File size: %ld bytes\n", file_size);

    // Dump full content
    unsigned char* file_data = malloc(file_size);
    fread(file_data, 1, file_size, file);
    printf("DEBUG: Full file content:\n");
    dump_bytes(file_data, file_size);
    fseek(file, 0, SEEK_SET);

    // Read header
    KrbHeader header;
    if (!read_header(file, &header)) {
        free(file_data);
        fclose(file);
        return 1;
    }

    printf("DEBUG: Magic: %.4s\n", header.magic);
    printf("DEBUG: Version: 0x%04X\n", header.version);
    printf("DEBUG: Flags: 0x%04X\n", header.flags);
    printf("DEBUG: Element count: %d\n", header.element_count);
    printf("DEBUG: Style count: %d\n", header.style_count);
    printf("DEBUG: Animation count: %d\n", header.animation_count);
    printf("DEBUG: String count: %d\n", header.string_count);
    printf("DEBUG: Resource count: %d\n", header.resource_count);
    printf("DEBUG: Element offset: %u\n", header.element_offset);
    printf("DEBUG: Style offset: %u\n", header.style_offset);
    printf("DEBUG: Animation offset: %u\n", header.animation_offset);
    printf("DEBUG: String offset: %u\n", header.string_offset);
    printf("DEBUG: Resource offset: %u\n", header.resource_offset);
    printf("DEBUG: Total size: %d\n", header.total_size);
    printf("Raw header bytes:\n");
    dump_bytes(&header, sizeof(KrbHeader));

    // Read string table
    char** strings = NULL;
    if (header.string_count > 0 && header.string_offset >= 32 && header.string_offset < file_size) {
        strings = calloc(header.string_count, sizeof(char*));
        fseek(file, header.string_offset, SEEK_SET);
        uint16_t string_count;
        if (fread(&string_count, sizeof(uint16_t), 1, file) != 1) {
            printf("Error: Failed to read string count\n");
        } else {
            for (int i = 0; i < string_count; i++) {
                uint8_t length;
                if (fread(&length, 1, 1, file) != 1) break;
                strings[i] = malloc(length + 1);
                if (fread(strings[i], 1, length, file) != length) {
                    free(strings[i]);
                    strings[i] = NULL;
                    break;
                }
                strings[i][length] = '\0';
                printf("DEBUG: String %d: '%s'\n", i, strings[i]);
            }
        }
    } else {
        printf("DEBUG: Skipping string table - invalid offset %u\n", header.string_offset);
    }

    // Read elements
    KrbElement* elements = NULL;
    if (header.element_count > 0 && header.element_offset >= 32 && header.element_offset < file_size) {
        elements = calloc(header.element_count, sizeof(KrbElement));
        fseek(file, header.element_offset, SEEK_SET);
        printf("DEBUG: Seeking to element offset: %u\n", header.element_offset);

        for (int i = 0; i < header.element_count; i++) {
            printf("DEBUG: Reading element %d at offset %ld\n", i, ftell(file));
            read_element_header(file, &elements[i].header);

            printf("Raw element header bytes:\n");
            dump_bytes(&elements[i].header, sizeof(KrbElementHeader));

            printf("DEBUG: Element type: 0x%02X\n", elements[i].header.type);
            printf("DEBUG: Element ID: %d\n", elements[i].header.id);
            printf("DEBUG: Position: (%d, %d)\n", elements[i].header.pos_x, elements[i].header.pos_y);
            printf("DEBUG: Size: %dx%d\n", elements[i].header.width, elements[i].header.height);
            printf("DEBUG: Property count: %d\n", elements[i].header.property_count);

            for (int j = 0; j < elements[i].header.property_count; j++) {
                KrbProperty prop;
                read_property(file, &prop);
                if (prop.property_id == 0x08 && prop.value_type == 0x04 && prop.size == 1 && strings) {
                    uint8_t string_index = *(uint8_t*)prop.value;
                    if (string_index < header.string_count && strings[string_index]) {
                        elements[i].text = strdup(strings[string_index]);
                        printf("DEBUG: Element text: '%s'\n", elements[i].text);
                    }
                }
                if (prop.value) free(prop.value);
            }
        }
    }

    // Render
    if (elements) {
        printf("\033[2J\033[H"); // Clear screen
        for (int i = 0; i < header.element_count; i++) {
            if (elements[i].text) {
                int rows = elements[i].header.pos_y / 16 + 1;
                int cols = elements[i].header.pos_x / 8 + 1;
                printf("\033[%d;%dH%s", rows, cols, elements[i].text);
            }
        }
        printf("\033[25;1H\n");
    }

    // Cleanup
    if (strings) {
        for (int i = 0; i < header.string_count; i++) if (strings[i]) free(strings[i]);
        free(strings);
    }
    if (elements) {
        for (int i = 0; i < header.element_count; i++) if (elements[i].text) free(elements[i].text);
        free(elements);
    }
    free(file_data);
    fclose(file);
    return 0;
}