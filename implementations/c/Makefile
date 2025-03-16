CC = gcc
CFLAGS = -Wall -g -Iinclude
LDFLAGS_SDL = -lSDL2 -lSDL2_ttf
LDFLAGS_TERM = # No extra libs for terminal

# Directories
SRC_DIR = src
BIN_DIR = bin

# Source files
READER_SRC = $(SRC_DIR)/krb_reader.c
SDL_RENDERER_SRC = $(SRC_DIR)/sdl_renderer.c
TERM_RENDERER_SRC = $(SRC_DIR)/term_renderer.c

# Default renderer
RENDERER ?= sdl

# Targets
all: $(BIN_DIR)/krb_renderer

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# Renderer-specific targets
$(BIN_DIR)/krb_renderer: $(READER_SRC) $(SRC_DIR)/$(RENDERER)_renderer.c | $(BIN_DIR)
ifeq ($(RENDERER),sdl)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS_SDL)
else ifeq ($(RENDERER),term)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS_TERM)
else
	@echo "Error: Unknown renderer '$(RENDERER)'. Use 'sdl' or 'term'."
	@exit 1
endif

# Clean
clean:
	rm -rf $(BIN_DIR)

# Phony targets
.PHONY: all clean sdl term

sdl:
	$(MAKE) RENDERER=sdl

term:
	$(MAKE) RENDERER=term