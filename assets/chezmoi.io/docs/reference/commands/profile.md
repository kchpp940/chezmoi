# `profile`

Show the currently active profile, its source, the effective configuration
summary, profile overrides, and a list of all available profiles.

This command helps you confirm which profile is in effect before running
init or apply, preventing accidental contamination across different machine
environments (e.g., dev workstation, GPU training machine, CI runner, WSL,
or remote inference nodes).

## Common flags

### `-f`, `--format` `json`|`yaml`

--8<-- "common-flags/format.md"

## Examples

```sh
# Show the active profile and effective configuration in human-readable form
chezmoi profile

# Show the same information as JSON (useful for scripting)
chezmoi profile --format=json

# List all available profiles
chezmoi profile list

# Confirm the profile before applying
chezmoi profile && chezmoi apply --dry-run

# Switch profile on the command line and verify
chezmoi --profile=gpu profile
```
