CC = gcc
CFLAGS = -Wall -g
SRC_DIR = src
BIN_DIR = bin

# Source file
COMPILER_SRC = $(SRC_DIR)/kry_compiler.c

# Targets
all: $(BIN_DIR)/kry_compiler

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/kry_compiler: $(COMPILER_SRC) | $(BIN_DIR)
	$(CC) $(CFLAGS) -o $@ $^

# Clean
clean:
	rm -rf $(BIN_DIR)

# Phony targets
.PHONY: all clean
