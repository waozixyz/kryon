#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "krb.h"
#include "raylib.h"

typedef struct {
    KrbElementHeader header;
    char* text;
    Color bg_color;
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

    // Get file size and dump content
    fseek(file, 0, SEEK_END);
    long file_size = ftell(file);
    fseek(file, 0, SEEK_SET);
    printf("DEBUG: File size: %ld bytes\n", file_size);
    unsigned char* file_data = malloc(file_size);
    fread(file_data, 1, file_size, file);
    printf("DEBUG: Full file content:\n");
    dump_bytes(file_data, file_size);
    fseek(file, 0, SEEK_SET);
    free(file_data);

    // Read the document
    KrbDocument doc = {0};
    if (!krb_read_document(file, &doc)) {
        fclose(file);
        return 1;
    }

    // Convert to renderer-specific structure
    KrbElement* elements = calloc(doc.header.element_count, sizeof(KrbElement));
    for (int i = 0; i < doc.header.element_count; i++) {
        elements[i].header = doc.elements[i];
        elements[i].bg_color = WHITE; // Default background
        for (int j = 0; j < doc.elements[i].property_count; j++) {
            KrbProperty* prop = &doc.properties[i][j];
            if (prop->property_id == 0x08 && prop->value_type == 0x04 && prop->size == 1 && doc.strings) {
                uint8_t string_index = *(uint8_t*)prop->value;
                if (string_index < doc.header.string_count && doc.strings[string_index]) {
                    elements[i].text = strdup(doc.strings[string_index]);
                    printf("DEBUG: Element text: '%s'\n", elements[i].text);
                }
            }
            else if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) {
                // BackgroundColor (RGBA)
                unsigned char* color = (unsigned char*)prop->value;
                elements[i].bg_color = (Color){color[0], color[1], color[2], color[3]};
            }
        }
    }

    // Initialize Raylib
    InitWindow(800, 600, "KRB Raylib Renderer");
    SetTargetFPS(60);

    // Main render loop
    while (!WindowShouldClose()) {
        BeginDrawing();
        ClearBackground(RAYWHITE);

        // Render elements
        for (int i = 0; i < doc.header.element_count; i++) {
            KrbElement* el = &elements[i];
            Rectangle rect = {
                el->header.pos_x,
                el->header.pos_y,
                el->header.width,
                el->header.height
            };
            DrawRectangleRec(rect, el->bg_color);
            if (el->text) {
                DrawText(el->text, el->header.pos_x + 5, el->header.pos_y + 5, 20, BLACK);
            }
        }

        EndDrawing();
    }

    // Cleanup
    CloseWindow();
    for (int i = 0; i < doc.header.element_count; i++) {
        if (elements[i].text) free(elements[i].text);
    }
    free(elements);
    krb_free_document(&doc);
    fclose(file);
    return 0;
}