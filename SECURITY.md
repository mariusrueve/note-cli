# Security

The first supported release line is 0.1.x. Use the latest patch release on that
line; snapshots are development artifacts.

Report security vulnerabilities privately through the repository's **Security →
Report a vulnerability** page. If private reporting is unavailable, contact the
repository owner through their GitHub profile before posting sensitive details.
Include the affected version, platform, reproduction steps, and expected impact.
Use temporary notes and a local bare Git remote when reproducing problems.

CI scans reachable application vulnerabilities with the official Go
`govulncheck`, and a scheduled weekly run checks newly disclosed vulnerabilities.
Go modules and development tools are locked; workflow actions use full commit
hashes. Changes and dependency updates must pass the native test and archive
matrix before release. Review Pixi updates with `pixi update`, commit the changed
lockfile, and run the same checks.

`note sync` obeys Git hooks and configuration, and editor/tool commands are
explicitly configured executable arguments. Only use a knowledge repository and
tool configuration you trust. Note filenames and query text are passed as data.
