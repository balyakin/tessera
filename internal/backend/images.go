package backend

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func ImageKey(image ir.Image) string {
	if len(image.Data) == 0 {
		return "name:" + image.Name
	}
	sum := sha256.Sum256(image.Data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
