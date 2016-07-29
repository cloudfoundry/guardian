package os_wrap

import real_os "os"

/* builds golang native os object */
func NewOs() Os {
	return new(_os)
}

type _os struct{}

func (_ *_os) Open(name string) (*real_os.File, error) {
	return real_os.Open(name)
}
