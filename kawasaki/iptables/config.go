package iptables

type FilterConfig struct {
	AllowHostAccess bool
	DenyNetworks    []string
	InputChain      string
	ForwardChain    string
	DefaultChain    string
}

type NATConfig struct {
	PreroutingChain  string
	PostroutingChain string
}
