# Decisions

## 2026-05-28: CLI And TOML Dependencies

Tessera uses `github.com/spf13/cobra` and `github.com/pelletier/go-toml/v2` because `SPEC.md` requires them. No additional runtime Go dependencies are used.

## 2026-05-28: Deterministic Demo Fixtures

The example ODT and DOCX files are generated from synthetic XML definitions in `internal/demo`. This keeps examples reproducible without requiring LibreOffice or Microsoft Word in CI.
