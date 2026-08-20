# Superstack CLI

`superstack` is the command line interface to Superstack: log in, claim
devices, upload Lua code, and stream logs from your fleet. It is a single
static binary for managing Superstack from a terminal.

## Install

- **macOS**, with [Homebrew](https://brew.sh):

    ```sh
    brew install --cask siliconwitchery/tap/superstack   # install
    brew upgrade --cask superstack                       # update
    ```

- **Windows**, with [Scoop](https://scoop.sh) and git:

    ```sh
    scoop bucket add siliconwitchery https://github.com/siliconwitchery/scoop-bucket
    scoop install superstack                             # install
    scoop update superstack                              # update
    ```

- **Arch Linux**, from the AUR:

    ```sh
    yay -S superstack-bin                                # install and update
    ```

- **NixOS**, with [Nix](https://nixos.org) `nix-command` and `flakes` enabled:

    ```sh
    nix profile install github:siliconwitchery/superstack-cli#superstack   # install
    nix profile upgrade superstack                                         # update
    ```

- **Any platform.** Download an archive from the
  [releases page](https://github.com/siliconwitchery/superstack-cli/releases),
  unpack it, and move `superstack` onto your `PATH`. Repeat to update.

## Local development

1. Install the toolchain:

    - **Any platform:** [Go](https://go.dev) 1.25 or newer.
    - **Nix:** `nix develop`, or `direnv allow` once with
      [direnv](https://direnv.net) hooked into your shell.

1. Clone the repository:

    ```sh
    git clone git@github.com:siliconwitchery/superstack-cli.git ~/projects/superstack-cli
    cd ~/projects/superstack-cli
    ```

1. Build and run:

    ```sh
    CGO_ENABLED=0 go build -o superstack .
    ./superstack
    ```

1. Run the checks:

    ```sh
    gofmt -l .
    go mod tidy
    git status --porcelain -- go.mod go.sum
    CGO_ENABLED=0 go vet ./...
    CGO_ENABLED=0 go test ./...
    ```

## Releasing

1. Create `dev` fresh from `main`:

    ```sh
    git fetch origin
    git switch -C dev origin/main
    ```

1. Change `version` in `main.go`, run the checks, commit, and push:

    ```sh
    gofmt -l .
    go mod tidy
    git status --porcelain -- go.mod go.sum
    CGO_ENABLED=0 go vet ./...
    CGO_ENABLED=0 go test ./...
    git diff --check
    git add main.go
    git commit -m "Version <version>"
    git push -u origin dev
    ```

1. Open the `dev` pull request, review it, and merge it with squash.

1. Return to `main`, remove the stale branch, and tag the commit that merging
   created:

    ```sh
    git switch main
    git pull --ff-only
    git branch -D dev
    tag="v$(sed -n 's/^const version = "\(.*\)"$/\1/p' main.go)"
    git tag "$tag" && git push origin "$tag"
    ```

1. Write the release notes into the empty release body on GitHub.

A prerelease suffix, `v0.0.2-rc1`, skips every package manager. Tags cannot be
moved or deleted.
