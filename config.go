package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"runtime"
)

var (
	ConfFile string
)

func init() {
	flag.StringVar(&ConfFile, "c", "./gopush.conf", " set gopush config file path")
}

type Config struct {
	Addr                string `json:"addr"`
	Port                int    `json:"port"`
	Pprof               int    `json:"pprof"`
	PprofAddr           string `json:"pprof_addr"`
	PprofPort           int    `json:"pprof_port"`
	PubAddr             string `json:"pub_addr"`
	PubPort             int    `json:"pub_port"`
	LongpollingTimeout  int    `json:"longpolling_timeout"`
	MessageTimeout      int    `json:"message_timeout"`
	Log                 string `json:"log"`
	RedisNetwork        string `json:"redis_network"`
	RedisAddr           string `json:"redis_addr"`
	RedisTimeout        int    `json:"redis_timeout"`
	RedisPoolSize       int    `json:"redis_poolsize"`
	RedisMQSize         int    `json:"redis_mqsize"`
	MaxProcs            int    `json:"max_procs"`
	MaxSubscriberPerKey int    `json:"max_subscriber_per_key"`
}

func InitConfig(file string) (*Config, error) {
	c, err := ioutil.ReadFile(file)
	if err != nil {
		Log.Printf("ioutil.ReadFile(\"%s\") failed (%s)", file, err.Error())
		return nil, err
	}

	cf := &Config{
		Addr:                "localhost",
		Port:                8080,
		PprofAddr:           "localhost",
		PprofPort:           8080,
		PubAddr:             "localhost",
		PubPort:             8080,
		Pprof:               1,
		LongpollingTimeout:  300,
		MessageTimeout:      7200,
		Log:                 "./gopush.log",
		RedisNetwork:        "tcp",
		RedisAddr:           "localhost:6379",
		RedisTimeout:        28800,
		RedisPoolSize:       50,
		RedisMQSize:         20,
		MaxSubscriberPerKey: 0, // no limit
		MaxProcs:            runtime.NumCPU(),
	}
	if err = json.Unmarshal(c, cf); err != nil {
		Log.Printf("json.Unmarshal(\"%s\", cf) failed (%s)", string(c), err.Error())
		return nil, err
	}

	return cf, nil
}
