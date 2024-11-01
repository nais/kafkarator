{
  description = "Kafkarator";

  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";

  outputs = {nixpkgs, ...}: let
    goOverlay = final: prev: {
      go = prev.go.overrideAttrs (old: rec {
        version = "1.23.0";
        src = prev.fetchurl {
          url = "https://go.dev/dl/go${version}.src.tar.gz";
          hash = "sha256-Qreo6A2AXaoDAi7T/eQyHUw78smQoUQWXQHu7Nb2mcY=";
        };
      });
    };
    # helpers
    withSystem = nixpkgs.lib.genAttrs ["x86_64-linux" "x86_64-darwin" "aarch64-linux" "aarch64-darwin"];
    withPkgs = f:
      withSystem (system:
        f (import nixpkgs {
          inherit system;
          overlays = [goOverlay];
        }));
  in {
    devShells = withPkgs (pkgs: {
      default = pkgs.mkShell {
        buildInputs = with pkgs; [
          gnumake
          go
          golangci-lint-langserver
          # go-dlv  # TODO: Add
          gopls
          python3
        ];
      };
    });
    formatter = withPkgs (pkgs: pkgs.nixfmt-rfc-style);
  };
}
