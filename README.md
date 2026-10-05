# Superstack CLI

`superstack` is the command line interface to Superstack: log in, create
fleets, manage devices, and control who can reach them. It is a single static
binary for managing Superstack from a terminal.

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

## Follow a fleet's logs

`tail` shows a fleet's logs as they arrive. Keep it open in one terminal while
you upload code from another:

```sh
superstack tail <fleet_id> [imei ...] [-n num]
```

- IMEIs after the fleet id limit the logs to those devices.
- Without `-n`, `tail` prints the 10 newest logs, then each new log as it
  arrives. Ctrl-C ends it.
- `-n` prints that many of the newest logs and ends. The server keeps logs for
  fourteen days.

Each line gives the time, the IMEI, the kind of log, the device's name in
brackets, and the text. The time is local and carries its offset. The kind is
`lua` for `print` output, `lifecycle` for code starting or stopping, and
`error` for an error. A device with no name prints `[]`:

```
2026-10-02T12:01:07+02:00 356938035643809 lua [back door] hello
2026-10-02T12:01:09+02:00 356938035643809 error [back door] Code crashed: main.lua:3: ...
2026-10-02T12:01:09+02:00 356938035643810 lifecycle [] Code started
```

The time, the IMEI, and the kind never contain a space, so `grep` and `awk`
can match on them:

```sh
superstack tail 3 -n 500 | grep ' 356938035643809 error '   # one device's errors
```

`print` output appears as Lua prints it. A log of several lines prints one
line for each, with every field repeated. Characters that would control the
terminal appear escaped, such as `\x1b`.

If the server stops answering, `tail` says so on the error stream and keeps
trying. It then carries on from where it stopped, with no log lost or
repeated. The output holds only logs, so `tee` can keep a copy:

```sh
superstack tail 3 | tee tail.log
```

## Local development

1. Clone the repository:

    ```sh
    git clone https://github.com/siliconwitchery/superstack-cli.git ~/projects/superstack-cli
    cd ~/projects/superstack-cli
    ```

1. Install the toolchain:

    - **Any platform:** [Go](https://go.dev) 1.25 or newer.
    - **Nix:** `nix develop` from inside the clone, or `direnv allow` once with
      [direnv](https://direnv.net) hooked into your shell.

1. Build and run:

    ```sh
    CGO_ENABLED=0 go build -o superstack .
    ./superstack
    ```

1. Run the tests while you work. Go caches per package, so a change to one
   part leaves every other part cached:

    ```sh
    CGO_ENABLED=0 go test ./...
    ```

1. Run every check before opening a pull request:

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

1. Change `version` in `main.go`.

1. Run every check:

    ```sh
    gofmt -l .
    go mod tidy
    git status --porcelain -- go.mod go.sum
    CGO_ENABLED=0 go vet ./...
    CGO_ENABLED=0 go test ./...
    git diff --check
    ```

1. Commit and push:

    ```sh
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
