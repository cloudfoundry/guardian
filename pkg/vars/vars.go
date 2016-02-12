package vars

import "strings"

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
