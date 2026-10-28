package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// host relays between the extension (native messaging on stdin/stdout) and clients on the Unix socket.
type host struct {
	mu      sync.Mutex
	clients map[net.Conn]bool
	pending map[uint64]pendingRequest
	nextID  uint64
}

// pendingRequest remembers who sent a request and the id they gave it, since ids are rewritten to be unique.
type pendingRequest struct {
	conn net.Conn
	id   json.RawMessage
}

func runHost() {
	h := &host{clients: map[net.Conn]bool{}, pending: map[uint64]pendingRequest{}}
	listeners := make(chan net.Listener, 1)
	go func() {
		ln := listenWhenFree(socketPath())
		listeners <- ln
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go h.serveClient(conn)
		}
	}()

	h.readExtension()

	select {
	case ln := <-listeners:
		ln.Close()
	default:
	}
}

// listenWhenFree waits until no other host (e.g. in another browser) owns the socket.
func listenWhenFree(path string) net.Listener {
	for {
		ln, err := net.Listen("unix", path)
		if err == nil {
			return ln
		}
		conn, dialErr := net.Dial("unix", path)
		if dialErr == nil {
			conn.Close()
		} else if os.Remove(path) == nil {
			continue
		} else {
			log.Print(err)
		}
		time.Sleep(3 * time.Second)
	}
}

func (h *host) readExtension() {
	for {
		var size uint32
		if binary.Read(os.Stdin, binary.NativeEndian, &size) != nil {
			return
		}
		msg := make([]byte, size)
		if _, err := io.ReadFull(os.Stdin, msg); err != nil {
			return
		}
		h.fromExtension(msg)
	}
}

func (h *host) fromExtension(msg []byte) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		log.Print(err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := m["event"]; ok {
		for conn := range h.clients {
			conn.Write(append(msg, '\n'))
		}
		return
	}

	var id uint64
	json.Unmarshal(m["id"], &id)
	req, ok := h.pending[id]
	if !ok {
		return
	}
	delete(h.pending, id)
	m["id"] = req.id
	line, _ := json.Marshal(m)
	req.conn.Write(append(line, '\n'))
}

func (h *host) serveClient(conn net.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var m map[string]json.RawMessage
		if json.Unmarshal(scanner.Bytes(), &m) != nil {
			continue
		}

		h.mu.Lock()
		h.nextID++
		h.pending[h.nextID] = pendingRequest{conn, m["id"]}
		m["id"], _ = json.Marshal(h.nextID)
		h.writeExtension(m)
		h.mu.Unlock()
	}
}

// writeExtension must be called with h.mu held so messages aren't interleaved on stdout.
func (h *host) writeExtension(m any) {
	msg, _ := json.Marshal(m)
	binary.Write(os.Stdout, binary.NativeEndian, uint32(len(msg)))
	os.Stdout.Write(msg)
}
