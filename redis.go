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
	mqPrefix     = "Q"
	ConnectedKey = "Connected"
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

	if Conf.MaxSubscriberPerKey > 0 {
		// init clean connected key job
		go CleanConnKeyJob()
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
	// refresh message timedout
	if err := redisExpire(c, key); err != nil {
		Log.Printf("redisExpire(c, \"%s\") failed (%s)", key, err.Error())
		return err
	}
	// check message queue
	for {
		reply, err := c.Do("LPOP", key)
		if err != nil {
			Log.Printf("c.Do(\"LPOP\", \"%s\") failed (%s)", mqPrefix+key, err.Error())
			return err
		}

		if reply == nil {
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

func RedisRestore(key, msg string) error {
	qkey := mqPrefix + key
	c := redisPool.Get()
	defer c.Close()
	// RPUSH, LTRIM, EXPIRE
	qc, err := redisLPush(c, qkey, msg)
	if err != nil {
		Log.Printf("redisLPush(c, \"%s\", \"%s\") failed (%s)", qkey, msg, err.Error())
		return err
	}
	// truncate old message
	if qc > Conf.RedisMQSize {
		if err = redisLTrim(c, qkey); err != nil {
			Log.Printf("redisLTrim(c, \"%s\") failed (%s)", qkey, err.Error())
			return err
		}
	}
	// set message timedout
	if err = redisExpire(c, qkey); err != nil {
		Log.Printf("redisExpire(c, \"%s\") failed (%s)", qkey, err.Error())
		return err
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
		qc, err := redisRPush(c, qkey, msg)
		if err != nil {
			Log.Printf("redisRPush(c, \"%s\", \"%s\") failed (%s)", qkey, msg, err.Error())
			return err
		}
		// truncate old message
		if qc > Conf.RedisMQSize {
			if err = redisLTrim(c, qkey); err != nil {
				Log.Printf("redisLTrim(c, \"%s\") failed (%s)", qkey, err.Error())
				return err
			}
		}
		// set message timedout
		if err = redisExpire(c, qkey); err != nil {
			Log.Printf("redisExpire(c, \"%s\") failed (%s)", qkey, err.Error())
			return err
		}
	}

	return nil
}

func redisExpire(c redis.Conn, key string) error {
	if Conf.MessageTimeout > 0 {
		_, err := c.Do("EXPIRE", key, Conf.MessageTimeout)
		if err != nil {
			Log.Printf("c.Do(\"EXPIRE\", \"%s\", %d) failed (%s)", key, Conf.MessageTimeout, err.Error())
			return err
		}
	}

	return nil
}

func redisRPush(c redis.Conn, qkey, msg string) (int, error) {
	reply, err := c.Do("RPUSH", qkey, msg)
	if err != nil {
		Log.Printf("c.Do(\"RPUSH\", \"%s\", \"%s\") failed (%s)", qkey, msg, err.Error())
		return 0, err
	}

	qc, err := redis.Int(reply, nil)
	if err != nil {
		Log.Printf("redis.Int() failed (%s)", err.Error())
		return 0, err
	}

	return qc, nil
}

func redisLPush(c redis.Conn, qkey, msg string) (int, error) {
	reply, err := c.Do("LPUSH", qkey, msg)
	if err != nil {
		Log.Printf("c.Do(\"LPUSH\", \"%s\", \"%s\") failed (%s)", qkey, msg, err.Error())
		return 0, err
	}

	qc, err := redis.Int(reply, nil)
	if err != nil {
		Log.Printf("redis.Int() failed (%s)", err.Error())
		return 0, err
	}

	return qc, nil
}

func redisLTrim(c redis.Conn, qkey string) error {
	reply, err := c.Do("LTRIM", qkey, 1, -1)
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

	return nil
}

// Integer reply, specifically:
// 1 if field is a new field in the hash and value was set.
// 0 if field already exists in the hash and no operation was performed.
func RedisHSetnx(key, field, value string) (int, error) {
	c := redisPool.Get()
	defer c.Close()
	reply, err := c.Do("HSETNX", key, field, value)
	if err != nil {
		Log.Printf("c.Do(\"HSETNX\", \"%s\", \"%s\", \"%s\") failed (%s)", key, field, value, err.Error())
		return 0, err
	}

	v, err := redis.Int(reply, nil)
	if err != nil {
		Log.Printf("redis.Int() failed (%s)", err.Error())
		return 0, err
	}

	return v, nil
}

func RedisHExists(key, field string) (int, error) {
	c := redisPool.Get()
	defer c.Close()
	reply, err := c.Do("HEXISTS", key, field)
	if err != nil {
		Log.Printf("c.Do(\"HEXISTS\", \"%s\") failed (%s)", key, err.Error())
		return 0, err
	}

	v, err := redis.Int(reply, nil)
	if err != nil {
		Log.Printf("redis.Int() failed (%s)", err.Error())
		return 0, err
	}

	return v, nil
}

func RedisHGet(key, field string) (int, error) {
	c := redisPool.Get()
	defer c.Close()
	reply, err := c.Do("HGET", key, field)
	if err != nil {
		Log.Printf("c.Do(\"HGET\", \"%s\") failed (%s)", key, err.Error())
		return 0, err
	}

	if reply == nil {
		return -1, nil
	}

	v, err := redis.Int(reply, nil)
	if err != nil {
		Log.Printf("redis.Int() failed (%s)", err.Error())
		return 0, err
	}

	return v, nil
}

func RedisIncr(key, field string) (int, error) {
	c := redisPool.Get()
	defer c.Close()
	reply, err := c.Do("HINCRBY", key, field, 1)
	if err != nil {
		Log.Printf("c.Do(\"HINCRBY\", \"%s\", 1) failed (%s)", key, err.Error())
		return 0, err
	}

	v, err := redis.Int(reply, nil)
	if err != nil {
		Log.Printf("redis.Int() failed (%s)", err.Error())
		return 0, err
	}

	return v, nil
}

func RedisDecr(key, field string) error {
	c := redisPool.Get()
	defer c.Close()
	_, err := c.Do("HINCRBY", key, field, -1)
	if err != nil {
		Log.Printf("c.Do(\"HINCRBY\", \"%s\", -1) failed (%s)", key, err.Error())
		ConnectedKeyCh <- key
		return err
	}

	return nil
}

func RedisHDel(key, field string) error {
	c := redisPool.Get()
	defer c.Close()
	_, err := c.Do("HDEL", key, field)
	if err != nil {
		Log.Printf("c.Do(\"HDEL\", \"%s\", \"%s\") failed (%s)", key, field, err.Error())
		return err
	}

	return nil
}
