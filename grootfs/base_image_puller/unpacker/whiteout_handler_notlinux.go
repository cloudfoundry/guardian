// +build !linux

package unpacker

import "errors"

func (h *overlayWhiteoutHandler) RemoveWhiteout(path string) error {
	return errors.New("Not implemented on non-linux platforms")
}
