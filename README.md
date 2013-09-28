# Terry-Mao/gopush2
`Terry-Mao/gopush` is developing which don't use redis anymore, cause it's double tcp connection (redis pub/sub) waste of many memory.

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
# start the gopush server
$ ./gopush -c ./gopush.conf

# 1. open http://localhost:8080/client in browser and press the Send button
# 2. you can use curl
$ curl -d "test" http://localhost:8080/pub?key=yourkey
# 3. you can use redis 
$ redis-cli 
$ redis > PUBLISH youKey "message"

# then your browser will alert the "message"
```
a simple java client example
```java
import java.io.IOException;
import java.util.concurrent.ExecutionException;

import com.ning.http.client.*;
import com.ning.http.client.websocket.*;

public class test {

	/**
	 * @param args
	 * @throws IOException
	 * @throws ExecutionException
	 * @throws InterruptedException
	 */
	public static void main(String[] args) throws InterruptedException,
			ExecutionException, IOException {

		AsyncHttpClient c = new AsyncHttpClient();

		WebSocket websocket = c
				.prepareGet("ws://10.33.30.66:8080/sub")
				.execute(
						new WebSocketUpgradeHandler.Builder()
								.addWebSocketListener(
										new WebSocketTextListener() {

											@Override
											public void onMessage(String message) {
												System.out.println(message);
											}

											@Override
											public void onOpen(
													WebSocket websocket) {
												System.out.println("ok");
											}

											public void onClose(
													WebSocket websocket) {
											}

											@Override
											public void onError(Throwable t) {
											}

											@Override
											public void onFragment(String arg0,
													boolean arg1) {
												// TODO Auto-generated method
												// stub

											}
										}).build()).get();

		websocket.sendTextMessage("Terry-Mao");
		Thread.sleep(100000000);
	}
}

```

## Protocol
Subscribers use `websocket` connect to the `gopush` then write a `sub key` to
the server and it will receive a json response when someone publish a message 
to the key in your `redis`.

response json field:
### ret
* 0 : ok
* 65535 : internal error
* 1 : authentication error

### msg
* error message

### data
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

```sh
$ go doc github.com/Terry-Mao/gopush
```

Alternatively, you can [gopush](http://go.pkgdoc.org/github.com/Terry-Mao/gopush).
