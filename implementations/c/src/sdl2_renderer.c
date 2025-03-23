#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <SDL2/SDL.h>
#include <SDL2/SDL_ttf.h>
#include "krb.h"

#define MAX_ELEMENTS 256
#define WINDOW_WIDTH 800
#define WINDOW_HEIGHT 600
#define SCALE_FACTOR 2

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

SDL_Color rgba_to_sdl_color(uint32_t rgba) {
    uint8_t r = (rgba >> 24) & 0xFF;
    uint8_t g = (rgba >> 16) & 0xFF;
    uint8_t b = (rgba >> 8) & 0xFF;
    uint8_t a = rgba & 0xFF;
    return (SDL_Color){r, g, b, a};
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

void render_element(SDL_Renderer* renderer, TTF_Font* font, RenderElement* el, int parent_x, int parent_y, FILE* debug_file) {
    int x = parent_x + (el->header.pos_x * SCALE_FACTOR);
    int y = parent_y + (el->header.pos_y * SCALE_FACTOR);
    int width = (el->header.width * SCALE_FACTOR);
    int height = (el->header.height * SCALE_FACTOR);
    if (width < 10) width = 10;
    if (height < 6) height = 6;

    if (el->bg_color == 0 && el->parent) el->bg_color = el->parent->bg_color;
    if (el->fg_color == 0 && el->parent) el->fg_color = el->parent->fg_color;
    if (el->border_color == 0 && el->parent) el->border_color = el->parent->border_color;
    if (el->bg_color == 0) el->bg_color = 0x000000FF;     // Default black
    if (el->fg_color == 0) el->fg_color = 0xFFFFFFFF;     // Default white
    if (el->border_color == 0) el->border_color = 0x808080FF; // Default gray

    SDL_Color bg_color = rgba_to_sdl_color(el->bg_color);
    SDL_Color fg_color = rgba_to_sdl_color(el->fg_color);
    SDL_Color border_color = rgba_to_sdl_color(el->border_color);

    fprintf(debug_file, "DEBUG: Rendering element type=0x%02X at (%d, %d), size=%dx%d, text=%s\n",
            el->header.type, x, y, width, height, el->text ? el->text : "NULL");

    if (el->header.type == 0x01) { // Container
        // Draw background
        SDL_SetRenderDrawColor(renderer, bg_color.r, bg_color.g, bg_color.b, bg_color.a);
        SDL_Rect bg_rect = {x, y, width, height};
        SDL_RenderFillRect(renderer, &bg_rect);

        // Draw borders
        int top = el->border_widths[0] * SCALE_FACTOR;
        int right = el->border_widths[1] * SCALE_FACTOR;
        int bottom = el->border_widths[2] * SCALE_FACTOR;
        int left = el->border_widths[3] * SCALE_FACTOR;

        SDL_SetRenderDrawColor(renderer, border_color.r, border_color.g, border_color.b, border_color.a);
        if (top > 0) {
            SDL_Rect top_rect = {x, y, width, top};
            SDL_RenderFillRect(renderer, &top_rect);
        }
        if (bottom > 0) {
            SDL_Rect bottom_rect = {x, y + height - bottom, width, bottom};
            SDL_RenderFillRect(renderer, &bottom_rect);
        }
        if (left > 0) {
            SDL_Rect left_rect = {x, y, left, height};
            SDL_RenderFillRect(renderer, &left_rect);
        }
        if (right > 0) {
            SDL_Rect right_rect = {x + width - right, y, right, height};
            SDL_RenderFillRect(renderer, &right_rect);
        }

        // Render children
        for (int i = 0; i < el->child_count; i++) {
            render_element(renderer, font, el->children[i], x + left, y + top, debug_file);
        }
    } else if (el->header.type == 0x02 && el->text) { // Text
        int text_x = x + (2 * SCALE_FACTOR);
        int text_y = y + (2 * SCALE_FACTOR);

        // Draw text background
        SDL_SetRenderDrawColor(renderer, bg_color.r, bg_color.g, bg_color.b, bg_color.a);
        SDL_Rect bg_rect = {x, y, width, height};
        SDL_RenderFillRect(renderer, &bg_rect);

        // Draw text
        SDL_Surface* text_surface = TTF_RenderText_Solid(font, el->text, fg_color);
        if (text_surface) {
            SDL_Texture* text_texture = SDL_CreateTextureFromSurface(renderer, text_surface);
            if (text_texture) {
                SDL_Rect text_rect = {text_x, text_y, text_surface->w, text_surface->h};
                SDL_RenderCopy(renderer, text_texture, NULL, &text_rect);
                SDL_DestroyTexture(text_texture);
            }
            SDL_FreeSurface(text_surface);
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

    // Initialize SDL
    if (SDL_Init(SDL_INIT_VIDEO) < 0) {
        fprintf(debug_file, "ERROR: SDL could not initialize! SDL_Error: %s\n", SDL_GetError());
        goto cleanup;
    }

    if (TTF_Init() == -1) {
        fprintf(debug_file, "ERROR: SDL_ttf could not initialize! TTF_Error: %s\n", TTF_GetError());
        SDL_Quit();
        goto cleanup;
    }

    SDL_Window* window = SDL_CreateWindow("KRB Renderer (SDL2)",
                                        SDL_WINDOWPOS_UNDEFINED,
                                        SDL_WINDOWPOS_UNDEFINED,
                                        WINDOW_WIDTH, WINDOW_HEIGHT,
                                        SDL_WINDOW_SHOWN);
    if (!window) {
        fprintf(debug_file, "ERROR: Window could not be created! SDL_Error: %s\n", SDL_GetError());
        TTF_Quit();
        SDL_Quit();
        goto cleanup;
    }

    SDL_Renderer* renderer = SDL_CreateRenderer(window, -1, SDL_RENDERER_ACCELERATED);
    if (!renderer) {
        fprintf(debug_file, "ERROR: Renderer could not be created! SDL_Error: %s\n", SDL_GetError());
        SDL_DestroyWindow(window);
        TTF_Quit();
        SDL_Quit();
        goto cleanup;
    }

    TTF_Font* font = TTF_OpenFont("DejaVuSans.ttf", 20); // You'll need a font file
    if (!font) {
        fprintf(debug_file, "ERROR: Failed to load font! TTF_Error: %s\n", TTF_GetError());
        SDL_DestroyRenderer(renderer);
        SDL_DestroyWindow(window);
        TTF_Quit();
        SDL_Quit();
        goto cleanup;
    }

    SDL_bool running = SDL_TRUE;
    SDL_Event event;

    while (running) {
        while (SDL_PollEvent(&event)) {
            if (event.type == SDL_QUIT) {
                running = SDL_FALSE;
            }
        }

        SDL_SetRenderDrawColor(renderer, 0, 0, 0, 255); // Black background
        SDL_RenderClear(renderer);

        // Render all root elements
        for (int i = 0; i < doc.header.element_count; i++) {
            if (!elements[i].parent) {
                render_element(renderer, font, &elements[i], 0, 0, debug_file);
            }
        }

        SDL_RenderPresent(renderer);
    }

    // Cleanup SDL
    TTF_CloseFont(font);
    SDL_DestroyRenderer(renderer);
    SDL_DestroyWindow(window);
    TTF_Quit();
    SDL_Quit();

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