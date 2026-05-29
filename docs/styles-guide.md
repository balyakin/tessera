# Styles Guide

Named styles are Tessera's source of semantic meaning. Use paragraph styles for structure such as `Poem`, `Letter`, `Epigraph`, `Quote`, and headings. Use character styles for inline meaning such as `Foreign - Latin`, `Direct Thought`, `Prayer`, and `Work Title`.

In LibreOffice Writer, open the Styles sidebar, create a paragraph or character style, name it with one of Tessera's defaults, and apply it to manuscript text. In Microsoft Word, use the Styles pane and create a style with the same visible name.

Run:

```sh
tessera inspect book.docx
```

Unknown styles include suggested TOML snippets. Add them to `tessera.toml` under `[paragraph_styles]` or `[character_styles]`.
