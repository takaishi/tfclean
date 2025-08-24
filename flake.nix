{
  description = "tfclean is a tool for cleaning up Terraform configuration files by automatically removing applied moved, import, and removed blocks.";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in
      {
        packages.default = import ./default.nix { inherit pkgs; };
      }) // {
        overlays.default = final: prev: { 
          tfclean = import ./default.nix { pkgs = final; };
        };
      };
}