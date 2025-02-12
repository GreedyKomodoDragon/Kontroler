package ws

import (
	"context"
	"kontroler-server/internal/db"
	"log"
	"time"

	"github.com/gofiber/websocket/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type WebSocketLogStream struct {
	db        db.DbManager
	clientSet *kubernetes.Clientset
}

func NewWebSocketLogStream(db db.DbManager, clientSet *kubernetes.Clientset) *WebSocketLogStream {
	return &WebSocketLogStream{
		db:        db,
		clientSet: clientSet,
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
		c.WriteMessage(websocket.CloseMessage, []byte("Missing pod UUID"))
		return
	}

	namespace, name, err := w.db.GetPodNameAndNamespace(context.Background(), podUUID)
	if err != nil {
		c.WriteMessage(websocket.CloseMessage, []byte("Failed to get logs"))
		return
	}

	// Get logs from the pod
	req := w.clientSet.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{
		Follow: true,
	})

	logStream, err := req.Stream(context.TODO())
	if err != nil {
		c.WriteMessage(websocket.CloseMessage, []byte("Failed to get logs"))
		return
	}
	defer logStream.Close()

	for {
		buf := make([]byte, 1024)
		n, err := logStream.Read(buf)
		if err != nil {
			log.Println("Error reading log stream:", err)
			break
		}

		err = c.WriteMessage(websocket.TextMessage, buf[:n])
		if err != nil {
			log.Println("Error writing message to WebSocket:", err)
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	c.WriteMessage(websocket.CloseMessage, []byte("finished"))
}
