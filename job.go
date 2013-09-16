package main

import (
	"time"
)

var (
	ConnectedKeyCh = make(chan string, 1024)
)

// job for clean connected client flag
func CleanConnKeyJob() {
	for {
		key := <-ConnectedKeyCh
		if err := RedisDecr(ConnectedKey, key); err != nil {
			Log.Printf("RedisDecrBy(\"%s\", \"%s\") failed (%s)", ConnectedKey, key, err.Error())
			// if failed, sleep 1 second and retry
			time.Sleep(1 * time.Second)
		}
	}
}
