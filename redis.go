package main

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"time"
)

var (
	redisPool *redis.Pool
)

func init() {
}

func InitRedis() {
	//Create redis conn
	redisPool = &redis.Pool{
		MaxIdle:     Conf.RedisPoolSize,
		IdleTimeout: time.Duration(Conf.RedisTimeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial(Conf.RedisNetwork, Conf.RedisAddr)
			if err != nil {
				Log.Printf("redis.Dial(\"%s\", \"%s\") failed (%s)", Conf.RedisNetwork, Conf.RedisAddr, err.Error())
				panic(err)
			}
			return c, err
		},
	}
}

func RedisUnSub(key string, psc redis.PubSubConn) error {
	if err := psc.Unsubscribe(key); err != nil {
		Log.Printf("psc.Unsubscribe(\"%s\") faild (%s)", key, err.Error())
		return err
	}

	return nil
}

func RedisSub(key string) (mq chan interface{}, psc redis.PubSubConn) {
	mq = make(chan interface{}, Conf.RedisMQSize)
	c := redisPool.Get()
	defer c.Close()
	pc, err := redis.Dial(Conf.RedisNetwork, Conf.RedisAddr)
	if err != nil {
		Log.Printf("redis.Dial(\"%s\", \"%s\") failed (%s)", Conf.RedisNetwork, Conf.RedisAddr, err.Error())
		mq <- err
		return
	}

	psc = redis.PubSubConn{pc}
	// check queue
	err = redisQueue(c, key, mq)
	if err != nil {
		Log.Printf("redisQueue failed (%s)", err.Error())
		mq <- err
		return
	}

	// subscribe
	psc.Subscribe(key)
	n := psc.Receive()
	if _, ok := n.(redis.Subscription); !ok {
		Log.Printf("init sub must redis.Subscription")
		mq <- fmt.Errorf("first sub must init")
		return
	}

	// double check
	err = redisQueue(c, key, mq)
	if err != nil {
		Log.Printf("redisQueue failed (%s)", err.Error())
		mq <- err
		return
	}

	go func() {
		// DEBUG
		defer Log.Printf("redis routine exit")
		defer psc.Close()
		for {
			switch n := psc.Receive().(type) {
			case redis.Message:
				mq <- string(n.Data)
			case redis.PMessage:
				mq <- string(n.Data)
			case redis.Subscription:
				// DEBUG
				// Log.Printf("redis UnSubscrption")
				return
			case error:
				Log.Printf("psc.Receive() failed (%s)", n.Error())
				mq <- n
				return
			}
		}
	}()

	return
}

func redisQueue(c redis.Conn, key string, mq chan interface{}) error {
	// check message queue
	for {
		reply, err := c.Do("LPOP", key)
		if err != nil {
			Log.Printf("c.Do(\"LPOP\", \"%s\") failed (%s)", key, err.Error())
			return err
		}

		if reply == nil {
			// empty
			break
		}

		msg, err := redis.String(reply, nil)
		if err != nil {
			Log.Printf("redis.String() failed (%s)", err.Error())
			return err
		}

		mq <- msg
	}

	return nil
}
