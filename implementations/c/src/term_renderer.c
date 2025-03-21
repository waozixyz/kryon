#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <termbox.h>
#include "krb.h"

#define MAX_ELEMENTS 256

typedef struct RenderElement {
    KrbElementHeader header;
    char* text;
    uint32_t bg_color; // RGBA
    struct RenderElement* parent;
    struct RenderElement* children[MAX_ELEMENTS];
    int child_count;
} RenderElement;

void dump_bytes(const void* data, size_t size, FILE* debug_file) {
    const unsigned char* bytes = (const unsigned char*)data;
    for (size_t i = 0; i < size; i++) {
        fprintf(debug_file, "%02X ", bytes[i]);
        if ((i + 1) % 16 == 0) fprintf(debug_file, "\n");
    }
    fprintf(debug_file, "\n");
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

// Convert RGBA to termbox colors (approximate)
int rgb_to_tb_color(uint32_t rgba) {
    uint8_t r = (rgba >> 24) & 0xFF;
    uint8_t g = (rgba >> 16) & 0xFF;
    uint8_t b = (rgba >> 8) & 0xFF;
    
    // Simple conversion to closest termbox color
    if (r > 200 && g > 200 && b > 200) return TB_WHITE;
    if (r > 200 && g < 100 && b < 100) return TB_RED;
    if (r < 100 && g > 200 && b < 100) return TB_GREEN;
    if (r < 100 && g < 100 && b > 200) return TB_BLUE;
    if (r > 200 && g > 200 && b < 100) return TB_YELLOW;
    if (r > 200 && g < 100 && b > 200) return TB_MAGENTA;
    if (r < 100 && g > 200 && b > 200) return TB_CYAN;
    if (r < 100 && g < 100 && b < 100) return TB_BLACK;
    
    return TB_DEFAULT;
}

void render_element(RenderElement* el, int parent_x, int parent_y, FILE* debug_file) {
    // Calculate absolute position
    int x = parent_x + el->header.pos_x / 30;
    int y = parent_y + el->header.pos_y / 30;
    int width = el->header.width / 20;
    int height = el->header.height / 30;
    
    // Ensure minimum size
    if (width < 5) width = 5;
    if (height < 3) height = 3;
    
    fprintf(debug_file, "DEBUG: Rendering element type=%d at (%d, %d), size=%dx%d\n",
            el->header.type, x, y, width, height);
    
    // Get terminal dimensions
    int width_term = tb_width();
    int height_term = tb_height();
    
    // Check if element is outside terminal bounds
    if (x >= width_term || y >= height_term) {
        fprintf(debug_file, "WARNING: Element at (%d, %d) is outside terminal bounds\n", x, y);
        return;
    }
    
    // Convert background color
    int bg_color = rgb_to_tb_color(el->bg_color);
    
    if (el->header.type == 0x01) {
        // Draw box
        for (int i = 0; i < width; i++) {
            for (int j = 0; j < height; j++) {
                int cur_x = x + i;
                int cur_y = y + j;
                
                if (cur_x >= width_term || cur_y >= height_term)
                    continue;
                
                if (i == 0 && j == 0) {
                    // Top-left corner
                    tb_change_cell(cur_x, cur_y, '+', TB_WHITE, bg_color);
                } else if (i == width-1 && j == 0) {
                    // Top-right corner
                    tb_change_cell(cur_x, cur_y, '+', TB_WHITE, bg_color);
                } else if (i == 0 && j == height-1) {
                    // Bottom-left corner
                    tb_change_cell(cur_x, cur_y, '+', TB_WHITE, bg_color);
                } else if (i == width-1 && j == height-1) {
                    // Bottom-right corner
                    tb_change_cell(cur_x, cur_y, '+', TB_WHITE, bg_color);
                } else if (i == 0 || i == width-1) {
                    // Vertical borders
                    tb_change_cell(cur_x, cur_y, '|', TB_WHITE, bg_color);
                } else if (j == 0 || j == height-1) {
                    // Horizontal borders
                    tb_change_cell(cur_x, cur_y, '-', TB_WHITE, bg_color);
                } else {
                    // Interior
                    tb_change_cell(cur_x, cur_y, ' ', TB_DEFAULT, bg_color);
                }
            }
        }
    }
    
    // Draw text
    if (el->text) {
        int text_x = x + 1;
        int text_y = y + 1;
        if (text_x < width_term && text_y < height_term) {
            const char* text = el->text;
            for (size_t i = 0; i < strlen(text) && text_x + i < width_term; i++) {
                tb_change_cell(text_x + i, text_y, text[i], TB_WHITE, bg_color);
            }
        }
    }
    
    // Render children
    for (int i = 0; i < el->child_count; i++) {
        render_element(el->children[i], x, y, debug_file);
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
    }
    
    FILE* file = fopen(argv[1], "rb");
    if (!file) {
        fprintf(debug_file, "Error: Could not open file %s\n", argv[1]);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }
    
    // Read file size for debug
    fseek(file, 0, SEEK_END);
    long file_size = ftell(file);
    fseek(file, 0, SEEK_SET);
    fprintf(debug_file, "DEBUG: File size: %ld bytes\n", file_size);
    
    // Parse KRB document
    KrbDocument doc = {0};
    if (!krb_read_document(file, &doc)) {
        fprintf(debug_file, "ERROR: Failed to parse KRB document\n");
        fclose(file);
        if (debug_file != stderr) fclose(debug_file);
        return 1;
    }
    
    // Create render elements
    RenderElement* elements = calloc(doc.header.element_count, sizeof(RenderElement));
    for (int i = 0; i < doc.header.element_count; i++) {
        elements[i].header = doc.elements[i];
        for (int j = 0; j < doc.elements[i].property_count; j++) {
            KrbProperty* prop = &doc.properties[i][j];
            if (prop->property_id == 0x08 && prop->value_type == 0x04 && prop->size == 1 && doc.strings) {
                uint8_t string_index = *(uint8_t*)prop->value;
                if (string_index < doc.header.string_count && doc.strings[string_index]) {
                    elements[i].text = strip_quotes(doc.strings[string_index]);
                    fprintf(debug_file, "DEBUG: Element %d text: '%s'\n", i, elements[i].text);
                }
            }
            if (prop->property_id == 0x01 && prop->value_type == 0x03 && prop->size == 4) {
                elements[i].bg_color = *(uint32_t*)prop->value;
            }
        }
    }
    
    // Set up parent-child relationships
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
    
    // Initialize termbox
    int tb_init_result = tb_init();
    if (tb_init_result != 0) {
        fprintf(debug_file, "ERROR: Failed to initialize termbox: %d\n", tb_init_result);
        goto cleanup;
    }
    
    // Set input mode
    tb_select_input_mode(TB_INPUT_ESC);
    
    // Clear screen
    tb_clear();
    
    // Draw a title
    const char* title = "KRB Renderer";
    int title_len = strlen(title);
    int title_x = (tb_width() - title_len) / 2;
    for (int i = 0; i < title_len; i++) {
        tb_change_cell(title_x + i, 0, title[i], TB_WHITE | TB_BOLD, TB_DEFAULT);
    }
    
    // Render elements
    for (int i = 0; i < doc.header.element_count; i++) {
        if (!elements[i].parent) {
            render_element(&elements[i], 2, 2, debug_file); // Start with a small offset
        }
    }
    
    // Present the screen
    tb_present();
    
    // Wait for key press
    struct tb_event ev;
    tb_poll_event(&ev);
    
    // Shutdown termbox
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