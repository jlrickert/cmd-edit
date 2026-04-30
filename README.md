# cmd-edit / ed

Editor wrapper that exports project context to the child editor and ships
shell completions. The binary is named `ed`; the Homebrew formula is named
`cmd-edit` to avoid colliding with the GNU `ed` line editor on Homebrew core.

## Install

    brew install jlrickert/formulae/cmd-edit

## Usage

    ed [path]

`ed` resolves the project root (via `git rev-parse --show-toplevel` from the
target directory, with `dirname` as a fallback), classifies the project type
from marker files (`composer.json`, `package.json`, `deno.json`, `init.lua`,
etc.), and exports the following variables into the child editor's
environment before invoking `$VISUAL` / `$EDITOR` (falling back to `nano`):

- `EDITOR_PROJECT_ROOT`
- `EDITOR_EDIT_TYPE` (`file`, `directory`, or `unknown`)
- `EDITOR_PATH`
- `EDITOR_FILE_TYPE` (mime type from `file --mime-type -b`)
- `EDITOR_PROJECT_TYPE`

## Shell completions

Cobra-generated completions are available via:

    ed completion bash
    ed completion zsh
    ed completion fish
    ed completion powershell

The Homebrew formula installs them automatically.
