# Consistent Hashing in Go

A consistent hashing implementation in Go using a virtual node ring, with an HTTP API to simulate request routing across servers. Built as a learning project to understand how distributed systems route requests without reshuffling everything when the cluster changes.

<img width="591" height="431" alt="image" src="https://github.com/user-attachments/assets/23c9d473-f35a-4b66-b148-e15808f49ce0" />

## How it works

Incoming requests hit the `/user/:id` endpoint. The user ID is hashed using CRC32 and looked up on the ring to determine which server should handle it. Because the ring is consistent, the same user ID always routes to the same server. Adding or removing a server only remaps a fraction of keys rather than reshuffling everything.

In a real system, the consistent hash would typically live inside an API Gateway that routes to actual backend servers. Here, k6 is used to verify the distribution and confirm the hashing behaves correctly.

### Virtual nodes

Each physical server is placed on the ring multiple times as virtual nodes. With `replicas: 5`, adding `server1` inserts `server1-0` through `server1-4` as distinct points on the ring. This spreads load more evenly and avoids hotspots that would occur if each server only had one position.

Without virtual nodes, a small cluster can end up with very uneven key distribution depending on where servers land on the ring. More replicas means a smoother spread.

### Node lookup

To find the server for a given key:

1. Hash the key with CRC32
2. Binary search the sorted ring for the first position `>=` the hash
3. If the hash is past the last position, wrap around to index 0
4. Return the server mapped to that position

This is the core of consistent hashing: the ring is just a sorted array of hash positions, and lookup is a single binary search.

### Adding and removing nodes

`AddNode` and `RemoveNode` keep the ring sorted after every change using binary search insertion and deletion. A read/write mutex (`sync.RWMutex`) protects concurrent access so reads can happen in parallel while writes get exclusive access.

Only the keys that fall between the removed node and its predecessor need to be remapped. Everything else stays put.

## API

```
GET /user/:id
```

Returns the server that the given user ID maps to.

**Response**
```json
{"user":"5961","server":"server1"}
```

## Running it

```bash
go run main.go
```

The server starts on port `8080` with three nodes and 5 virtual replicas each:



```go
ch = NewConsistentHash(5)
ch.AddNode("server1")
ch.AddNode("server2")
ch.AddNode("server3")
```

Test a few user IDs manually to see which server they map to:

```bash
curl http://localhost:8080/user/123
curl http://localhost:8080/user/456
curl http://localhost:8080/user/789
```

The same user ID will always return the same server as long as the cluster does not change.

## Load testing with k6

A k6 script is included to verify distribution across servers. It sends requests with random user IDs and tallies how many each server received.

```bash
k6 run k6_consistent_hash_test.js
```

Expected output:

```
=== Consistent Hash Distribution ===
  server2    4821  (38.2%)  ████████████
  server1    4209  (33.4%)  ██████████
  server3    3570  (28.4%)  █████████
  TOTAL     12600
=====================================
```

A well-balanced ring shows roughly equal distribution across all servers. If one server is receiving significantly more traffic, it is usually a sign that the replica count is too low. Try increasing `replicas` from 5 to 50 and re-running the test to see how the distribution improves.

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `replicas` | `5` | Virtual nodes per server |
| `port` | `8080` | HTTP listen port |

## Requirements

- Go 1.18+
- [k6](https://k6.io/) (load testing only)
