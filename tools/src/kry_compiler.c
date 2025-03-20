#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#define MAX_ELEMENTS 256
#define MAX_STRINGS 256
#define MAX_PROPERTIES 16
#define MAX_STYLES 16

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
    uint16_t child_offset; // Added for child offset in .krb
} KrbElementHeader;

typedef struct {
    uint8_t property_id;
    uint8_t value_type;
    uint8_t size;
    void* value;
} KrbProperty;

typedef struct {
    char* text;
    uint8_t index;
} StringEntry;

typedef struct {
    char* name;
    uint8_t id;
    KrbProperty properties[MAX_PROPERTIES];
    uint8_t property_count;
} StyleEntry;

typedef struct Element {
    KrbElementHeader header;
    KrbProperty properties[MAX_PROPERTIES];
    struct Element* children[MAX_ELEMENTS];
    int child_count;
    struct Element* parent;
} Element;

void write_u16(FILE* file, uint16_t value) {
    fputc(value & 0xFF, file);
    fputc((value >> 8) & 0xFF, file);
}

void write_u32(FILE* file, uint32_t value) {
    fputc(value & 0xFF, file);
    fputc((value >> 8) & 0xFF, file);
    fputc((value >> 16) & 0xFF, file);
    fputc((value >> 24) & 0xFF, file);
}

