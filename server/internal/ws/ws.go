package ws

import (
	"kontroler-server/internal/db"
	"log"
	"time"

	"github.com/gofiber/websocket/v2"
	"k8s.io/client-go/dynamic"
)

type WebSocketLogStream struct {
	db        db.DbManager
	kubClient dynamic.Interface
}

func NewWebSocketLogStream(db db.DbManager, kubClient dynamic.Interface) *WebSocketLogStream {
	return &WebSocketLogStream{
		db:        db,
		kubClient: kubClient,
	}
}

type LogRequest struct {
	Action string `json:"action"`
	PodUID string `json:"podUID"`
}

func (w *WebSocketLogStream) StreamLogs(c *websocket.Conn) {
	defer c.Close()

	podUUID := c.Query("pod")
	if podUUID == "" {
		log.Println("❌ No pod UUID provided, closing connection")
		c.WriteMessage(websocket.CloseMessage, []byte("Missing pod UUID"))
		return
	}

	// Simulated log streaming
	for i := 1; i <= 50; i++ {
		logMessage := []byte("Log Entry " + podUUID)
		if err := c.WriteMessage(websocket.TextMessage, logMessage); err != nil {
			log.Println("WebSocket write error:", err)
			break
		}

		time.Sleep(1 * time.Second)
	}
}
