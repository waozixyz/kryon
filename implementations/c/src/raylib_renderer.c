#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "raylib.h"
#include "krb.h"

#define MAX_ELEMENTS 256
#define DEFAULT_WINDOW_WIDTH 800
#define DEFAULT_WINDOW_HEIGHT 600
#define DEFAULT_SCALE_FACTOR 1.0f

typedef struct RenderElement {
    KrbElementHeader header;
    char* text;
    uint32_t bg_color;      // Background color (RGBA)
    uint32_t fg_color;      // Text/Foreground color (RGBA)
    uint32_t border_color;  // Border color (RGBA)
    uint8_t border_widths[4]; // Top, right, bottom, left
    struct RenderElement* parent;
    struct RenderElement* children[MAX_ELEMENTS];
    int child_count;
} RenderElement;

Color rgba_to_raylib_color(uint32_t rgba) {
    uint8_t r = rgba & 0xFF;         // R at low byte (matches compiler)
    uint8_t g = (rgba >> 8) & 0xFF;
    uint8_t b = (rgba >> 16) & 0xFF;
    uint8_t a = (rgba >> 24) & 0xFF;
    return (Color){r, g, b, a};
}

char* strip_quotes(const char* input) {
    if (!input) return NULL;
    size_t len = strlen(input);
    if (len >= 2 && input[0] == '"' && input[len - 1] == '"') {
        char* stripped = malloc(len - 1);
        strncpy(stripped, input + 1, len - 2);
        stripped[len - 2] = '\0';
        return stripped;
    }
    return strdup(input);
}

void render_element(RenderElement* el, int parent_x, int parent_y, float scale_factor, FILE* debug_file) {
    int x = parent_x + (int)(el->header.pos_x * scale_factor);
    int y = parent_y + (int)(el->header.pos_y * scale_factor);
    int width = (int)(el->header.width * scale_factor);
    int height = (int)(el->header.height * scale_factor);
    if (width < 10) width = 10;  // Minimum size in pixels
    if (height < 6) height = 6;

    if (el->bg_color == 0 && el->parent) el->bg_color = el->parent->bg_color;
    if (el->fg_color == 0 && el->parent) el->fg_color = el->parent->fg_color;
    if (el->border_color == 0 && el->parent) el->border_color = el->parent->border_color;
    if (el->bg_color == 0) el->bg_color = 0xFF000000;     // Default black (ABGR)
    if (el->fg_color == 0) el->fg_color = 0xFFFFFFFF;     // Default white
    if (el->border_color == 0) el->border_color = 0xFF808080; // Default gray

    Color bg_color = rgba_to_raylib_color(el->bg_color);
    Color fg_color = rgba_to_raylib_color(el->fg_color);
    Color border_color = rgba_to_raylib_color(el->border_color);

    fprintf(debug_file, "DEBUG: Rendering element type=0x%02X at (%d, %d), size=%dx%d, text=%s, bg=0x%08X, fg=0x%08X, border=0x%08X\n",
            el->header.type, x, y, width, height, el->text ? el->text : "NULL", el->bg_color, el->fg_color, el->border_color);

    if (el->header.type == 0x00) { // App
        for (int i = 0; i < el->child_count; i++) {
            render_element(el->children[i], x, y, scale_factor, debug_file);
        }
    } else if (el->header.type == 0x01) { // Container
        DrawRectangle(x, y, width, height, bg_color);

        int top = (int)(el->border_widths[0] * scale_factor);
        int right = (int)(el->border_widths[1] * scale_factor);
        int bottom = (int)(el->border_widths[2] * scale_factor);
        int left = (int)(el->border_widths[3] * scale_factor);

        if (top > 0)
            DrawRectangle(x, y, width, top, border_color);
        if (bottom > 0)
            DrawRectangle(x, y + height - bottom, width, bottom, border_color);
        if (left > 0)
            DrawRectangle(x, y, left, height, border_color);
        if (right > 0)
            DrawRectangle(x + width - right, y, right, height, border_color);

        for (int i = 0; i < el->child_count; i++) {
            render_element(el->children[i], x + left, y + top, scale_factor, debug_file);
        }
    } else if (el->header.type == 0x02 && el->text) { // Text
        DrawRectangle(x, y, width, height, bg_color);
        int text_x = x + (int)(2 * scale_factor);
        int text_y = y + (int)(2 * scale_factor);
        DrawText(el->text, text_x, text_y, (int)(20 * scale_factor), fg_color);
    } else if (el->header.type == 0x30) { // Video (placeholder)
        fprintf(debug_file, "DEBUG: Video element (type 0x30) not supported, drawing placeholder\n");
        DrawRectangle(x, y, width, height, GRAY);
        DrawText("Video Placeholder", x + (int)(2 * scale_factor), y + height / 2, (int)(20 * scale_factor), WHITE);
    }
}

