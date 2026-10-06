# Templates

The binary embeds `default`, `concept`, `reference`, and `empty`. Default is
`# {{title}}` plus a blank line. Concept adds Idea, Explanation, Example, Sources;
reference adds Overview, Details, Sources. Empty produces zero bytes. Templates
add no metadata or frontmatter automatically.

Put flat `NAME.md` files in the configured templates directory. Names contain
letters, digits, hyphens, or underscores. Symlinked files and path identifiers are
rejected. A user file overrides the same embedded name; `note templates` lists
sources, overrides, and the configured default. `--template` takes precedence
over `default_template`. Explicit missing/invalid templates are errors.

Only `{{title}}`, `{{slug}}`, and `{{path}}` are supported. Values are inserted once
without evaluation. Unknown/malformed placeholders fail before note creation.
Escape an opener as `\{{` to emit literal `{{`. `$PATH` and `$E = mc^2$` remain
unchanged. There are no executable expressions, includes, functions, or dates.

Title defaults to the original basename without `.md`, hyphens/underscores
becoming spaces, whitespace collapsed, and its first letter uppercased. Acronyms
and Unicode are preserved. `--title` changes only the heading; basename slugging
uses German umlaut transliteration, NFKD, ASCII letters/digits, and hyphens.
Existing notes never change when a template or title changes.
