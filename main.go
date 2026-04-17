package main

import (
	"fmt"
	"hash/crc32"
	"net/http"
	"sort"
	"strings"
	"sync"
)

var ch *ConsistentHash

type ConsistentHash struct {
	ring     []uint32          // Ring to store hashes
	hashMap  map[uint32]string // Maps hash -> server1, server2
	replicas int               // Generates additional shards to place around the ring for even distribution
	mu       sync.RWMutex      // To allow concurrency of reads/writes
}

func hashKey(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

func (c *ConsistentHash) RemoveNode(node string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := 0; i < c.replicas; i++ {
		virtualNodeKey := fmt.Sprintf("%s-%d", node, i)
		hash := hashKey(virtualNodeKey)

		idx := sort.Search(len(c.ring), func(i int) bool {
			return c.ring[i] >= hash
		})

		if idx < len(c.ring) && c.ring[idx] == hash {
			c.ring = append(c.ring[:idx], c.ring[idx+1:]...)
			delete(c.hashMap, hash)
		}

	}

}

func (c *ConsistentHash) AddNode(node string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < c.replicas; i++ {
		// Generate replica key for hashing
		virtualNodeKey := fmt.Sprintf("%s-%d", node, i)
		hash := hashKey(virtualNodeKey)

		// Find index to insert
		idx := sort.Search(len(c.ring), func(i int) bool {
			return c.ring[i] >= hash
		})

		// Insert in right position
		c.ring = append(c.ring, 0)
		copy(c.ring[idx+1:], c.ring[idx:])
		c.ring[idx] = hash

		// Update map to point towards the server
		c.hashMap[hash] = node
	}
}

func (c *ConsistentHash) GetNode(key string) string {

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.ring) == 0 {
		return ""
	}

	hash := hashKey(key)

	// Find the closest idx clockwise
	idx := sort.Search(len(c.ring), func(i int) bool {
		return c.ring[i] >= hash
	})

	// We circled around the ring as search returned N
	if idx == len(c.ring) {
		idx = 0
	}

	return c.hashMap[c.ring[idx]]
}

// Constructor for consistent hash
func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		ring:     []uint32{},
		hashMap:  make(map[uint32]string),
		replicas: replicas,
	}
}

// Endpoint to handle testing
func userHandler(w http.ResponseWriter, r *http.Request) {
	if ch == nil {
		http.Error(w, "hash not initialized", 500)
		return
	}

	// URL: /user/123
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "missing user id", http.StatusBadRequest)
		return
	}

	userID := parts[2]

	// consistent hash lookup
	node := ch.GetNode(userID)

	// simulate routing to that node
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"user":"%s","server":"%s"}`, userID, node)
}
func main() {
	ch = NewConsistentHash(5)

	ch.AddNode("server1")
	ch.AddNode("server2")
	ch.AddNode("server3")
	http.HandleFunc("/user/", userHandler)
	fmt.Printf("Server starting on http://localhost:8080/")
	http.ListenAndServe(":8080", nil)
}
