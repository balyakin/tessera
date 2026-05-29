# Docker

```sh
docker run --rm -v "$PWD:/work" ghcr.io/balyakin/tessera:latest \
  build /work/examples/semantic-demo.docx --output /work/dist --lint
```

The image runs as non-root user `tessera` with UID `10001`, uses `/work` as its working directory, and includes Tessera, TeX Live runtime packages, EB Garamond compatible fonts, and `epubcheck`.
