package main

import (
	"bytes"
	"fmt"
	native "log"
	"net/http"
	"os"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/kelseyhightower/envconfig"
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

type config struct {
	Port   int64  `default:"8080"`
	Prefix string `default:"/"`
}

func main() {
	var c config
	if err := envconfig.Process("ws", &c); err != nil {
		errors.Fatal(err)
	}
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
	http.HandleFunc(fmt.Sprintf("%sws", c.Prefix), func(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/", index(c.Prefix))
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
	logger.Printf("[service] listening on port %d", c.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", c.Port), nil); err != nil {
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

var html = `
<!DOCTYPE html>
<html lang="en">
<head>
    <title>Chat Example</title>
    <script type="text/javascript">
    window.onload = function () {
        var conn;
        var msg = document.getElementById("msg");
        var log = document.getElementById("log");

        function appendLog(item) {
            var doScroll = log.scrollTop > log.scrollHeight - log.clientHeight - 1;
            log.appendChild(item);
            if (doScroll) {
                log.scrollTop = log.scrollHeight - log.clientHeight;
            }
        }
        document.getElementById("form").onsubmit = function () {
            if (!conn) {
                return false;
            }
            if (!msg.value) {
                return false;
            }
            conn.send(msg.value);
            msg.value = "";
            return false;
        };
        if (window["WebSocket"]) {
            conn = new WebSocket("ws://" + document.location.host + "%sws");
            conn.onclose = function (evt) {
                var item = document.createElement("div");
                item.innerHTML = "<b>Connection closed.</b>";
                appendLog(item);
            };
            conn.onmessage = function (evt) {
                var messages = evt.data.split('\n');
                for (var i = 0; i < messages.length; i++) {
                    var item = document.createElement("div");
                    item.innerText = messages[i];
                    appendLog(item);
                }
            };
        } else {
            var item = document.createElement("div");
            item.innerHTML = "<b>Your browser does not support WebSockets.</b>";
            appendLog(item);
        }
    };
    </script>
    <style type="text/css">
    html {
        overflow: hidden;
    }
    body {
        overflow: hidden;
        padding: 0;
        margin: 0;
        width: 100%%;
        height: 100%%;
        background: gray;
    }
    #log {
        background: white;
        margin: 0;
        padding: 0.5em 0.5em 0.5em 0.5em;
        position: absolute;
        top: 0.5em;
        left: 0.5em;
        right: 0.5em;
        bottom: 3em;
        overflow: auto;
    }
    #form {
        padding: 0 0.5em 0 0.5em;
        margin: 0;
        position: absolute;
        bottom: 1em;
        left: 0px;
        width: 100%;
        overflow: hidden;
    }
    </style>
    </head>
<body>
    <div id="log"></div>
    <form id="form">
        <input type="submit" value="Send" />
        <input type="text" id="msg" size="64"/>
    </form>
</body>
</html>
`

func index(prefix string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fmt.Sprintf(html, prefix))
	}
}
