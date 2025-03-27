#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <termbox.h>
#include "krb.h"

#define MAX_ELEMENTS 256

typedef struct RenderElement {
    KrbElementHeader header;
    char* text;
    uint32_t bg_color;
    uint32_t fg_color;
    uint32_t border_color;
    uint8_t border_widths[4];
    uint8_t text_alignment; // NEW: 0=left, 1=center, 2=right
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

    if (a < 128) return TB_DEFAULT;

    if (r > 200 && g > 200 && b > 200) return TB_WHITE;
    if (r > 200 && g < 100 && b < 100) return TB_RED;
    if (r < 100 && g > 200 && b < 100) return TB_GREEN;
    if (r < 100 && g < 100 && b > 100) return TB_BLUE;
    if (r > 200 && g > 200 && b < 100) return TB_YELLOW;
    if (r > 150 && g < 100 && b > 150) return TB_MAGENTA;
    if (r < 100 && g > 200 && b > 200) return TB_CYAN;
    if (r < 50 && g < 50 && b < 50) return TB_BLACK;

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

    if (el->header.type == 0x02 && el->text) { // Auto-size Text properly
        width = el->header.width == 0 ? strlen(el->text) + 2 : width;
        height = el->header.height == 0 ? 3 : height;
    } else {
        if (width < 5) width = 5;
        if (height < 3) height = 3;
    }

    if (el->bg_color == 0 && el->parent) el->bg_color = el->parent->bg_color;
    if (el->fg_color == 0 && el->parent) el->fg_color = el->parent->fg_color;
    if (el->border_color == 0 && el->parent) el->border_color = el->parent->border_color;
    if (el->bg_color == 0) el->bg_color = 0x000000FF;
    if (el->fg_color == 0) el->fg_color = 0xFFFFFFFF;
    if (el->border_color == 0) el->border_color = 0x808080FF;

    int width_term = tb_width();
    int height_term = tb_height();
    if (x >= width_term || y >= height_term) {
        fprintf(debug_file, "WARNING: Element at (%d, %d) outside bounds (%d, %d)\n", x, y, width_term, height_term);
        return;
    }

    int bg_color = rgb_to_tb_color(el->bg_color, debug_file);
    int fg_color = rgb_to_tb_color(el->fg_color, debug_file);
    int border_color = rgb_to_tb_color(el->border_color, debug_file);

    fprintf(debug_file, "DEBUG: Rendering element type=0x%02X at (%d, %d), size=%dx%d, text=%s, bg=0x%08X (%d), fg=0x%08X (%d), border=0x%08X (%d), layout=0x%02X\n",
            el->header.type, x, y, width, height, el->text ? el->text : "NULL", el->bg_color, bg_color, el->fg_color, fg_color, el->border_color, border_color, el->header.layout);

    if (el->header.type == 0x00) { // App
        for (int i = 0; i < el->child_count; i++) {
            render_element(el->children[i], x, y, debug_file);
        }
    } else if (el->header.type == 0x01) { // Container
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
                    tb_change_cell(cur_x, cur_y, ' ', fg_color, bg_color);
                }
            }
        }

        int content_x = x + el->border_widths[3];
        int content_y = y + el->border_widths[0];
        int content_width = width - el->border_widths[3] - el->border_widths[1];
        int content_height = height - el->border_widths[0] - el->border_widths[2];
        uint8_t alignment = (el->header.layout >> 2) & 0x03;
        uint8_t direction = el->header.layout & 0x03;

        if (alignment == 0x01 && direction == 0x01) { // Center, Column
            int total_child_height = 0;
            for (int i = 0; i < el->child_count; i++) {
                int child_height = el->children[i]->header.height / 10 ? el->children[i]->header.height / 10 : (el->children[i]->text ? 3 : 3);
                total_child_height += child_height;
            }
            int start_y = content_y + (content_height - total_child_height) / 2;
            for (int i = 0; i < el->child_count; i++) {
                int child_width = el->children[i]->header.width / 10 ? el->children[i]->header.width / 10 : (el->children[i]->text ? strlen(el->children[i]->text) + 2 : 5);
                int child_x = content_x + (content_width - child_width) / 2;
                render_element(el->children[i], child_x, start_y, debug_file);
                start_y += (el->children[i]->header.height / 10 ? el->children[i]->header.height / 10 : 3);
            }
        } else {
            for (int i = 0; i < el->child_count; i++) {
                render_element(el->children[i], content_x, content_y, debug_file);
            }
        }
    } else if (el->header.type == 0x02 && el->text) { // Text
        int text_len = strlen(el->text);
        int text_x;
        switch (el->text_alignment) {
            case 1: // Center
                text_x = x + (width - text_len) / 2;
                break;
            case 2: // Right
                text_x = x + width - text_len - 1;
                break;
            case 0: // Left
            default:
                text_x = x + 1;
                break;
        }
        int text_y = y + (height - 1) / 2;

        fprintf(debug_file, "DEBUG: Rendering text '%s' at (%d, %d) with fg=0x%08X (%d), bg=0x%08X (%d), alignment=%d\n", 
                el->text, text_x, text_y, el->fg_color, fg_color, el->bg_color, bg_color, el->text_alignment);

        if (text_x < width_term && text_y < height_term) {
            for (int i = 0; i < width && x + i < width_term; i++) {
                tb_change_cell(x + i, text_y, ' ', fg_color, bg_color);
            }
            for (int i = 0; i < text_len && text_x + i < width_term; i++) {
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
    RenderElement* app_element = NULL;

    for (int i = 0; i < doc.header.element_count; i++) {
        elements[i].header = doc.elements[i];
        elements[i].text_alignment = 0; // Default to left

        if (elements[i].header.type == 0x00) {
            app_element = &elements[i];
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
            } else if (prop->property_id == 0x0B && prop->value_type == 0x09 && prop->size == 1) {
                elements[i].text_alignment = *(uint8_t*)prop->value;
            } else if (elements[i].header.type == 0x00) {
                if (prop->property_id == 0x20 && prop->value_type == 0x02 && prop->size == 2) {
                    uint16_t window_width = *(uint16_t*)prop->value;
                    fprintf(debug_file, "DEBUG: App WindowWidth=%d (not applied in Termbox)\n", window_width);
                } else if (prop->property_id == 0x21 && prop->value_type == 0x02 && prop->size == 2) {
                    uint16_t window_height = *(uint16_t*)prop->value;
                    fprintf(debug_file, "DEBUG: App WindowHeight=%d (not applied in Termbox)\n", window_height);
                } else if (prop->property_id == 0x22 && prop->value_type == 0x04 && prop->size == 1) {
                    uint8_t string_index = *(uint8_t*)prop->value;
                    if (string_index < doc.header.string_count) {
                        fprintf(debug_file, "DEBUG: App WindowTitle='%s' (not applied in Termbox)\n", doc.strings[string_index]);
                    }
                } else if (prop->property_id == 0x23 && prop->value_type == 0x01 && prop->size == 1) {
                    uint8_t resizable = *(uint8_t*)prop->value;
                    fprintf(debug_file, "DEBUG: App Resizable=%d (not applied in Termbox)\n", resizable);
                } else if (prop->property_id == 0x24 && prop->value_type == 0x01 && prop->size == 1) {
                    uint8_t keep_aspect = *(uint8_t*)prop->value;
                    fprintf(debug_file, "DEBUG: App KeepAspect=%d (not applied in Termbox)\n", keep_aspect);
                } else if (prop->property_id == 0x25 && prop->value_type == 0x06 && prop->size == 2) {
                    uint16_t scale_factor = *(uint16_t*)prop->value;
                    float scale = scale_factor / 256.0f;
                    fprintf(debug_file, "DEBUG: App ScaleFactor=%.2f (not applied in Termbox)\n", scale);
                } else if (prop->property_id == 0x26 && prop->value_type == 0x05 && prop->size == 1) {
                    uint8_t icon_index = *(uint8_t*)prop->value;
                    if (icon_index < doc.header.string_count) {
                        fprintf(debug_file, "DEBUG: App Icon='%s' (not applied in Termbox)\n", doc.strings[icon_index]);
                    }
                } else if (prop->property_id == 0x27 && prop->value_type == 0x04 && prop->size == 1) {
                    uint8_t version_index = *(uint8_t*)prop->value;
                    if (version_index < doc.header.string_count) {
                        fprintf(debug_file, "DEBUG: App Version='%s'\n", doc.strings[version_index]);
                    }
                } else if (prop->property_id == 0x28 && prop->value_type == 0x04 && prop->size == 1) {
                    uint8_t author_index = *(uint8_t*)prop->value;
                    if (author_index < doc.header.string_count) {
                        fprintf(debug_file, "DEBUG: App Author='%s'\n", doc.strings[author_index]);
                    }
                }
            }
        }
    }

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
