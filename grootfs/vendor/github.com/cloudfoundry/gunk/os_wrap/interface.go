/*
Package os_wrap wraps golang os in an interface.

With this you can mock os system calls. Please add to this interface
and implementations with other calls that are to be mocked.

The fake/mock implementation is in an aptly named subdirectory.
*/
package os_wrap

import real_os "os"

//go:generate counterfeiter -o osfakes/fake_os.go . Os

/*
Wraps os calls.
*/
type Os interface {
	Open(name string) (*real_os.File, error)
}
