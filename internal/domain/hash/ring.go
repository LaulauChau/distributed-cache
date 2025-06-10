package hash

type HashRing interface {
	GetNode(key string) string
	AddNode(node string)
	RemoveNode(node string)
	Nodes() []string
}
