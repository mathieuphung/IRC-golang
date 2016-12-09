package main

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	// "strconv"
	"strings"
)

var upgrader = websocket.Upgrader{}

func check(e error, function string) {
	if e != nil {
		log.Printf(function + " error : ")
		log.Print(e)
	}
}

type Hub struct {
	clients      map[*Client]bool
	broadcast    chan []byte
	addClient    chan *Client
	removeClient chan *Client
}

var hub = Hub{
	broadcast:    make(chan []byte),
	addClient:    make(chan *Client),
	removeClient: make(chan *Client),
	clients:      make(map[*Client]bool),
}

type Message struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Users   string `json:"users"`
}

var users []string

func (hub *Hub) start() {
	for {
		select {
		case conn := <-hub.addClient:
			hub.clients[conn] = true
		case conn := <-hub.removeClient:
			if _, ok := hub.clients[conn]; ok {
				delete(hub.clients, conn)
				close(conn.send)
			}
		case message := <-hub.broadcast:
			for conn := range hub.clients {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(hub.clients, conn)
				}
			}
		}
	}
}

type Client struct {
	ws   *websocket.Conn
	send chan []byte
}

// func usernameCheck(name string, i int) string {
// 	var nameExists bool
// 	if i != 0 {
// 		name = name + strconv.Itoa(i)
// 	}
// 	log.Print(name)
// 	for _, v := range users {
// 		if v == name {
// 			nameExists = true
// 		}
// 	}
// 	if nameExists {
// 		i++
// 		return usernameCheck(name, i)
// 	} else {
// 		return name
// 	}
// }

func userDelete(name string) []string {
	for k, v := range users {
		if v == name {
			return append(users[:k], users[k+1:]...)
			break
		}
	}
	return users
}

func (c *Client) write() {
	defer func() {
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.ws.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func (c *Client) read() {
	defer func() {
		hub.removeClient <- c
		c.ws.Close()
	}()

	var j Message
	usersList := strings.Join(users, ",")
	if j.Users != usersList {
		j.Users = usersList
	}
	mess, err := json.Marshal(j)
	check(err, "json.Marshal(j)")
	hub.broadcast <- mess
	for {
		_, message, err := c.ws.ReadMessage()
		check(err, "c.ws.ReadMessage()")
		if err != nil {
			hub.removeClient <- c
			c.ws.Close()
			break
		}

		err = json.Unmarshal(message, &j)
		check(err, "json.Unmarshal(message, &j)")

		log.Print(j.Title)
		if j.Title == "new user" {
			// j.Content = usernameCheck(j.Content, 0)
			users = append(users, j.Content)
			log.Print(users)
		}

		if j.Title == "user disconnect" {
			users = userDelete(j.Content)
		}
		usersList := strings.Join(users, ",")
		if j.Users != usersList {
			j.Users = usersList
		}
		mess, err := json.Marshal(j)
		check(err, "json.Marshal(j)")

		hub.broadcast <- mess
	}
}

func wsPage(res http.ResponseWriter, req *http.Request) {
	for {
		conn, err := upgrader.Upgrade(res, req, nil)
		if err != nil {
			http.NotFound(res, req)
			return
		}

		client := &Client{
			ws:   conn,
			send: make(chan []byte),
		}
		hub.addClient <- client
		log.Print("new connection")

		go client.write()
		go client.read()
	}
}

func homePage(res http.ResponseWriter, req *http.Request) {
	http.ServeFile(res, req, "index.html")
}

func main() {
	go hub.start()
	http.HandleFunc("/ws", wsPage)
	http.HandleFunc("/", homePage)
	err := http.ListenAndServe(":8000", nil)
	check(err, "http.ListenAndServe(\":8000\", nil)")
}
