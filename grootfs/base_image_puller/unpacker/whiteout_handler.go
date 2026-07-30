package unpacker

import (
	"os"
)

type overlayWhiteoutHandler struct {
	storeDir *os.File
}

func NewOverlayWhiteoutHandler(storeDir *os.File) WhiteoutHandler {
	return &overlayWhiteoutHandler{
		storeDir: storeDir,
	}
}
