package main

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"time"
)

var (
	redisPool *redis.Pool
)

const (
	mqPrefix = "Q"
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

func RedisSub(key string) (chan interface{}, redis.PubSubConn, error) {
	mq := make(chan interface{}, Conf.RedisMQSize)
	c := redisPool.Get()
	defer c.Close()
	pc, err := redis.Dial(Conf.RedisNetwork, Conf.RedisAddr)
	if err != nil {
		Log.Printf("redis.Dial(\"%s\", \"%s\") failed (%s)", Conf.RedisNetwork, Conf.RedisAddr, err.Error())
		return nil, redis.PubSubConn{}, err
	}

	psc := redis.PubSubConn{pc}
	// check queue
	err = redisQueue(c, key, mq)
	if err != nil {
		Log.Printf("redisQueue failed (%s)", err.Error())
		return nil, redis.PubSubConn{}, err
	}

	// subscribe
	psc.Subscribe(key)
	if _, ok := psc.Receive().(redis.Subscription); !ok {
		Log.Printf("init sub must redis.Subscription")
		return nil, redis.PubSubConn{}, fmt.Errorf("first sub must init")
	}

	// double check
	err = redisQueue(c, key, mq)
	if err != nil {
		Log.Printf("redisQueue failed (%s)", err.Error())
		return nil, redis.PubSubConn{}, err
	}

	go func() {
		// DEBUG
		Log.Printf("redis routine start")
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
				Log.Printf("redis UnSubscrption")
				return
			case error:
				Log.Printf("psc.Receive() failed (%s)", n.Error())
				mq <- n
				return
			}
		}
	}()

	return mq, psc, nil
}

func redisQueue(c redis.Conn, key string, mq chan interface{}) error {
	key = mqPrefix + key
	// check message queue
	for {
		reply, err := c.Do("LPOP", key)
		if err != nil {
			Log.Printf("c.Do(\"LPOP\", \"%s\") failed (%s)", mqPrefix+key, err.Error())
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

func RedisPub(key, msg string) error {
	qkey := mqPrefix + key
	c := redisPool.Get()
	defer c.Close()
	reply, err := c.Do("PUBLISH", key, msg)
	if err != nil {
		Log.Printf("c.Do(\"PUBLISH\", \"%s\", \"%s\") failed (%s)", key, msg, err.Error())
		return err
	}

	active, err := redis.Int(reply, nil)
	if err != nil {
		Log.Printf("redis.Int() failed (%s)", err.Error())
		return err
	}

	// send to message queue
	if active == 0 {
		// RPUSH, LTRIM, EXPIRE
		reply, err = c.Do("RPUSH", qkey, msg)
		if err != nil {
			Log.Printf("c.Do(\"RPUSH\", \"%s\", \"%s\") failed (%s)", qkey, msg, err.Error())
			return err
		}

		qc, err := redis.Int(reply, nil)
		if err != nil {
			Log.Printf("redis.Int() failed (%s)", err.Error())
			return err
		}
		// truncate old message
		if qc > Conf.RedisMQSize {
			reply, err = c.Do("LTRIM", qkey, 1, -1)
			if err != nil {
				Log.Printf("c.Do(\"LTRIM\", \"%s\", 1, -1) failed (%s)", qkey, err.Error())
				return err
			}

			status, err := redis.String(reply, nil)
			if err != nil {
				Log.Printf("redis.String() failed (%s)", err.Error())
				return err
			}

			if status != "OK" {
				return fmt.Errorf("LTRIM failed")
			}
		}
		// set message timedout
		if Conf.MessageTimeout > 0 {
			reply, err = c.Do("EXPIRE", qkey, Conf.MessageTimeout)
			if err != nil {
				Log.Printf("c.Do(\"EXPIRE\", \"%s\", %d) failed (%s)", qkey, Conf.MessageTimeout, err.Error())
				return err
			}

			status, err := redis.Int(reply, nil)
			if err != nil {
				Log.Printf("redis.String() failed (%s)", err.Error())
				return err
			}

			if status != 1 {
				return fmt.Errorf("EXPIRE failed")
			}
		}
	}

	return nil
}
