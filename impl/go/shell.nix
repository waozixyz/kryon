{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  name = "go-raylib-dev";

  packages = with pkgs; [
    go

    pkg-config
    raylib
    
    wayland 
    wayland-protocols
    libxkbcommon

    # If you still face issues, you might need to uncomment/add some of these:
    # mesa              # OpenGL implementation (includes libGL headers)
    # alsa-lib          # ALSA audio library (libasound)
    # libX11            # Core X11 library
    xorg.libXrandr
    xorg.libXinerama 
    xorg.libXcursor 
    xorg.libXi
  ];

}