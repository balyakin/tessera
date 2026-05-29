package common

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func ResolveMetadata(raw []byte, inputPath string, source ir.Metadata, cfg *config.Config, overrides map[string]string) (ir.Metadata, error) {
	meta := ir.Metadata{Extra: map[string]string{}}
	if cfg != nil {
		meta.Language = cfg.Document.DefaultLanguage
		meta.Title = cfg.Document.Title
		meta.Author = cfg.Document.Author
		for key, value := range cfg.Document.Extra {
			if value != "" {
				meta.Extra[key] = value
			}
		}
	}
	applySource(&meta, source)
	if meta.Title == "" {
		base := filepath.Base(inputPath)
		meta.Title = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if meta.Language == "" && cfg != nil {
		meta.Language = cfg.Document.DefaultLanguage
	}
	for key, value := range overrides {
		switch strings.ToLower(key) {
		case "title":
			if value == "" {
				return meta, fmt.Errorf("metadata title cannot be empty")
			}
			meta.Title = value
		case "subtitle":
			meta.Subtitle = value
		case "author":
			meta.Author = value
		case "language":
			if value == "" {
				return meta, fmt.Errorf("metadata language cannot be empty")
			}
			meta.Language = value
		case "identifier":
			meta.Identifier = value
		case "date":
			meta.Date = value
		case "publisher":
			meta.Publisher = value
		case "description":
			meta.Description = value
		default:
			if value == "" {
				continue
			}
			if meta.Extra == nil {
				meta.Extra = map[string]string{}
			}
			meta.Extra[key] = value
		}
	}
	if meta.Identifier == "" {
		sum := sha256.Sum256(raw)
		meta.Identifier = "urn:tessera:sha256:" + fmt.Sprintf("%x", sum)[:32]
	}
	return meta, nil
}

func applySource(meta *ir.Metadata, source ir.Metadata) {
	if source.Title != "" {
		meta.Title = source.Title
	}
	if source.Subtitle != "" {
		meta.Subtitle = source.Subtitle
	}
	if source.Author != "" {
		meta.Author = source.Author
	}
	if source.Language != "" {
		meta.Language = source.Language
	}
	if source.Identifier != "" {
		meta.Identifier = source.Identifier
	}
	if source.Date != "" {
		meta.Date = source.Date
	}
	if source.Publisher != "" {
		meta.Publisher = source.Publisher
	}
	if source.Description != "" {
		meta.Description = source.Description
	}
	for k, v := range source.Extra {
		if v != "" {
			meta.Extra[k] = v
		}
	}
}
