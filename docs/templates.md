# Templates

The binary embeds `default`, `concept`, `reference`, `empty`, and the research
templates `paper`, `experiment`, `meeting`, and `idea`. Default is `# {{title}}`
plus a blank line. Concept adds Idea, Explanation, Example, Sources; reference
adds Overview, Details, Sources. Empty produces zero bytes.

| Template | Sections after the title |
| --- | --- |
| `paper` | Citation (key, authors, year, DOI, URL), Summary, Contributions, Methods, Relevance, Questions and critique |
| `experiment` | `Date:` line, Question, Setup, Procedure, Results, Interpretation, Next steps |
| `meeting` | `Date:` and `Attendees:` lines, Agenda, Notes, Decisions, Action items |
| `idea` | `Date:` line, Idea, Why it matters, First step |

Templates add no metadata or frontmatter automatically: a date appears only where
a template contains `{{date}}`. Override any of them with a same-named user file.

Put flat `NAME.md` files in the configured templates directory. Names contain
letters, digits, hyphens, or underscores. Symlinked files and path identifiers are
rejected. A user file overrides the same embedded name; `note templates` lists
sources, overrides, and the configured default. `--template` takes precedence
over `default_template`. Explicit missing/invalid templates are errors.

Only `{{title}}`, `{{slug}}`, `{{path}}`, `{{date}}`, and the citation
placeholders `{{citekey}}`, `{{authors}}`, `{{year}}`, `{{doi}}`, and `{{url}}` are
supported. `{{date}}` is the local creation date as `YYYY-MM-DD`. Citation
placeholders are filled by `note cite` and `note new --cite` from the configured
bibliography and are empty otherwise. `{{authors}}` lists names as
`First Last, …`; `{{year}}` uses `year` or the first four characters of `date`;
`{{title}}` is the entry title unless `--title` is given. Titles and names lose
protective braces and common LaTeX escapes/accents; DOI and URL are verbatim. Values are inserted once
without evaluation. Unknown/malformed placeholders fail before note creation.
Escape an opener as `\{{` to emit literal `{{`. `$PATH` and `$E = mc^2$` remain
unchanged. There are no executable expressions, includes, functions, or date formats.

Title defaults to the original basename without `.md`, hyphens/underscores
becoming spaces, whitespace collapsed, and its first letter uppercased. Acronyms
and Unicode are preserved. `--title` changes only the heading; basename slugging
uses German umlaut transliteration, NFKD, ASCII letters/digits, and hyphens.
Existing notes never change when a template or title changes.
