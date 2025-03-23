#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <termbox.h>
#include "krb.h"

#define MAX_ELEMENTS 256

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

int rgb_to_tb_color(uint32_t rgba, FILE* debug_file) {
    uint8_t r = (rgba >> 24) & 0xFF;
    uint8_t g = (rgba >> 16) & 0xFF;
    uint8_t b = (rgba >> 8) & 0xFF;
    uint8_t a = rgba & 0xFF;

    fprintf(debug_file, "DEBUG: Converting RGBA=0x%08X (R=%d, G=%d, B=%d, A=%d)\n", rgba, r, g, b, a);

    if (a < 128) return TB_DEFAULT; // Transparent defaults to terminal background

    // Enhanced color mapping for Termbox
    if (r > 200 && g > 200 && b > 200) return TB_WHITE;
    if (r > 200 && g < 100 && b < 100) return TB_RED;
    if (r < 100 && g > 200 && b < 100) return TB_GREEN;
    if (r < 100 && g < 100 && b > 200) return TB_BLUE; // Matches #191970FF (R=25, G=112, B=112)
    if (r > 200 && g > 200 && b < 100) return TB_YELLOW; // Matches #FFFF00FF
    if (r > 150 && g < 100 && b > 150) return TB_MAGENTA;
    if (r < 100 && g > 200 && b > 200) return TB_CYAN; // Matches #00FFFFFF
    if (r < 50 && g < 50 && b < 50) return TB_BLACK;

    // Fallback for dark blue like #191970FF
    if (r < 100 && g > 50 && b > 50) return TB_BLUE;

    return TB_DEFAULT;
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

void render_element(RenderElement* el, int parent_x, int parent_y, FILE* debug_file) {
    int x = parent_x + el->header.pos_x / 10;
    int y = parent_y + el->header.pos_y / 10;
    int width = el->header.width / 10;
    int height = el->header.height / 10;
    if (width < 5) width = 5;
    if (height < 3) height = 3;

    if (el->bg_color == 0 && el->parent) el->bg_color = el->parent->bg_color;
    if (el->fg_color == 0 && el->parent) el->fg_color = el->parent->fg_color;
    if (el->border_color == 0 && el->parent) el->border_color = el->parent->border_color;
    if (el->bg_color == 0) el->bg_color = 0x000000FF; // Default transparent (will use TB_DEFAULT)
    if (el->fg_color == 0) el->fg_color = 0xFFFFFFFF; // Default white
    if (el->border_color == 0) el->border_color = 0x808080FF; // Default gray

    int width_term = tb_width();
    int height_term = tb_height();
    if (x >= width_term || y >= height_term) {
        fprintf(debug_file, "WARNING: Element at (%d, %d) outside bounds (%d, %d)\n", x, y, width_term, height_term);
        return;
    }

    int bg_color = rgb_to_tb_color(el->bg_color, debug_file);
    int fg_color = rgb_to_tb_color(el->fg_color, debug_file);
    int border_color = rgb_to_tb_color(el->border_color, debug_file);

    fprintf(debug_file, "DEBUG: Rendering element type=0x%02X at (%d, %d), size=%dx%d, text=%s, bg=0x%08X (%d), fg=0x%08X (%d), border=0x%08X (%d)\n",
            el->header.type, x, y, width, height, el->text ? el->text : "NULL", el->bg_color, bg_color, el->fg_color, fg_color, el->border_color, border_color);

    if (el->header.type == 0x01) { // Container
        fprintf(debug_file, "DEBUG: Drawing Container with border widths: top=%d, right=%d, bottom=%d, left=%d\n",
                el->border_widths[0], el->border_widths[1], el->border_widths[2], el->border_widths[3]);

        for (int i = 0; i < width; i++) {
            for (int j = 0; j < height; j++) {
                int cur_x = x + i;
                int cur_y = y + j;
                if (cur_x >= width_term || cur_y >= height_term) continue;

                int is_border = (j < el->border_widths[0] || j >= height - el->border_widths[2] || 
                                 i < el->border_widths[3] || i >= width - el->border_widths[1]);
                if (is_border) {
                    char border_char = (i == 0 || i == width-1) ? (j == 0 || j == height-1 ? '+' : '|') : '-';
                    tb_change_cell(cur_x, cur_y, border_char, border_color, bg_color);
                } else {
                    tb_change_cell(cur_x, cur_y, ' ', fg_color, bg_color); // Explicitly fill background
                }
            }
        }

        for (int i = 0; i < el->child_count; i++) {
            render_element(el->children[i], x + el->border_widths[3], y + el->border_widths[0], debug_file);
        }
    } else if (el->header.type == 0x02 && el->text) { // Text
        int text_x = x + 1;
        int text_y = y + 1;
        fprintf(debug_file, "DEBUG: Rendering text '%s' at (%d, %d) with fg=0x%08X (%d), bg=0x%08X (%d)\n", 
                el->text, text_x, text_y, el->fg_color, fg_color, el->bg_color, bg_color);

        if (text_x < width_term && text_y < height_term) {
            size_t text_len = strlen(el->text);
            for (int i = 0; i < width - 2 && text_x + i < width_term; i++) {
                tb_change_cell(text_x + i, text_y, ' ', fg_color, bg_color);
            }
            for (size_t i = 0; i < text_len && i < width - 2 && text_x + i < width_term; i++) {
                tb_change_cell(text_x + i, text_y, el->text[i], fg_color, bg_color);
            }
        }
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
    for (int i = 0; i < doc.header.element_count; i++) {
        elements[i].header = doc.elements[i];
        fprintf(debug_file, "Element %d: type=0x%02X, style_id=%d, props=%d\n",
                i, elements[i].header.type, elements[i].header.style_id, elements[i].header.property_count);

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
            }
        }
    }

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

    if (tb_init() != 0) {
        fprintf(debug_file, "ERROR: Failed to initialize termbox\n");
        goto cleanup;
    }
    tb_clear();

    for (int i = 0; i < doc.header.element_count; i++) {
        if (!elements[i].parent) {
            render_element(&elements[i], 0, 0, debug_file);
        }
    }
    tb_present();

    struct tb_event ev;
    tb_poll_event(&ev);
    tb_shutdown();

cleanup:
    for (int i = 0; i < doc.header.element_count; i++) {
        free(elements[i].text);
    }
    free(elements);
    krb_free_document(&doc);
    fclose(file);
    if (debug_file != stderr) fclose(debug_file);
    return 0;
}