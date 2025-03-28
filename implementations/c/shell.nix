{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  name = "kryon-c-dev";

  # Build inputs (dependencies)
  buildInputs = with pkgs; [
    gcc           # C compiler
    gnumake       # Make for building
    SDL2          # SDL2 library
    SDL2_ttf      # SDL2_ttf for text rendering
    raylib        # Raylib library
    termbox       # Termbox for terminal rendering (replacing notcurses)
    pkg-config
  ];

  # Shell hook to set up environment variables
  shellHook = ''
    export C_INCLUDE_PATH="${pkgs.SDL2.dev}/include:${pkgs.SDL2_ttf}/include:${pkgs.raylib}/include:${pkgs.termbox}/include:$C_INCLUDE_PATH"
    export LIBRARY_PATH="${pkgs.SDL2}/lib:${pkgs.SDL2_ttf}/lib:${pkgs.raylib}/lib:${pkgs.termbox}/lib:$LIBRARY_PATH"
    export LD_LIBRARY_PATH="${pkgs.SDL2}/lib:${pkgs.SDL2_ttf}/lib:${pkgs.raylib}/lib:${pkgs.termbox}/lib:$LD_LIBRARY_PATH"
    echo "Kryon C development environment loaded."
    echo "Available renderers: raylib, term (using termbox)"
    echo "Run 'make term' to build the termbox terminal renderer."
  '';
}