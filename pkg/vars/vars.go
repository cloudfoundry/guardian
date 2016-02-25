package vars

import (
	"fmt"
	"net"
	"strings"
)

type StringList struct {
	List []string
}

func (sl *StringList) Set(arg string) error {
	sl.List = append(sl.List, arg)
	return nil
}

func (sl *StringList) String() string {
	return strings.Join(sl.List, ", ")
}

func (sl StringList) Get() interface{} {
	return sl.List
}

// IPList is a flag.Value to hold a list of IP addresses
type IPList struct {
	List *[]net.IP
}

func (l IPList) String() string {
	var strs []string
	for _, ip := range *l.List {
		strs = append(strs, ip.String())
	}
	return strings.Join(strs, ", ")
}

func (l IPList) Set(s string) error {
	ip := net.ParseIP(s)
	if ip == nil {
		return fmt.Errorf("'%s' is not a valid IP address", s)
	}
	*l.List = append(*l.List, ip)
	return nil
}