int compile_kry_to_krb(const char* input_file, const char* output_file) {
    FILE* in = fopen(input_file, "r");
    if (!in) {
        printf("Error: Could not open input file %s\n", input_file);
        return 1;
    }

    FILE* out = fopen(output_file, "wb");
    if (!out) {
        printf("Error: Could not open output file %s\n", output_file);
        fclose(in);
        return 1;
    }

    Element elements[MAX_ELEMENTS] = {0};
    StringEntry strings[MAX_STRINGS] = {0};
    StyleEntry styles[MAX_STYLES] = {0};
    int element_count = 0;
    int string_count = 0;
    int style_count = 0;

    char line[256];
    Element* current_element = NULL;
    StyleEntry* current_style = NULL;
    int indent_level = 0;

    while (fgets(line, sizeof(line), in)) {
        char* trimmed = line;
        int indent = 0;
        while (*trimmed == ' ' || *trimmed == '\t') {
            indent += (*trimmed == '\t' ? 4 : 1);
            trimmed++;
        }
        if (*trimmed == '\0' || *trimmed == '\n') continue;

        if (strncmp(trimmed, "style ", 6) == 0) {
            if (style_count >= MAX_STYLES) {
                printf("Error: Too many styles\n");
                fclose(in);
                fclose(out);
                return 1;
            }
            char style_name[64];
            sscanf(trimmed, "style \"%63[^\"]\" {", style_name);
            current_style = &styles[style_count++];
            current_style->name = strdup(style_name);
            current_style->id = style_count;
            indent_level = indent;
        }
        else if (current_style && indent > indent_level && strncmp(trimmed, "}", 1) != 0) {
            char key[64], value[128];
            if (sscanf(trimmed, "%63[^:]: %127[^\n]", key, value) == 2) {
                if (strcmp(key, "border_width") == 0) {
                    current_style->properties[current_style->property_count].property_id = 0x04;
                    current_style->properties[current_style->property_count].value_type = 0x01;
                    current_style->properties[current_style->property_count].size = 1;
                    current_style->properties[current_style->property_count].value = malloc(1);
                    *(uint8_t*)current_style->properties[current_style->property_count].value = atoi(value);
                    current_style->property_count++;
                }
                else if (strcmp(key, "background_color") == 0) {
                    uint8_t r, g, b, a = 255;
                    if (sscanf(value, "#%2hhx%2hhx%2hhx", &r, &g, &b) == 3 ||
                        sscanf(value, "#%2hhx%2hhx%2hhx%2hhx", &r, &g, &b, &a) == 4) {
                        current_style->properties[current_style->property_count].property_id = 0x01;
                        current_style->properties[current_style->property_count].value_type = 0x03;
                        current_style->properties[current_style->property_count].size = 4;
                        current_style->properties[current_style->property_count].value = malloc(4);
                        uint8_t* color = current_style->properties[current_style->property_count].value;
                        color[0] = r; color[1] = g; color[2] = b; color[3] = a;
                        current_style->property_count++;
                    }
                }
            }
        }
        else if (current_style && indent <= indent_level && strncmp(trimmed, "}", 1) == 0) {
            current_style = NULL;
            indent_level = 0;
        }
        else if (strncmp(trimmed, "Container {", 11) == 0 || strncmp(trimmed, "Text {", 6) == 0) {
            if (element_count >= MAX_ELEMENTS) {
                printf("Error: Too many elements\n");
                fclose(in);
                fclose(out);
                return 1;
            }
            current_element = &elements[element_count++];
            current_element->header.type = (strncmp(trimmed, "Container", 9) == 0) ? 0x01 : 0x02;
            current_element->header.id = 0;
            current_element->header.pos_x = 0;
            current_element->header.pos_y = 0;
            current_element->header.width = 0;
            current_element->header.height = 0;
            current_element->header.layout = 0;
            current_element->header.style_id = 0;
            if (indent > 0 && element_count > 1) {
                Element* parent = &elements[element_count - 2];
                parent->children[parent->child_count++] = current_element;
                current_element->parent = parent;
                parent->header.child_count++;
            }
            indent_level = indent;
        }
        else if (current_element && indent <= indent_level && strncmp(trimmed, "}", 1) == 0) {
            current_element = NULL;
            indent_level = 0;
        }
        else if (current_element && indent > indent_level) {
            char key[64], value[128];
            if (sscanf(trimmed, "%63[^:]: %127[^\n]", key, value) == 2) {
                if (strcmp(key, "pos_x") == 0) current_element->header.pos_x = atoi(value);
                else if (strcmp(key, "pos_y") == 0) current_element->header.pos_y = atoi(value);
                else if (strcmp(key, "width") == 0) current_element->header.width = atoi(value);
                else if (strcmp(key, "height") == 0) current_element->header.height = atoi(value);
                else if (strcmp(key, "style") == 0) {
                    for (int i = 0; i < style_count; i++) {
                        if (strcmp(styles[i].name, value) == 0) {
                            current_element->header.style_id = styles[i].id;
                            break;
                        }
                    }
                }
                else if (strcmp(key, "text") == 0) {
                    if (string_count >= MAX_STRINGS) {
                        printf("Error: Too many strings\n");
                        fclose(in);
                        fclose(out);
                        return 1;
                    }
                    strings[string_count].text = strdup(value);
                    strings[string_count].index = string_count;
                    current_element->properties[current_element->header.property_count].property_id = 0x08;
                    current_element->properties[current_element->header.property_count].value_type = 0x04;
                    current_element->properties[current_element->header.property_count].size = 1;
                    current_element->properties[current_element->header.property_count].value = malloc(1);
                    *(uint8_t*)current_element->properties[current_element->header.property_count].value = string_count;
                    current_element->header.property_count++;
                    string_count++;
                }
            }
        }
    }

    // Calculate child offsets
    uint32_t current_offset = 38; // After header
    for (int i = 0; i < element_count; i++) {
        elements[i].header.child_offset = 0;
        if (elements[i].header.child_count > 0) {
            current_offset += 16 + elements[i].header.property_count * (3 + elements[i].properties->size);
            elements[i].header.child_offset = current_offset - 38; // Relative to start of elements
        }
        current_offset += 16 + elements[i].header.property_count * (3 + elements[i].properties->size);
    }

    // Write Header
    fwrite("KRB1", 1, 4, out);
    write_u16(out, 0x0001);
    write_u16(out, style_count > 0 ? 0x0001 : 0x0000); // Flags: has styles if any
    write_u16(out, element_count);
    write_u16(out, style_count);
    write_u16(out, 0);
    write_u16(out, string_count);
    write_u16(out, 0);
    uint32_t element_offset = 38;
    write_u32(out, element_offset);
    uint32_t style_offset = element_offset;
    for (int i = 0; i < element_count; i++) {
        style_offset += 16 + elements[i].header.property_count * (3 + elements[i].properties->size);
    }
    write_u32(out, style_count > 0 ? style_offset : 0);
    write_u32(out, 0);
    uint32_t string_offset = style_offset;
    for (int i = 0; i < style_count; i++) {
        string_offset += 3 + styles[i].property_count * (3 + styles[i].properties->size);
    }
    write_u32(out, string_offset);
    uint32_t total_size = string_offset + 2;
    for (int i = 0; i < string_count; i++) {
        total_size += 1 + strlen(strings[i].text);
    }
    write_u32(out, total_size);

    // Write Elements
    for (int i = 0; i < element_count; i++) {
        Element* el = &elements[i];
        fputc(el->header.type, out);
        fputc(el->header.id, out);
        write_u16(out, el->header.pos_x);
        write_u16(out, el->header.pos_y);
        write_u16(out, el->header.width);
        write_u16(out, el->header.height);
        fputc(el->header.layout, out);
        fputc(el->header.style_id, out);
        fputc(el->header.property_count, out);
        fputc(el->header.child_count, out);
        fputc(el->header.event_count, out);
        fputc(el->header.animation_count, out);
        write_u16(out, el->header.child_offset);
        for (int j = 0; j < el->header.property_count; j++) {
            fputc(el->properties[j].property_id, out);
            fputc(el->properties[j].value_type, out);
            fputc(el->properties[j].size, out);
            fwrite(el->properties[j].value, 1, el->properties[j].size, out);
        }
    }

    // Write Styles
    for (int i = 0; i < style_count; i++) {
        StyleEntry* st = &styles[i];
        fputc(st->id, out);
        fputc(st->name ? st->id - 1 : 0, out); // String index (assuming style names are in order)
        fputc(st->property_count, out);
        for (int j = 0; j < st->property_count; j++) {
            fputc(st->properties[j].property_id, out);
            fputc(st->properties[j].value_type, out);
            fputc(st->properties[j].size, out);
            fwrite(st->properties[j].value, 1, st->properties[j].size, out);
        }
    }

    // Write String Table
    write_u16(out, string_count);
    for (int i = 0; i < string_count; i++) {
        size_t len = strlen(strings[i].text);
        fputc(len, out);
        fwrite(strings[i].text, 1, len, out);
    }

    // Cleanup
    for (int i = 0; i < element_count; i++) {
        for (int j = 0; j < elements[i].header.property_count; j++) {
            free(elements[i].properties[j].value);
        }
    }
    for (int i = 0; i < style_count; i++) {
        free(styles[i].name);
        for (int j = 0; j < styles[i].property_count; j++) {
            free(styles[i].properties[j].value);
        }
    }
    for (int i = 0; i < string_count; i++) {
        free(strings[i].text);
    }
    fclose(in);
    fclose(out);
    return 0;
}

int main(int argc, char* argv[]) {
    if (argc != 3) {
        printf("Usage: %s <input.kry> <output.krb>\n", argv[0]);
        return 1;
    }
    return compile_kry_to_krb(argv[1], argv[2]);
}