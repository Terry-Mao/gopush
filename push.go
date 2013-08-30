package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime/debug"
	"time"
)

const (
	OK             = 0
	InternalErr    = 65535
	AuthErr        = 1
	OKStr          = "OK"
	InternalErrStr = "Internal Exception"
	AuthErrStr     = "Authentication Exception"
)

var (
	errMsg = map[int]string{}
	pusher Pusher
)

type Pusher interface {
	Auth(key string) bool
	Key(key string) string
}

type DefPusher struct{}

func (p *DefPusher) Auth(key string) bool {
	return true
}

func (p *DefPusher) Key(key string) string {
	return key
}

func init() {
	errMsg[OK] = OKStr
	errMsg[InternalErr] = InternalErrStr
	errMsg[AuthErr] = AuthErrStr

	pusher = &DefPusher{}
}

func Publish(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
	}

	params := r.URL.Query()
	key := params.Get("key")
	if key == "" {
		http.Error(w, "must specified query parameter ?key=your_pub_key", 405)
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body error", 500)
	}

	// send to redis
	if err = RedisPub(key, string(body)); err != nil {
		http.Error(w, "interlanl redis error", 500)
	}
}

func SetPusher(p Pusher) {
	pusher = p
}

func Subscribe(ws *websocket.Conn) {
	var (
		result = map[string]interface{}{}
		key    string
	)

	defer recoverFunc()
	// set read deadline
	err := ws.SetReadDeadline(time.Now().Add(time.Duration(Conf.LongpollingTimeout) * time.Second))
	if err != nil {
		Log.Printf("ws.SetReadDeadline() failed (%s)", err.Error())
		return
	}
	// get key
	if err = websocket.Message.Receive(ws, &key); err != nil {
		Log.Printf("websocket.Message.Receive failed (%s)", err.Error())
		if err = responseWriter(ws, InternalErr, result); err != nil {
			Log.Printf("responseWriter failed (%s)", err.Error())
			return
		}
	}
	// Auth
	if !pusher.Auth(key) {
		if err = responseWriter(ws, AuthErr, result); err != nil {
			Log.Printf("responseWriter failed (%s)", err.Error())
			return
		}
	}
	// Generate Key
	key = pusher.Key(key)
	// create a routine wait for client read(only closed or error) return a channel
	netC := netRead(ws)
	redisC, psc, err := RedisSub(key)
    if err != nil {
        Log.Printf("RedisSub(\"%s\") failed (%s)", key, err.Error())
        return
    }

	defer RedisUnSub(key, psc)
	for {
		select {
		case err := <-netC:
			Log.Printf("websocket.Message.Receive faild (%s)", err.Error())
			return
		case msg := <-redisC:
			if err, ok := msg.(error); !ok {
				result["data"] = msg
				if err = responseWriter(ws, OK, result); err != nil {
					Log.Printf("responseWriter failed (%s)", err.Error())
                    // Restore the unsent message
                    if err = RedisPub(key, msg); err != nil {
                        Log.Printf("RedisPub(\"%s\", \"%s\") failed", key, msg)
                        return
                    }

					return
				}
			} else {
				// DEBUG
				Log.Printf("Subscribe() failed (%s)", err.Error())
				return
			}
		}
	}
}

func netRead(ws *websocket.Conn) chan error {
	c := make(chan error, 1)
	// client close or network error, go routine exit
	go func() {
		// DEBUG
		Log.Printf("netRead routine start")
		var reply string
		if err := websocket.Message.Receive(ws, &reply); err != nil {
			Log.Printf("websocket.Message.Receive() failed (%s)", err.Error())
			c <- err
		} else {
			c <- fmt.Errorf("client must not send any data")
		}
		// DEBUG
		Log.Printf("netRead routine exit")
	}()

	return c
}

func recoverFunc() {
	if err := recover(); err != nil {
		Log.Printf("Error : %v, Debug : \n%s", err, string(debug.Stack()))
	}
}

func responseWriter(ws *websocket.Conn, ret int, result map[string]interface{}) error {
	result["ret"] = ret
	result["msg"] = getErrMsg(ret)
	strJson, err := json.Marshal(result)
	if err != nil {
		Log.Printf("json.Marshal(\"%v\") failed", result)
		return err
	}

	respJson := string(strJson)
	Log.Printf("Respjson : %s", respJson)
	if _, err := ws.Write(strJson); err != nil {
		Log.Printf("ws.Write(\"%s\") failed (%s)", respJson, err.Error())
		return err
	}

	return nil
}

func getErrMsg(ret int) string {
	if msg, ok := errMsg[ret]; !ok {
		return ""
	} else {
		return msg
	}
}
