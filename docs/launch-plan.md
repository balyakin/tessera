# Launch Plan

## Show HN Draft

Show HN: Tessera - semantic book publishing from DOCX/ODT to PDF and EPUB

I built Tessera for authors who already mark meaning with named Word/LibreOffice styles. Instead of flattening those styles, Tessera turns them into a versioned IR and renders PDF through LaTeX plus EPUB 3.

## Reddit Angles

- `/r/golang`: public Go API for semantic DOCX/ODT processing.
- `/r/selfpublish`: one manuscript to print PDF and EPUB.
- `/r/latex`: generated semantic LaTeX macros from word processor styles.
- `/r/epub`: EPUB 3 semantics and built-in lint.
- `/r/libreoffice`: named Writer styles as publishing semantics.
- `/r/commandline`: no-GUI publishing CLI.

## Lobsters Draft

Tessera is a Go CLI and library that preserves named author styles from DOCX/ODT into LaTeX and EPUB.

## Blog Outline

1. Why visual conversion loses author intent.
2. Named styles as semantic markup.
3. IR design.
4. EPUB and LaTeX backend decisions.
5. Reproducible demo.

## Social Thread

1. DOCX/ODT can already contain semantic style names.
2. Generic converters usually flatten them.
3. Tessera preserves them into IR, LaTeX, and EPUB.
4. Demo command and Docker command.

## Checklist

- Release binaries.
- Docker image.
- README screenshots/assets.
- Demo EPUB.
- Example PDF.
- Checksums and SBOM artifacts.
