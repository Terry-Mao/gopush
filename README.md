## Terry-Mao/gopush

`Terry-Mao/gopush` is an push server written by golang. (redis + websocket)

## Requeriments
regisgo and golang websocket is needed.
```sh
# all platform
# redis
$ go get github.com/garyburd/redigo
# websocket
$ go get code.google.com/p/go.net/websocket 
```

## Installation
Just pull `Terry-Mao/gopush` from github using `go get`:

```sh
$ go get github.com/Terry-Mao/gopush
```

## Usage
```sh
$ ./gopush -c ./gopush.conf

# open [client](http://localhost:8080/client) and send a sub key

$ redis-cli 
$ redis > PUBLISH youKey "message"

# then your browser will alert the "message"
```

## Protocol
Subscribers first send a key to the gopush and will receive a json response 
when someone publish a message to the key.
ret:
* 0 : ok
* 65535 : internal error
* 1 : authentication error
msg:
* error message
data:
* the publish message
the reponse json examples:
```json
{
    "ret" : 0,
    "msg" : "ok",
    "data" : "message"
}
```

## Documentation
Read the `Terry-Mao/gopush` documentation from a terminal

```go
$ go doc github.com/Terry-Mao/gopush
```

Alternatively, you can [gopush](http://go.pkgdoc.org/github.com/Terry-Mao/gopush).
