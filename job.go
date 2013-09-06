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

		for {
			if err := RedisHDel(ConnectedKey, key); err != nil {
				Log.Printf("RedisHDel(\"%s\", \"%s\") failed (%s)", ConnectedKey, key, err.Error())
				// if failed, sleep 1 second and retry
				time.Sleep(1 * time.Second)
				continue
			}

			break
		}
	}
}
