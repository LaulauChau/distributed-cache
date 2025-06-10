package hash

import (
	"crypto/sha256"
	"sort"
	"strconv"
)

const virtualNodes = 150

type ConsistentHashRing struct {
	nodes        map[uint32]string
	sortedHashes []uint32
	nodeSet      map[string]bool
}

func NewConsistentHashRing() *ConsistentHashRing {
	return &ConsistentHashRing{
		nodes:   make(map[uint32]string),
		nodeSet: make(map[string]bool),
	}
}

func (r *ConsistentHashRing) GetNode(key string) string {
	if len(r.sortedHashes) == 0 {
		return ""
	}

	hash := r.hashKey(key)
	idx := sort.Search(len(r.sortedHashes), func(i int) bool {
		return r.sortedHashes[i] >= hash
	})

	if idx == len(r.sortedHashes) {
		idx = 0
	}

	return r.nodes[r.sortedHashes[idx]]
}

func (r *ConsistentHashRing) AddNode(node string) {
	if r.nodeSet[node] {
		return
	}

	r.nodeSet[node] = true
	for i := range virtualNodes {
		virtualKey := node + ":" + strconv.Itoa(i)
		hash := r.hashKey(virtualKey)
		r.nodes[hash] = node
		r.sortedHashes = append(r.sortedHashes, hash)
	}
	sort.Slice(r.sortedHashes, func(i, j int) bool {
		return r.sortedHashes[i] < r.sortedHashes[j]
	})
}

func (r *ConsistentHashRing) RemoveNode(node string) {
	if !r.nodeSet[node] {
		return
	}

	delete(r.nodeSet, node)
	for i := range virtualNodes {
		virtualKey := node + ":" + strconv.Itoa(i)
		hash := r.hashKey(virtualKey)
		delete(r.nodes, hash)
	}

	r.rebuildSortedHashes()
}

func (r *ConsistentHashRing) Nodes() []string {
	nodes := make([]string, 0, len(r.nodeSet))
	for node := range r.nodeSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	return nodes
}

func (r *ConsistentHashRing) hashKey(key string) uint32 {
	h := sha256.New()
	h.Write([]byte(key))
	hashBytes := h.Sum(nil)

	return uint32(hashBytes[0])<<24 |
		uint32(hashBytes[1])<<16 |
		uint32(hashBytes[2])<<8 |
		uint32(hashBytes[3])
}

func (r *ConsistentHashRing) rebuildSortedHashes() {
	r.sortedHashes = make([]uint32, 0, len(r.nodes))
	for hash := range r.nodes {
		r.sortedHashes = append(r.sortedHashes, hash)
	}
	sort.Slice(r.sortedHashes, func(i, j int) bool {
		return r.sortedHashes[i] < r.sortedHashes[j]
	})
}
