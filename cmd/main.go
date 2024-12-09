package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/turplespace/msgqueue/internal/models"
	"github.com/turplespace/msgqueue/internal/services"
)

type WebSocketService struct {
	handler *models.WebSocketHandler
}

func (wsh WebSocketService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := wsh.handler.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade Error:", err)
	}

	defer conn.Close()
	var message models.Message
	newQueueService := services.NewQueueService(wsh.handler)
	newPublishService := services.NewPublishService(wsh.handler)
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Connection closed: %v\n", err)
				newPublishService.RemoveConnection(conn)
				return
			}
			log.Println("Read Error:", err)
			newPublishService.RemoveConnection(conn)
		}
		err = json.Unmarshal(msg, &message)
		if err != nil {
			log.Println("JSON Unmarshal Error:", err)
			continue
		}
		if message.Role == "consumer" {
			if message.Subscribe {
				newPublishService.AddSubscribers(message.Topic, conn)
			} else {
				msg, is_data := newQueueService.DeQueue(message.Topic)
				if is_data {
					conn.WriteJSON(msg)
				}
			}
		} else if message.Role == "producer" {
			if message.TransmissionMode == "buffered" {
				newQueueService.EnQueue(message.Topic, message.Message)

			} else if message.TransmissionMode == "broadcast" {
				newPublishService.SendMessageToSubscribers(message)

			} else {
				log.Printf("Invalid TransmissionMode %s\n", message.TransmissionMode)
			}

		} else {
			log.Printf("Invalid Role %s\n", message.Role)
		}
	}
}

func main() {
	ws := models.WebSocketHandler{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		Subscribers: make(map[string][]*websocket.Conn),
		Queue:       make(map[string][]string),
	}
	handler := &WebSocketService{
		handler: &ws,
	}
	http.Handle("/", handler)
	log.Println("Server listurning :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
