{
  description = "flow-go development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          # Signal to Makefile to skip tool installation
          FLOW_GO_SKIP_TOOL_INSTALL = "1";

          buildInputs = with pkgs; [
            # Go
            go

            # Code generation
            go-mockery
            buf
            protobuf
            protoc-gen-go
            protoc-gen-go-grpc
            gotools # includes stringer

            # Linting
            golangci-lint

            # Development tools
            delve
            gnumake
            git

            # Required for CGO (crypto library)
            gcc
          ];
        };
      }
    );
}