int main(int argc, char* argv[]) {
    if (argc != 2) {
        printf("Usage: %s <krb_file>\n", argv[0]);
        return 1;
    }

    FILE* debug_file = fopen("krb_debug.log", "w");
    if (!debug_file) {
        debug_file = stderr;
        fprintf(debug_file, "DEBUG: Using stderr for logging\n");
    }

    FILE* file = fopen(argv[1], "rb");
    if (!file) {
        fprintf(debug_file, "Error: Could not open file %s\n", argv[1]);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }

    KrbDocument doc = {0};
    if (!krb_read_document(file, &doc)) {
        fprintf(debug_file, "ERROR: Failed to parse KRB document\n");
        fclose(file);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }

    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    RenderElement* app_element = NULL;
    int window_width = DEFAULT_WINDOW_WIDTH;
    int window_height = DEFAULT_WINDOW_HEIGHT;
    float scale_factor = DEFAULT_SCALE_FACTOR;
    const char* window_title = "KRB Renderer (Raylib)";

    for (int i = 0; i < doc.header.element_count; i++) {
        elements[i].header = doc.elements[i];
        fprintf(debug_file, "Element %d: type=0x%02X, style_id=%d, props=%d\n",
                i, elements[i].header.type, elements[i].header.style_id, elements[i].header.property_count);

        if (elements[i].header.type == 0x00) {
            app_element = &elements[i];
            fprintf(debug_file, "DEBUG: Found App element at index %d\n", i);
        }

        if (elements[i].header.style_id > 0 && elements[i].header.style_id <= doc.header.style_count) {
            KrbStyle* style = &doc.styles[elements[i].header.style_id - 1];
            for (int j = 0; j < style->property_count; j++) {
                KrbProperty* prop = &style->properties[j];
                if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) {
                    elements[i].bg_color = *(uint32_t*)prop->value;
                } else if (prop->property_id == 0x02 && prop->value_type == 0x03 && prop->size == 4) {
                    elements[i].fg_color = *(uint32_t*)prop->value;
                } else if (prop->property_id == 0x03 && prop->value_type == 0x03 && prop->size == 4) {
                    elements[i].border_color = *(uint32_t*)prop->value;
                } else if (prop->property_id == 0x04 && prop->value_type == 0x01 && prop->size == 1) {
                    uint8_t width = *(uint8_t*)prop->value;
                    elements[i].border_widths[0] = elements[i].border_widths[1] = 
                    elements[i].border_widths[2] = elements[i].border_widths[3] = width;
                } else if (prop->property_id == 0x04 && prop->value_type == 0x08 && prop->size == 4) {
                    uint8_t* widths = (uint8_t*)prop->value;
                    elements[i].border_widths[0] = widths[0];
                    elements[i].border_widths[1] = widths[1];
                    elements[i].border_widths[2] = widths[2];
                    elements[i].border_widths[3] = widths[3];
                }
            }
        }

        for (int j = 0; j < elements[i].header.property_count; j++) {
            KrbProperty* prop = &doc.properties[i][j];
            if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) {
                elements[i].bg_color = *(uint32_t*)prop->value;
            } else if (prop->property_id == 0x02 && prop->value_type == 0x03 && prop->size == 4) {
                elements[i].fg_color = *(uint32_t*)prop->value;
            } else if (prop->property_id == 0x03 && prop->value_type == 0x03 && prop->size == 4) {
                elements[i].border_color = *(uint32_t*)prop->value;
            } else if (prop->property_id == 0x04 && prop->value_type == 0x01 && prop->size == 1) {
                uint8_t width = *(uint8_t*)prop->value;
                elements[i].border_widths[0] = elements[i].border_widths[1] = 
                elements[i].border_widths[2] = elements[i].border_widths[3] = width;
            } else if (prop->property_id == 0x04 && prop->value_type == 0x08 && prop->size == 4) {
                uint8_t* widths = (uint8_t*)prop->value;
                elements[i].border_widths[0] = widths[0];
                elements[i].border_widths[1] = widths[1];
                elements[i].border_widths[2] = widths[2];
                elements[i].border_widths[3] = widths[3];
            } else if (prop->property_id == 0x08 && prop->value_type == 0x04 && prop->size == 1) {
                uint8_t string_index = *(uint8_t*)prop->value;
                if (string_index < doc.header.string_count && doc.strings[string_index]) {
                    elements[i].text = strip_quotes(doc.strings[string_index]);
                }
            } else if (elements[i].header.type == 0x00) { // App-specific properties
                if (prop->property_id == 0x20 && prop->value_type == 0x02 && prop->size == 2) {
                    window_width = *(uint16_t*)prop->value;
                    fprintf(debug_file, "DEBUG: App WindowWidth=%d\n", window_width);
                } else if (prop->property_id == 0x21 && prop->value_type == 0x02 && prop->size == 2) {
                    window_height = *(uint16_t*)prop->value;
                    fprintf(debug_file, "DEBUG: App WindowHeight=%d\n", window_height);
                } else if (prop->property_id == 0x22 && prop->value_type == 0x04 && prop->size == 1) {
                    uint8_t string_index = *(uint8_t*)prop->value;
                    if (string_index < doc.header.string_count) {
                        window_title = doc.strings[string_index];
                        elements[i].text = strip_quotes(doc.strings[string_index]); // For rendering
                        fprintf(debug_file, "DEBUG: App WindowTitle='%s'\n", window_title);
                    }
                } else if (prop->property_id == 0x25 && prop->value_type == 0x06 && prop->size == 2) {
                    uint16_t fixed_point = *(uint16_t*)prop->value;
                    scale_factor = fixed_point / 256.0f;
                    fprintf(debug_file, "DEBUG: App ScaleFactor=%.2f\n", scale_factor);
                }
            }
        }
    }

    // Apply cascading styles from App
    if (app_element) {
        for (int i = 0; i < doc.header.element_count; i++) {
            if (elements[i].header.type != 0x00 && elements[i].header.style_id == 0) {
                elements[i].bg_color = elements[i].bg_color ? elements[i].bg_color : app_element->bg_color;
                elements[i].fg_color = elements[i].fg_color ? elements[i].fg_color : app_element->fg_color;
                elements[i].border_color = elements[i].border_color ? elements[i].border_color : app_element->border_color;
                if (elements[i].border_widths[0] == 0 && elements[i].border_widths[1] == 0 &&
                    elements[i].border_widths[2] == 0 && elements[i].border_widths[3] == 0) {
                    memcpy(elements[i].border_widths, app_element->border_widths, sizeof(elements[i].border_widths));
                }
            }
        }
    }

    // Build hierarchy
    for (int i = 0; i < doc.header.element_count; i++) {
        if (elements[i].header.child_count > 0) {
            int child_start = i + 1;
            for (int j = 0; j < elements[i].header.child_count && child_start + j < doc.header.element_count; j++) {
                elements[i].children[j] = &elements[child_start + j];
                elements[child_start + j].parent = &elements[i];
                elements[i].child_count++;
            }
        }
    }

    InitWindow(window_width, window_height, strip_quotes(window_title));
    SetTargetFPS(60);

    while (!WindowShouldClose()) {
        BeginDrawing();
        ClearBackground(BLACK);

        for (int i = 0; i < doc.header.element_count; i++) {
            if (!elements[i].parent) {
                render_element(&elements[i], 0, 0, scale_factor, debug_file);
            }
        }

        EndDrawing();
    }

    CloseWindow();

    for (int i = 0; i < doc.header.element_count; i++) {
        free(elements[i].text);
    }
    free(elements);
    krb_free_document(&doc);
    fclose(file);
    if (debug_file != stderr) fclose(debug_file);
    return 0;
}