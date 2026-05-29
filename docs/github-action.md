# GitHub Action

```yaml
name: books
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: balyakin/tessera@v1
        with:
          input: examples/semantic-demo.docx
          output: dist
          args: --lint
      - uses: actions/upload-artifact@v4
        with:
          name: tessera-artifacts
          path: dist
```

For releases, upload `dist/*.pdf` and `dist/*.epub` as release assets after the Tessera step.
