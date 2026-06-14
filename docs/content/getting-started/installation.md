---
title: "Installation"
description: "Install gleif from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/gleif-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `gleif` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/gleif-cli/cmd/gleif@latest
```

That puts `gleif` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/gleif-cli
cd gleif-cli
make build        # produces ./bin/gleif
./bin/gleif version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/gleif:latest --help
```

## Checking the install

```bash
gleif version
```

prints the version and exits.
