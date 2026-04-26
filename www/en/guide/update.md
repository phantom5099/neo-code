---
title: Update & Version Check
description: Check your version and upgrade NeoCode.
---

# Update & Version Check

## Automatic update check

NeoCode silently checks for a newer version in the background at startup (3-second timeout). The update notice is printed after you exit the TUI, so it does not interrupt the session.

`url-dispatch` and `update` subcommands skip this check.

## Check version

```bash
neocode version
```

Include pre-release versions:

```bash
neocode version --prerelease
```

When the remote "semantic latest" is not installable on the current platform, `version` will also show the "highest installable version" upgrade hint and flag the remote asset anomaly.

## Manual upgrade

```bash
neocode update
```

Include pre-release versions:

```bash
neocode update --prerelease
```

## Version info

- Release builds have a version injected via `ldflags` into `internal/version.Version`
- Local development builds report `dev`

If you're running from source, seeing `dev` is expected.
