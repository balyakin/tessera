# Config Reference

Tessera config is TOML. Start with:

```sh
tessera init --output tessera.toml
```

Sections:

- `[document]`: default metadata, cover image, cover alt text, extra metadata.
- `[paragraph_styles]`: maps paragraph style names to roles.
- `[character_styles]`: maps character style names to roles.
- `[languages]`: maps BCP-47 language tags to Polyglossia names.
- `[latex]`: document class options, main font, preamble snippets, preamble file.
- `[epub]`: custom CSS and additional fonts.
- `[toc]`: depth, title, PDF inclusion.
- `[issues]`: suppress or promote warnings.
- `[limits]`: document complexity limits.
- `[output]`: reproducible output default.

Valid paragraph roles: `body`, `heading`, `title`, `subtitle`, `dedication`, `colophon`, `glossary`, `halftitle`, `verse`, `letter`, `epigraph`, `blockquote`.

Valid character roles: `emphasis`, `strong`, `foreign`, `thought`, `prayer`, `work-title`.
