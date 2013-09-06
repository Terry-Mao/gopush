package main

import (
	"time"
)

var (
	ConnectedKeyCh = make(chan string, 1024)
)

func CleanConnKeyJob() {
	for {
		key := <-ConnectedKeyCh

		for {
			if err := RedisHDel(ConnectedKey, key); err != nil {
				Log.Printf("RedisHDel(\"%s\", \"%s\") failed (%s)", ConnectedKey, key, err.Error())
				time.Sleep(1 * time.Second)
				continue
			}

			break
		}
	}
}
