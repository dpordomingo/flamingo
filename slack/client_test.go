package slack

import (
	"bytes"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/mvader/flamingo"
	"github.com/mvader/slack"
	"github.com/stretchr/testify/assert"
)

type helloCtrl struct {
	sync.RWMutex
	msgs        []flamingo.Message
	calledIntro int
}

func (*helloCtrl) CanHandle(msg flamingo.Message) bool {
	return msg.Text == "hello"
}

func (c *helloCtrl) Handle(bot flamingo.Bot, msg flamingo.Message) error {
	c.Lock()
	defer c.Unlock()
	c.msgs = append(c.msgs, msg)
	return nil
}

func (c *helloCtrl) HandleIntro(b flamingo.Bot, channel flamingo.Channel) error {
	c.Lock()
	defer c.Unlock()
	c.calledIntro++
	return nil
}

func TestControllerFor(t *testing.T) {
	assert := assert.New(t)
	cli := newClient("", ClientOptions{})
	var ctrl flamingo.Controller = &helloCtrl{}
	cli.AddController(ctrl)

	result, ok := cli.ControllerFor(flamingo.Message{Text: "hello"})
	assert.True(ok)
	assert.Equal(ctrl, result)

	result, ok = cli.ControllerFor(flamingo.Message{Text: "goodbye"})
	assert.False(ok)
	assert.Nil(result)
}

func TestActionHandler(t *testing.T) {
	assert := assert.New(t)
	cli := newClient("", ClientOptions{})
	var handler flamingo.ActionHandler = func(b flamingo.Bot, a flamingo.Action) {
	}
	cli.AddActionHandler("foo", handler)

	result, ok := cli.ActionHandler("foo")
	assert.True(ok)
	assert.Equal(reflect.ValueOf(handler).Pointer(), reflect.ValueOf(result).Pointer())

	result, ok = cli.ActionHandler("bar")
	assert.False(ok)
	assert.Nil(result)
}

func TestRunAndStopWebhook(t *testing.T) {
	assert := assert.New(t)
	cli := newClient("xAB3yVzGS4BQ3O9FACTa8Ho4", ClientOptions{
		WebhookAddr: "127.0.0.1:8989",
	})
	go cli.runWebhook()
	<-time.After(50 * time.Millisecond)

	resp, err := http.Post("http://127.0.0.1:8989", "application/json", bytes.NewBuffer([]byte(testCallback)))
	assert.Nil(err)
	assert.Equal(resp.StatusCode, http.StatusOK)

	cli.shutdownWebhook <- struct{}{}
	<-time.After(50 * time.Millisecond)

	client := http.Client{
		Timeout: 50 * time.Millisecond,
	}
	resp, err = client.Post("http://127.0.0.1:8989", "application/json", bytes.NewBuffer([]byte(testCallback)))
	assert.NotNil(err)
}

type clientBotMock struct {
	sync.RWMutex
	stopped  bool
	actions  []slack.AttachmentActionCallback
	channels []string
}

func (b *clientBotMock) stop() {
	b.Lock()
	defer b.Unlock()
	b.stopped = true
}

func (b *clientBotMock) handleAction(channel string, action slack.AttachmentActionCallback) {
	b.Lock()
	defer b.Unlock()
	b.channels = append(b.channels, channel)
	b.actions = append(b.actions, action)
}

func (b *clientBotMock) handleJob(job flamingo.Job) {

}

func TestRunAndStop(t *testing.T) {
	assert := assert.New(t)
	cli := newClient("xAB3yVzGS4BQ3O9FACTa8Ho4", ClientOptions{
		WebhookAddr:   "127.0.0.1:8787",
		EnableWebhook: true,
		Debug:         true,
	})

	bot := &clientBotMock{}
	bot2 := &clientBotMock{}
	cli.bots["bot"] = bot
	cli.bots["bot2"] = bot2

	var stopped = make(chan struct{}, 1)
	go func() {
		cli.Run()
		stopped <- struct{}{}
	}()

	<-time.After(50 * time.Millisecond)
	resp, err := http.Post("http://127.0.0.1:8787", "application/json", bytes.NewBuffer([]byte(testCallback)))
	assert.Nil(err)
	assert.Equal(resp.StatusCode, http.StatusOK)

	assert.Nil(cli.Stop())
	<-time.After(50 * time.Millisecond)

	select {
	case <-stopped:
	case <-time.After(50 * time.Millisecond):
		assert.FailNow("did not stop")
	}
	assert.True(bot.stopped)
	assert.Equal(1, len(bot.actions))
	assert.Equal("test_callback", bot.actions[0].CallbackID)
	assert.Equal("channel", bot.channels[0])

	assert.True(bot2.stopped)
	assert.Equal(0, len(bot2.actions))
}

func TestSetIntroHandler(t *testing.T) {
	cli := newClient("", ClientOptions{})
	ctrl := &helloCtrl{}
	cli.SetIntroHandler(ctrl)
	assert.Equal(t, reflect.ValueOf(ctrl).Pointer(), reflect.ValueOf(cli.introHandler).Pointer())
}

func TestHandleIntro(t *testing.T) {
	cli := newClient("", ClientOptions{})
	ctrl := &helloCtrl{}
	cli.HandleIntro(nil, flamingo.Channel{})
	cli.SetIntroHandler(ctrl)
	cli.HandleIntro(nil, flamingo.Channel{})
	assert.Equal(t, 1, ctrl.calledIntro)
}

func newClient(token string, options ClientOptions) *slackClient {
	options.Debug = true
	return NewClient(token, options).(*slackClient)
}
