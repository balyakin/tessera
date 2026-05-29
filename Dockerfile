FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tessera ./cmd/tessera

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    default-jre-headless \
    fonts-ebgaramond \
    texlive-latex-recommended \
    texlive-latex-extra \
    texlive-luatex \
    texlive-xetex \
    texlive-fonts-recommended \
    unzip \
    wget \
  && mkdir -p /opt/epubcheck \
  && wget -qO /tmp/epubcheck.zip https://github.com/w3c/epubcheck/releases/download/v5.1.0/epubcheck-5.1.0.zip \
  && unzip -q /tmp/epubcheck.zip -d /opt/epubcheck \
  && printf '#!/bin/sh\nexec java -jar /opt/epubcheck/epubcheck-5.1.0/epubcheck.jar "$@"\n' > /usr/local/bin/epubcheck \
  && chmod +x /usr/local/bin/epubcheck \
  && rm -f /tmp/epubcheck.zip \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/tessera /usr/local/bin/tessera
RUN useradd --uid 10001 --create-home --shell /usr/sbin/nologin tessera
USER tessera
WORKDIR /work
ENTRYPOINT ["tessera"]
