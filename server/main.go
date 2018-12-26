package main

import (
	"bytes"
	"fmt"
	native "log"
	"net/http"
	"os"
	"time"

	ws "github.com/gorilla/websocket"
)

var (
	version string
	date    string
	logger  = native.New(os.Stdout, "", 0)
	errors  = native.New(os.Stderr, "", 0)
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // disable CORS
	},
}

func main() {
	mngr := manager{
		clients:   make(map[*client]bool),
		add:       make(chan *client),
		remove:    make(chan *client),
		broadcast: make(chan []byte),
	}
	go func() {
		for {
			select {
			case client := <-mngr.add:
				mngr.clients[client] = true
			case client := <-mngr.remove:
				if _, ok := mngr.clients[client]; ok {
					delete(mngr.clients, client)
					close(client.send)
				}
			case message := <-mngr.broadcast:
				for client := range mngr.clients {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(mngr.clients, client)
					}
				}
			}
		}
	}()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Println(err)
			return
		}
		client := &client{
			conn: conn,
			send: make(chan []byte, 256),
			mngr: mngr,
		}
		mngr.add <- client

		go client.write()
		go client.read()
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		if len(version) > 0 && len(date) > 0 {
			fmt.Fprintf(w, "version: %s (built at %s)", version, date)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	logger.Printf("[service] listening on port %d", 8080)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		errors.Fatal(err)
	}
}

type manager struct {
	clients   map[*client]bool
	add       chan *client
	remove    chan *client
	broadcast chan []byte
}

type client struct {
	conn *ws.Conn
	send chan []byte
	mngr manager
}

const (
	maxMessageSize = 512
	readDeadline   = 60 * time.Second
	writeDeadline  = 10 * time.Second
	pingPeriod     = (writeDeadline * 9) / 10
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

func (c *client) read() {
	defer func() {
		c.mngr.remove <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(readDeadline))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				logger.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.mngr.broadcast <- message
	}
}

func (c *client) write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if !ok {
				c.conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(ws.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.conn.WriteMessage(ws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
