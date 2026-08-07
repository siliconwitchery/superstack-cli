{
  description = "The Superstack command line interface";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems
        (system: f (import nixpkgs { inherit system; }));
    in {
      packages = forAllSystems (pkgs: rec {
        superstack = pkgs.buildGoModule {
          pname = "superstack";

          version = builtins.head (
            builtins.match ".*const version = \"([^\"]+)\".*" (builtins.readFile ./main.go)
          );

          src = ./.;

          vendorHash = null;

          env.CGO_ENABLED = 0;

          ldflags = [
            "-s"
            "-w"
          ];

          postInstall = ''
            mv $out/bin/superstack-cli $out/bin/superstack
          '';

          meta = {
            description = "The Superstack command line interface";
            homepage = "https://github.com/siliconwitchery/superstack-cli";
            license = pkgs.lib.licenses.isc;
            mainProgram = "superstack";
            platforms = pkgs.lib.platforms.unix;
          };
        };

        default = superstack;
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            git
            goreleaser
            shellcheck
          ];

          shellHook = ''
            export CGO_ENABLED=0
          '';
        };
      });
    };
}
