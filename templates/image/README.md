# Go Application Image

A Go application packaged as a container image, bootstrapped with [go-overlay](https://github.com/purpleclay/go-overlay) and `dockerTools.buildLayeredImage`.

## Getting Started

Build the application:

```shell
nix build
./result/bin/example
```

> [!TIP]
> Run `nix build -L` to print full build logs and see each phase as it happens.

Or run the application directly:

```shell
nix run
curl http://localhost:8080/ping
# pong
```

## The Docker bit

Build, load and run the image with Docker:

```shell
nix build .#image
docker load < result
docker run -p 8080:8080 ping-server:latest
```

> [!NOTE]
> On macOS, the image is built for Linux via the
> [linux-builder](https://nixos.org/manual/nixpkgs/stable/#sec-darwin-builder). Follow the
> setup guide to configure Nix, then spin up the VM before building:
> ```shell
> sudo nix run nixpkgs#darwin.linux-builder
> nix build .#image
> ```

Then call the ping endpoint:

```shell
curl http://localhost:8080/ping
# pong
```

## Developer Shell

Enter the development shell with the Go toolchain and `govendor` pre-installed:

```shell
nix develop
```
