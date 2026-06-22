# `profile-list`

List all profiles declared in the configuration file, marking the active
one.

Note: this command is invoked as `chezmoi profile list`.

## Common flags

### `-f`, `--format` `json`|`yaml`

--8<-- "common-flags/format.md"

## Examples

```sh
# List all available profiles
chezmoi profile list

# List profiles as JSON
chezmoi profile list --format=json
```
