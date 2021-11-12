To run: `go run .`
To test:

``
pip install websocket-client
``

And then:

```
from websocket import create_connection
ws = create_connection("ws://localhost:9000/connect")
ws.recv() # 'Tick at 2021-11-12 17:38:26.106213388 -0500 EST m=+334.502278142'
```

For now it just ticks to the connected clients.
