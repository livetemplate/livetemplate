package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/livetemplate/livetemplate"
	"github.com/livetemplate/livetemplate/pubsub"
	"github.com/redis/go-redis/v9"
)

type fixedGroupAuth struct {
	groupID string
}

func (a *fixedGroupAuth) Identify(_ *http.Request) (string, error) { return "", nil }
func (a *fixedGroupAuth) GetSessionGroup(_ *http.Request, _ string) (string, error) {
	return a.groupID, nil
}

type Message struct {
	User string `json:"user"`
	Text string `json:"text"`
}

// ChatController uses Redis for shared message storage so both instances
// see the same messages. This is the realistic pattern for multi-instance.
type ChatController struct {
	rdb *redis.Client
}

type ChatState struct {
	Messages    []Message
	CurrentUser string
	InstanceID  string
}

const messagesKey = "e2e:chat:messages"

func (c *ChatController) loadMessages() []Message {
	ctx := context.Background()
	data, err := c.rdb.LRange(ctx, messagesKey, 0, -1).Result()
	if err != nil {
		return nil
	}
	msgs := make([]Message, 0, len(data))
	for _, d := range data {
		var m Message
		if err := json.Unmarshal([]byte(d), &m); err == nil {
			msgs = append(msgs, m)
		}
	}
	return msgs
}

func (c *ChatController) Mount(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	state.Messages = c.loadMessages()
	state.InstanceID = os.Getenv("INSTANCE_ID")
	return state, nil
}

func (c *ChatController) OnConnect(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	state.CurrentUser = ""
	state.Messages = c.loadMessages()
	state.InstanceID = os.Getenv("INSTANCE_ID")
	return state, nil
}

func (c *ChatController) Join(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	state.CurrentUser = ctx.GetString("username")
	return state, nil
}

func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	text := ctx.GetString("message")
	if text == "" || state.CurrentUser == "" {
		return state, nil
	}

	msg := Message{User: state.CurrentUser, Text: text}
	data, _ := json.Marshal(msg)
	c.rdb.RPush(context.Background(), messagesKey, data)

	state.Messages = c.loadMessages()
	ctx.BroadcastAction("RefreshMessages", nil)
	return state, nil
}

func (c *ChatController) RefreshMessages(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	state.Messages = c.loadMessages()
	return state, nil
}

func main() {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "local"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Fatal("REDIS_URL is required")
	}

	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}
	rdb := redis.NewClient(redisOpts)

	// Clear test data on startup
	rdb.Del(context.Background(), messagesKey)

	broadcaster := pubsub.NewRedisBroadcaster(rdb)
	auth := &fixedGroupAuth{groupID: "e2e-test-group"}

	opts := []livetemplate.Option{
		livetemplate.WithTemplateBaseDir("/app"),
		livetemplate.WithAuthenticator(auth),
		livetemplate.WithPubSubBroadcaster(broadcaster),
	}

	tmpl, err := livetemplate.New("chat", opts...)
	if err != nil {
		log.Fatal(err)
	}
	tmpl, err = tmpl.Parse(chatTemplate)
	if err != nil {
		log.Fatal(err)
	}

	ctrl := &ChatController{rdb: rdb}
	handler := tmpl.Handle(ctrl, livetemplate.AsState(&ChatState{}))

	http.Handle("/", handler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintf(w, "ok:%s", instanceID)
	})

	log.Printf("[%s] Redis PubSub enabled: %s", instanceID, redisURL)
	log.Printf("[%s] Listening on :%s", instanceID, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

const chatTemplate = `<!DOCTYPE html>
<html>
<head><title>Chat E2E Test</title></head>
<body>
<div id="chat">
<div id="instance">Instance: {{.InstanceID}}</div>
{{if .CurrentUser}}
<div id="user">Logged in as: {{.CurrentUser}}</div>
<div id="messages">
{{range .Messages}}
<div class="msg" data-key="{{.User}}-{{.Text}}"><b>{{.User}}</b>: {{.Text}}</div>
{{end}}
</div>
<form name="send">
<input name="message" type="text" required>
<button type="submit">Send</button>
</form>
{{else}}
<div id="join-form">
<form name="join">
<input name="username" type="text" required placeholder="Enter username">
<button type="submit">Join</button>
</form>
</div>
{{end}}
</div>
<script src="https://cdn.jsdelivr.net/npm/@livetemplate/client@0.8.7/dist/livetemplate-client.browser.js"></script>
</body>
</html>`
