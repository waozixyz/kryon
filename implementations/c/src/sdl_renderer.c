#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <SDL2/SDL.h>
#include <SDL2/SDL_ttf.h>
#include "krb.h"

#define WINDOW_WIDTH 800
#define WINDOW_HEIGHT 600

typedef struct {
    KrbElementHeader header;
    char* text;              // For TextContent property
} KrbElement;

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

    // Read header
    KrbHeader header;
    if (!read_header(file, &header)) {
        fclose(file);
        return 1;
    }

    // Allocate elements
    KrbElement* elements = malloc(header.element_count * sizeof(KrbElement));
    if (!elements) {
        printf("Error: Memory allocation failed\n");
        fclose(file);
        return 1;
    }

    // Seek to elements and read them
    fseek(file, header.element_offset, SEEK_SET);
    for (int i = 0; i < header.element_count; i++) {
        read_element_header(file, &elements[i].header);
        elements[i].text = NULL;

        for (int j = 0; j < elements[i].header.property_count; j++) {
            KrbProperty prop;
            read_property(file, &prop);

            if (prop.property_id == 0x08 && prop.value_type == 0x04) { // TextContent, String
                uint8_t string_index = prop.value[0];
                long current_pos = ftell(file);
                fseek(file, header.string_offset, SEEK_SET);
                uint16_t string_count;
                fread(&string_count, 2, 1, file);

                if (string_index < string_count) {
                    for (int k = 0; k <= string_index; k++) {
                        uint8_t length;
                        fread(&length, 1, 1, file);
                        if (k == string_index) {
                            elements[i].text = malloc(length + 1);
                            fread(elements[i].text, length, 1, file);
                            elements[i].text[length] = '\0';
                        } else {
                            fseek(file, length, SEEK_CUR);
                        }
                    }
                }
                fseek(file, current_pos, SEEK_SET);
            }
            free(prop.value);
        }
    }

    // Initialize SDL2
    if (SDL_Init(SDL_INIT_VIDEO) < 0) {
        printf("SDL_Error: %s\n", SDL_GetError());
        goto cleanup;
    }

    // Initialize SDL_ttf
    if (TTF_Init() < 0) {
        printf("TTF_Error: %s\n", TTF_GetError());
        SDL_Quit();
        goto cleanup;
    }

    SDL_Window* window = SDL_CreateWindow("Kryon SDL2 Renderer",
                                          SDL_WINDOWPOS_UNDEFINED, SDL_WINDOWPOS_UNDEFINED,
                                          WINDOW_WIDTH, WINDOW_HEIGHT,
                                          SDL_WINDOW_SHOWN);
    if (!window) {
        printf("SDL_Error: %s\n", SDL_GetError());
        goto cleanup_ttf;
    }

    SDL_Renderer* renderer = SDL_CreateRenderer(window, -1, SDL_RENDERER_ACCELERATED);
    if (!renderer) {
        printf("SDL_Error: %s\n", SDL_GetError());
        goto cleanup_window;
    }

    TTF_Font* font = TTF_OpenFont("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 16);
    if (!font) {
        printf("TTF_Error: %s\n", TTF_GetError());
        goto cleanup_renderer;
    }

    // Main loop
    SDL_Event event;
    int running = 1;
    while (running) {
        while (SDL_PollEvent(&event)) {
            if (event.type == SDL_QUIT) running = 0;
        }

        SDL_SetRenderDrawColor(renderer, 255, 255, 255, 255); // White background
        SDL_RenderClear(renderer);

        for (int i = 0; i < header.element_count; i++) {
            if (elements[i].header.type == 0x02 && elements[i].text) { // Text element
                SDL_Color color = {0, 0, 0, 255}; // Black text
                SDL_Surface* surface = TTF_RenderText_Solid(font, elements[i].text, color);
                SDL_Texture* texture = SDL_CreateTextureFromSurface(renderer, surface);
                SDL_Rect rect = {elements[i].header.pos_x, elements[i].header.pos_y,
                                 elements[i].header.width, elements[i].header.height};
                SDL_RenderCopy(renderer, texture, NULL, &rect);
                SDL_FreeSurface(surface);
                SDL_DestroyTexture(texture);
            }
        }

        SDL_RenderPresent(renderer);
    }

    // Cleanup
    TTF_CloseFont(font);
cleanup_renderer:
    SDL_DestroyRenderer(renderer);
cleanup_window:
    SDL_DestroyWindow(window);
cleanup_ttf:
    TTF_Quit();
    SDL_Quit();
cleanup:
    for (int i = 0; i < header.element_count; i++) {
        if (elements[i].text) free(elements[i].text);
    }
    free(elements);
    fclose(file);
    return 0;
}