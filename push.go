package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"runtime/debug"
	"time"
)

const (
	OK             = 0
	InternalErr    = 65535
	AuthErr        = 1
	MultiCliErr    = 2
	OKStr          = "OK"
	InternalErrStr = "Internal Exception"
	AuthErrStr     = "Authentication Exception"
	MultiCliStr    = "Mutiple Client Connected Exception"
)

var (
	errMsg = map[int]string{}
	pusher Pusher
    protocolErr = errors.New("client must not send any data")
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
	errMsg[MultiCliErr] = MultiCliStr

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
	// cleanup on server side
	defer func() {
		if err := ws.Close(); err != nil {
			Log.Printf("wc.Close() failed (%s)", err.Error())
		}
	}()
	// set read deadline
	err := ws.SetReadDeadline(time.Now().Add(time.Duration(Conf.LongpollingTimeout) * time.Second))
	if err != nil {
		Log.Printf("ws.SetReadDeadline() failed (%s)", err.Error())
		return
	}
	// get key
	if err = websocket.Message.Receive(ws, &key); err != nil {
		Log.Printf("websocket.Message.Receive failed (%s)", err.Error())
		return
	}
	// Auth
	if !pusher.Auth(key) {
		if err = responseWriter(ws, AuthErr, result); err != nil {
			Log.Printf("responseWriter failed (%s)", err.Error())
		}

		return
	}
	// Generate Key
    key = pusher.Key(key)
	Log.Printf("Client (%s) subscribe to key %s", ws.Request().RemoteAddr, key)
	// check multi cli connected
	if Conf.MaxSubscriberPerKey > 0 {
		ok, err := multiCliCheck(key)
		if err != nil {
			if err = responseWriter(ws, InternalErr, result); err != nil {
				Log.Printf("responseWriter failed (%s)", err.Error())
			}

			return
		}

		defer func() {
			if err = RedisDecr(ConnectedKey, key); err != nil {
				Log.Printf("RedisDecr(\"%s\", \"%s\") failed (%s)", ConnectedKey, key, err.Error())
			}
		}()

		if !ok {
			if err = responseWriter(ws, MultiCliErr, result); err != nil {
				Log.Printf("responseWriter failed (%s)", err.Error())
			}

			return
		}
	}
	// redis routine for receive pub message or error
	redisC, psc, err := RedisSub(key)
	if err != nil {
		Log.Printf("RedisSub(\"%s\") failed (%s)", key, err.Error())
		if err = responseWriter(ws, InternalErr, result); err != nil {
			Log.Printf("responseWriter failed (%s)", err.Error())
		}

		return
	}
	// unsub redis
	defer func() {
		if err := RedisUnSub(key, psc); err != nil {
			Log.Printf("RedisUnSub(\"%s\", psc) failed (%s)", key, err.Error())
		}
	}()
	// create a routine wait for client read(only closed or error) return a channel
	netC := netRead(ws)
	for {
		select {
		case err := <-netC:
			Log.Printf("websocket.Message.Receive faild (%s)", err.Error())
			return
		case msg := <-redisC:
			if rmsg, ok := msg.(string); ok {
				result["data"] = rmsg
				if err = responseWriter(ws, OK, result); err != nil {
					Log.Printf("responseWriter failed (%s)", err.Error())
					// Restore the unsent message
					if err = RedisRestore(key, rmsg); err != nil {
						Log.Printf("RedisRestore(\"%s\", \"%s\") failed", key, msg)
						return
					}

					return
				}
			} else if err, ok := msg.(error); ok {
				// DEBUG
				Log.Printf("Subscribe() failed (%s)", err.Error())
				return
			} else {
				Log.Printf("Unknown msg in RedisSub")
				return
			}
		}
	}
}

func multiCliCheck(key string) (bool, error) {
	// incr client num
	cliNum, err := RedisIncr(ConnectedKey, key)
	if err != nil {
		Log.Printf("RedisIncr(\"%s\", \"%s\") failed (%s)", ConnectedKey, key, err.Error())
		return false, err
	}
	// check
	if cliNum > Conf.MaxSubscriberPerKey {
		Log.Printf("key %s has %d subscribers exceed %d", key, cliNum, Conf.MaxSubscriberPerKey)
		return false, nil
	}

	return true, nil
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
			c <- protocolErr
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
