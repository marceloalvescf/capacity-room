{
  description = "Development shell of k8s-capacity-report";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forEachSystem = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forEachSystem (pkgs: {
        default = pkgs.mkShell {
          packages = [
            pkgs.go_1_27
            pkgs.nodejs_26
            pkgs.typst
            pkgs.poppler-utils
            pkgs.gnumake
            pkgs.govulncheck
            pkgs.trivy
          ];
        };
      });
    };
}
