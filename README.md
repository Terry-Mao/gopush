## Terry-Mao/gopush

`Terry-Mao/gopush` is an push server written by golang.

## Requeriments
golang websocket is needed

```sh
# all
$ go get code.google.com/p/go.net/websocket 
```

## Installation

Just pull `Terry-Mao/paint` from github using `go get`:

```sh
$ go get github.com/Terry-Mao/gopush
```

## Usage

```go
sh ./gopush -c ./gopush.conf

open [client](http://localhost:8080/client) and send a sub key

sh redis-cli 
redis > PUBLISH youKey "message"
```

## Documentation

Read the `Terry-Mao/gopush` documentation from a terminal

```go
$ go doc github.com/Terry-Mao/gopush
```

Alternatively, you can [gopush](http://go.pkgdoc.org/github.com/Terry-Mao/gopush).
