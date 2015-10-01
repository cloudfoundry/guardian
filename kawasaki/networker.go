package kawasaki

type Networker struct{}

func New() *Networker {
	return &Networker{}
}

func (n *Networker) Network(spec string) (string, error) {
	return "", nil
}
