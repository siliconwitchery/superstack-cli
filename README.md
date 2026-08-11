# Superstack CLI

`superstack` is the command line interface to Superstack: sign in, claim
devices, push Lua code, and stream logs from your fleet. It is a
single static binary talking to the Superstack server's JSON API. The server is
a separate project; this repo is the CLI only. It is laid out as follows:

```sh
├── .envrc                 # Loads the Nix dev shell via direnv
├── .github/dependabot.yml # Weekly action and module update PRs
├── .github/workflows      # CI on pull requests, release on v* tags
├── .gitignore
├── .goreleaser.yaml       # Build matrix and every publishing target
├── CLAUDE.md              # Coding principles and architectural overview
├── flake.lock             # Pins nixpkgs
├── flake.nix              # The superstack package, and the dev shell
├── go.mod
├── internal
│   └── commands           # One file per command, plus the shared server client
├── LICENSE
├── main.go                # Entry point, version constant, and the command table
├── main_test.go
└── README.md
```

## Install

Each option needs a published release.

- **Homebrew:**

    ```sh
    brew install --cask siliconwitchery/tap/superstack
    ```

- **Scoop:**

    ```sh
    scoop bucket add siliconwitchery https://github.com/siliconwitchery/scoop-bucket
    scoop install superstack
    ```

- **Arch Linux:**

    ```sh
    yay -S superstack-bin
    ```

- **Nix.** Not in nixpkgs yet.

    ```sh
    nix profile install github:siliconwitchery/superstack-cli
    ```

- **Any platform.** Download an archive from the
  [releases page](https://github.com/siliconwitchery/superstack-cli/releases),
  unpack it, and move `superstack` onto your `PATH`.

## Local development

1. Install [Go](https://go.dev) 1.25 or newer.

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

[Nix](https://nixos.org) users need no Go install: `nix develop` enters the
dev shell, and with [direnv](https://direnv.net) hooked into your shell,
`direnv allow` run once in the checkout loads it automatically from then on.

## Release setup

Do everything below once.

1. Create public repositories `siliconwitchery/homebrew-tap` and
   `siliconwitchery/scoop-bucket`, each with a README.

1. Add a fine-grained token (Settings > Developer settings > Personal access
   tokens) as the Actions secret `TAP_GITHUB_TOKEN`:

    - Resource owner: `siliconwitchery`
    - Repository access: `homebrew-tap` and `scoop-bucket`
    - Permissions: Contents, read and write

1. Register at [aur.archlinux.org](https://aur.archlinux.org/register), then:

    ```sh
    ssh-keygen -t ed25519 -N "" -f aur_key
    cat aur_key.pub   # paste into SSH Public Key in your AUR account settings
    cat aur_key       # add as the Actions secret AUR_KEY
    rm aur_key aur_key.pub
    ```

1. Add one ruleset (Settings > Rules > Rulesets) targeting the default branch:
   require a pull request with 0 approvals, require the `build` status check,
   block force pushes, restrict deletions. Add a second targeting `v*` tags:
   block force pushes, restrict deletions.

## Releasing

1. Bump `version` in `main.go`, then open and merge a pull request:

    ```sh
    git checkout -b version-0.1.0
    git commit -am "Version 0.1.0"
    git push -u origin version-0.1.0
    ```

1. Merging creates a new commit, and the tag has to point at that one:

    ```sh
    git checkout main && git pull
    grep '^const version' main.go
    git tag v0.1.0
    git push origin v0.1.0
    ```

1. Write the release notes into the empty release body on GitHub, following the
   shape in CLAUDE.md.

A tag carrying a prerelease suffix, `v0.0.2-rc1`, publishes a GitHub prerelease
and skips every package manager. Tags cannot be moved or deleted.
