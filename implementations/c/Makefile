CC = gcc
CFLAGS = -Wall -g -Iinclude
LDFLAGS_RAYLIB = -lraylib -lm
LDFLAGS_TERM = -ltermbox -lm

# Directories
SRC_DIR = src
BIN_DIR = bin

# Source files
READER_SRC = $(SRC_DIR)/krb_reader.c
RAYLIB_RENDERER_SRC = $(SRC_DIR)/raylib_renderer.c
TERM_RENDERER_SRC = $(SRC_DIR)/term_renderer.c

# Default renderer
RENDERER ?= raylib

# Define the flag needed to enable the main() in raylib_renderer.c
RAYLIB_STANDALONE_FLAG = -DBUILD_STANDALONE_RENDERER

# Targets
all: $(BIN_DIR)/krb_renderer

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# Renderer-specific targets
# This rule now handles building the specific standalone renderer executable
$(BIN_DIR)/krb_renderer: $(READER_SRC) $(SRC_DIR)/$(RENDERER)_renderer.c | $(BIN_DIR)
ifeq ($(RENDERER),raylib)
	# Add the RAYLIB_STANDALONE_FLAG when compiling raylib
	@echo "Building Standalone Raylib Renderer..."
	$(CC) $(CFLAGS) $(RAYLIB_STANDALONE_FLAG) -o $@ $^ $(LDFLAGS_RAYLIB)
else ifeq ($(RENDERER),term)
	# Assuming term_renderer might also have a standalone mode or just compiles
	@echo "Building Terminal Renderer..."
	# If term_renderer also needs a flag for standalone, add it here
	# $(CC) $(CFLAGS) -DBUILD_STANDALONE_TERM -o $@ $^ $(LDFLAGS_TERM)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS_TERM)
else
	@echo "Error: Unknown renderer '$(RENDERER)'. Use 'raylib', or 'term'."
	@exit 1
endif
	@echo "Build successful: $@"

# Clean
clean:
	@echo "Cleaning build directory..."
	rm -rf $(BIN_DIR)

# Phony targets
.PHONY: all clean raylib term

# These targets simply re-invoke make with the RENDERER variable set
raylib:
	$(MAKE) RENDERER=raylib

term:
	$(MAKE) RENDERER=term