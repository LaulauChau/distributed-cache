package hash

import "testing"

func TestConsistentHashRing_EmptyRing(t *testing.T) {
	ring := NewConsistentHashRing()

	if node := ring.GetNode("test-key"); node != "" {
		t.Errorf("expected empty string for empty ring, got %s", node)
	}

	if nodes := ring.Nodes(); len(nodes) != 0 {
		t.Errorf("expected empty nodes list, got %v", nodes)
	}
}

func TestConsistentHashRing_SingleNode(t *testing.T) {
	ring := NewConsistentHashRing()
	ring.AddNode("node1")

	testKeys := []string{"key1", "key2", "key3", "key4", "key5"}
	for _, key := range testKeys {
		if node := ring.GetNode(key); node != "node1" {
			t.Errorf("expected node1 for key %s, got %s", key, node)
		}
	}

	nodes := ring.Nodes()
	if len(nodes) != 1 || nodes[0] != "node1" {
		t.Errorf("expected [node1], got %v", nodes)
	}
}

func TestConsistentHashRing_MultipleNodes(t *testing.T) {
	ring := NewConsistentHashRing()
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	nodes := ring.Nodes()
	expectedNodes := []string{"node1", "node2", "node3"}
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	for i, expected := range expectedNodes {
		if nodes[i] != expected {
			t.Errorf("expected node %s at index %d, got %s", expected, i, nodes[i])
		}
	}
}

func TestConsistentHashRing_KeyConsistency(t *testing.T) {
	ring := NewConsistentHashRing()
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	testKey := "consistent-test-key"
	firstResult := ring.GetNode(testKey)

	for i := 0; i < 10; i++ {
		if result := ring.GetNode(testKey); result != firstResult {
			t.Errorf("key mapping changed: expected %s, got %s", firstResult, result)
		}
	}
}

func TestConsistentHashRing_NodeRemoval(t *testing.T) {
	ring := NewConsistentHashRing()
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	ring.RemoveNode("node2")

	nodes := ring.Nodes()
	expectedNodes := []string{"node1", "node3"}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes after removal, got %d", len(nodes))
	}

	for i, expected := range expectedNodes {
		if nodes[i] != expected {
			t.Errorf("expected node %s at index %d, got %s", expected, i, nodes[i])
		}
	}

	testKey := "test-key"
	node := ring.GetNode(testKey)
	if node != "node1" && node != "node3" {
		t.Errorf("key mapped to removed node or invalid node: %s", node)
	}
}

func TestConsistentHashRing_DuplicateNodes(t *testing.T) {
	ring := NewConsistentHashRing()
	ring.AddNode("node1")
	ring.AddNode("node1")

	nodes := ring.Nodes()
	if len(nodes) != 1 || nodes[0] != "node1" {
		t.Errorf("expected single node1, got %v", nodes)
	}
}

func TestConsistentHashRing_RemoveNonexistentNode(t *testing.T) {
	ring := NewConsistentHashRing()
	ring.AddNode("node1")

	ring.RemoveNode("nonexistent")

	nodes := ring.Nodes()
	if len(nodes) != 1 || nodes[0] != "node1" {
		t.Errorf("removing nonexistent node affected ring: %v", nodes)
	}
}

func TestConsistentHashRing_Distribution(t *testing.T) {
	ring := NewConsistentHashRing()
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	nodeCount := make(map[string]int)
	totalKeys := 1000

	for i := 0; i < totalKeys; i++ {
		key := string(rune('a'+i%26)) + string(rune('0'+i%10))
		node := ring.GetNode(key)
		nodeCount[node]++
	}

	if len(nodeCount) != 3 {
		t.Errorf("expected keys distributed to 3 nodes, got %d", len(nodeCount))
	}

	for node, count := range nodeCount {
		percentage := float64(count) / float64(totalKeys) * 100
		if percentage < 10 || percentage > 60 {
			t.Errorf("poor distribution for %s: %.2f%% (%d keys)", node, percentage, count)
		}
	}
}
