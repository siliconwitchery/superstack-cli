# Superstack CLI

`superstack` is the command line interface to Superstack: sign in, claim
devices, push Lua code, and stream events and logs from your fleet. It is a
single static binary talking to the Superstack server's JSON API.

## Install

Every option below needs a published release to exist first.

### Linux and macOS

```sh
curl -fsSL https://raw.githubusercontent.com/siliconwitchery/superstack-cli/main/install.sh | sh
```

To install a specific version instead:

```sh
curl -fsSL https://raw.githubusercontent.com/siliconwitchery/superstack-cli/main/install.sh | VERSION=1.2.3 sh
```

### macOS, with Homebrew

```sh
brew install --cask siliconwitchery/tap/superstack
```

### Windows, with winget

```sh
winget install SiliconWitchery.Superstack
```

### Windows, with Scoop

```sh
scoop bucket add siliconwitchery https://github.com/siliconwitchery/scoop-bucket
scoop install superstack
```

### Arch Linux

```sh
yay -S superstack-bin
```

### Nix

Superstack is not in nixpkgs yet. Until it is, install it from this flake:

```sh
nix profile install github:siliconwitchery/superstack-cli
```

Or run it without installing anything:

```sh
nix run github:siliconwitchery/superstack-cli
```

### Any platform, by hand

Download an archive for your platform from the
[releases page](https://github.com/siliconwitchery/superstack-cli/releases),
unpack it, and move `superstack` somewhere on your `PATH`.

## Build

1. Install [Nix](https://nixos.org) with flakes enabled.

2. Clone the repository:

    ```sh
    git clone git@github.com:siliconwitchery/superstack-cli.git
    cd superstack-cli
    ```

3. Enter the dev shell:

    ```sh
    nix develop
    ```

4. Build and run:

    ```sh
    go build -o superstack .
    ./superstack
    ```

## Release

### One-time setup

Every step here is required before the first release. A release fails if any
of the three secrets is missing.

#### Homebrew and Scoop

1. Create a public repository `siliconwitchery/homebrew-tap`, ticking "Add a
   README file" so it has a default branch.

2. Create a public repository `siliconwitchery/scoop-bucket`, ticking "Add a
   README file" so it has a default branch.

3. Go to Settings > Developer settings > Personal access tokens > Fine-grained
   tokens, and generate a token with:

    - Resource owner: `siliconwitchery`
    - Repository access: only `homebrew-tap` and `scoop-bucket`
    - Permissions: Contents, read and write

4. In this repository, go to Settings > Secrets and variables > Actions > New
   repository secret. Name it `TAP_GITHUB_TOKEN` and paste the token in.

#### winget

1. Fork `microsoft/winget-pkgs` into `siliconwitchery`.

2. Go to Settings > Developer settings > Personal access tokens > Tokens
   (classic), and generate a token with the `public_repo` scope. It has to be
   a classic token, because a fine-grained token cannot open a pull request
   against a repository outside its resource owner.

3. In this repository, add it as an Actions secret named
   `WINGET_GITHUB_TOKEN`.

#### AUR

1. Register an account at
   [aur.archlinux.org/register](https://aur.archlinux.org/register).

2. Generate an SSH key with no passphrase:

    ```sh
    ssh-keygen -t ed25519 -N "" -f aur_key
    ```

3. Print the public key and paste it into "SSH Public Key" in your AUR account
   settings:

    ```sh
    cat aur_key.pub
    ```

4. Print the private key and add it to this repository as an Actions secret
   named `AUR_KEY`:

    ```sh
    cat aur_key
    ```

5. Delete both key files:

    ```sh
    rm aur_key aur_key.pub
    ```

### Cutting a release

1. Bump `version` in `main.go`. The tag you push next must match it exactly, or
   the release workflow refuses to build.

2. Commit and push:

    ```sh
    git commit -am "Version 0.1.0"
    git push
    ```

3. Tag that commit and push the tag:

    ```sh
    git tag v0.1.0
    git push origin v0.1.0
    ```

4. Wait for the release workflow to finish. It builds static binaries for
   Linux, macOS, and Windows on amd64 and arm64, publishes the GitHub release
   with checksums, updates the Homebrew tap and the Scoop bucket, pushes to
   the AUR, and opens a winget pull request.

5. Write the release notes into the release body on GitHub. It starts empty by
   design. Follow the shape in CLAUDE.md.

6. Check that the winget pull request opened against `microsoft/winget-pkgs`.
   It is the one step GoReleaser reports without failing the run, so a failure
   there is easy to miss.

## Repository layout

```sh
├── .github/dependabot.yml # Weekly action and module update PRs
├── .github/workflows      # CI on PRs and pushes to main, release on v* tags
├── .gitignore
├── .goreleaser.yaml       # Build matrix and every publishing target
├── CLAUDE.md              # Coding principles and release conventions
├── flake.lock             # Pinned nixpkgs for the package and the dev shell
├── flake.nix              # The superstack package, and the dev shell
├── go.mod
├── install.sh             # curl-to-shell installer for Linux and macOS
├── LICENSE
├── main.go                # Entry point, version constant, and the command table
├── main_test.go
└── README.md
```

CI runs `goreleaser release --snapshot` on every pull request, so a mistake in
`.goreleaser.yaml` fails there rather than halfway through a tagged release.
