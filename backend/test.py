# %%
import websocket
import threading
import json
from pprint import pprint
from IPython.display import clear_output
import time

# websocket.enableTrace(True)
websocket.enableTrace(False)

# %%
class TestClient:
    def __init__(self, name, verbose):
        self.name = name
        self.id = None
        self.wsapp = None
        self.wst = None
        self.sendt = None
        self.verbose = verbose
    
    def start(self):
        self.wsapp = websocket.WebSocketApp(f"ws://localhost:10000/connect", header={'name': self.name}, on_message=self.on_message, on_close=self.on_close)
        self.wst = threading.Thread(target=self.wsapp.run_forever)
        self.wst.daemon = True
        self.wst.start()

    def stop(self):
        self.wsapp.close()
        if self.wst:
            self.wst.join()
            self.wst = None
        if self.sendt:
            self.sendt.join()
            self.sendt = None

    def send_updates(self):
        while True:
            try:
                self.wsapp.send(json.dumps({'PlayerId': self.id, 'Position': {'X': 1000, 'Y': 1000}, 'Direction': {'X': -1, 'Y': 1}}))
            except:
                return
            if self.verbose:
                print('send_updates', self.id)
            time.sleep(0.01)

    def on_message(self, _ws, message):
        if self.id == None:
            self.id = message
            if self.verbose:
                print('id', self.id)
            self.sendt = threading.Thread(target=self.send_updates)
            self.sendt.daemon = True
            self.sendt.start()
            return
        message = json.loads(message)
        if message['Status'] == 2 or message['Status'] == 3:
            if self.verbose:
                print('game ended', self.id)
            self.stop()
            
        if self.verbose:
            print('message', self.id, message)

    def on_close(self, ws, close_status_code, close_msg):
        if self.verbose:
            print('close', self.id, close_status_code, close_msg)

# %%
N = 10
clients = [TestClient(f"test {i}", verbose=(i==N-1)) for i in range(N)]
print(len(clients), 'clients')
for client in clients:
    client.start()
time.sleep(3)
for i, client in enumerate(clients):
    print('Stopping client', client.id)
    client.stop()
    time.sleep(0.001 * i)
