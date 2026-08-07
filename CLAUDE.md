# CLAUDE.md

Guidance for working in this repository. This file holds **only** high-level
coding and operational principles plus the architectural overview as it takes
shape. It must never describe product functionality or per-feature behaviour.
The README.md holds the project-structure overview and the setup instructions
for local development and releasing.

**This repository is public.** Never commit secrets, tokens, or customer data:
the entire git history ships.

**Built in tandem with the server.** The server lives at
`~/projects/superstack-next` (github.com/siliconwitchery/superstack-next) and
owns the JSON API this binary speaks. A change on either side of the wire
usually implies one on the other, so read its CLAUDE.md before changing
anything that crosses it. The two files share the coding principles below
verbatim; a change to one belongs in both.

## Coding principles

General and meant to be reused verbatim across projects.

- **Complete names.** Use descriptive, whole-word names for non-trivial
  variables (`tuiWidth`, not `boxW`). Short names are acceptable only for
  receivers, loop indices, `err`, `ok`, and a framework's own idiomatic short
  names: `w` for an `http.ResponseWriter`, `r` for an `*http.Request`. Keep
  those reserved for that exact type: a reader that is not an `*http.Request`
  is `reader`, not `r`.
- **Breathing room.** Separate a statement that produces a value from the
  statement that consumes it with a blank line. For example: an assignment, a
  blank line, then the `if err != nil` check. Group code into readable
  paragraphs.
- **Guard clauses.** Handle edge cases and errors first and return early, so the
  happy path stays unindented and reads straight down the function.
- **No comments.** Code must explain itself through naming and structure.
  (Struct tags are not comments.) Two exceptions. The first is a constraint the
  code cannot express on its own: an external or internal protocol, not a
  restatement of what the code does. For example, noting that a flag exists
  only because another process invokes it. The second is one-line step headings
  that break a long procedural function into navigable sections; a step heading
  says what the paragraph below it is for, never how it works.
- **Procedural code.** Always inline simple logic so readers don't have to jump
  around to see what small functions do. 1-3 line functions shouldn't exist
  unless there's a very good reason, such as wrapping something that could
  change like a hardcoded filepath. Never create a function that is only used
  once. Long procedural functions are fine; reading one top to bottom should
  describe its entire behaviour with minimal jumping around.
- **Functional code.** Prefer functional, stateless code. Some libraries demand
  statefulness and it's fine to follow their style, but everywhere else avoid
  mutable state.
- **Switch over ladders.** Prefer a `switch` (including a type switch) to a long
  `if` / `else if` chain.
- **Co-location.** A self-contained unit lives entirely in its own file: its
  config, state, behaviour, and rendering together. Its only references from
  elsewhere are where it is wired in at the composition root. Removing it means
  deleting its file and that one line of wiring, nothing scattered across the
  project.
- **Table-driven tests.** Express tests as a table of input → expected cases
  iterated in a loop, not as repeated near-identical assertions.

## Operational principles

- Build and run with cgo disabled for a static, dependency-free binary:
  `CGO_ENABLED=0 go build`. Run `go vet` and `go test` with `CGO_ENABLED=0`
  too, so neither reaches for a C toolchain that need not exist.
- Keep the dependency set small: nothing a distribution's packager would balk
  at.

## Architecture

`main.go` is the composition root. It holds one table of sections and commands,
and that table is the single source of truth for both the help output and
dispatch: a command cannot exist in one and not the other. Names may be several
words, and dispatch takes the longest match, so `pipe set` wins over `pipe`.

A command whose `run` is nil reports that it is not implemented yet and exits
non-zero. Filling one in means writing its own file, holding everything that
command needs, and pointing the table entry at it. Removing one means deleting
that file and its line in the table.

Targets are flags rather than positions. Every command that touches devices
takes `--fleet` or `--device`, so an operation that can apply to one device or
to many stays one verb instead of splitting into a noun-verb pair per level.

## Releases

The version lives in one place, `version` in `main.go`. Bump it, then tag the
commit that bumped it. Nothing injects the version at build time, because
`-ldflags -X` cannot write to a Go const; the release workflow instead refuses
to build when the tag and the const disagree.

The workflow also refuses to build a tag that does not sit on `main`. GitHub
rulesets cannot express that, because a tag rule can restrict who creates a
tag but not which commit it points at, so the check lives in the workflow
where it can read the history.

Pushing a `v*` tag is the whole release. GoReleaser builds the static binaries
for Linux, macOS, and Windows, publishes the GitHub release with checksums,
pushes the Homebrew cask to `siliconwitchery/homebrew-tap`, pushes the Scoop
manifest to `siliconwitchery/scoop-bucket`, pushes the `superstack-bin`
PKGBUILD to the AUR, and opens a winget pull request against
`microsoft/winget-pkgs` from the fork at `siliconwitchery/winget-pkgs`. The
`install.sh` at the repo root installs straight from GitHub Releases and needs
no per-release attention.

The flake is the fifth distribution path, and the only one not driven by a
tag. It builds from source at whatever commit the user points it at, so it
needs no release to work. Superstack is not in nixpkgs and will not be until
the project has traction, so the flake is how Nix users install until then.
`flake.nix` reads the version straight out of `main.go`, which keeps the
single source of truth intact, and renames the binary in `postInstall`
because Go names it after the module path rather than after the command.

`.goreleaser.yaml` is the only description of the build matrix. Nothing else
may restate it, because a second copy drifts. CI proves the release path by
running `goreleaser release --snapshot` rather than by rebuilding the same
targets by hand, so the release workflow is never the first thing to exercise
that config.

Every publisher carries `skip_upload: auto`, so a tag with a prerelease suffix
publishes a GitHub release and touches no package manager. That is how the
pipeline gets exercised without shipping. It leaves the four publisher pushes
themselves untested, which only a real tag proves.

Three credentials sit behind the release, and the workflow's guard checks only
that they are present, never that they work. `TAP_GITHUB_TOKEN` is
fine-grained and scoped to the tap and the bucket. `WINGET_GITHUB_TOKEN` has to
be a classic token, because a fine-grained one cannot reach a repository
outside its resource owner and so cannot open the pull request against
Microsoft. `AUR_KEY` is a passphraseless SSH key registered with an AUR
account. GoReleaser reports a failed winget pull request without failing the
run, so that step needs checking by hand after a release.

The generated changelog is deliberately disabled, so a fresh release starts
with an empty body. Write the notes into it afterwards; GoReleaser keeps an
existing body and will not overwrite them on a re-run.

Release notes are written for someone deciding whether to upgrade, not for
someone reading the log. Lead with what changed for them, and never just list
commits. Order the bullets by what a user would notice first. Follow this
shape:

```
**Headline description** {Emoji}

- Top feature/change
- Top feature/change
- Top feature/change
- Other notable/meaningful changes for users
- Security fixes/issues addressed

**Breaking changes**

- Change - solution if available
- ...
```

Omit the breaking changes section entirely when there are none. Omit the
security bullet when nothing was fixed.

## Maintenance

After adding or changing a major feature, re-read this file and update it so
the principles and the architectural overview stay accurate.

Keep the README minimal as the project grows: the structure overview and the
setup steps, nothing else. Numbered imperative steps, self-contained command
blocks, constraints stated bare. No rationale, no explanation of how something
works, no troubleshooting. Anything that explains rather than instructs belongs
in this file, and a step that needs a paragraph to justify it is a sign the
step itself is wrong.
