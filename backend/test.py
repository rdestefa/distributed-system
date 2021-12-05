# %%
import websocket
import threading
import json
from pprint import pprint
import time
import datetime
import math
import random

# websocket.enableTrace(True)
websocket.enableTrace(False)

# %%

MOVE_SPEED = 120.0

class DateTimeJSONEncoder(json.JSONEncoder):
    def default(self, obj):
        if isinstance(obj, datetime.datetime):
            return obj.isoformat("T") + "Z"
        else:
            return super(DateTimeJSONEncoder, self).default(obj)

class TestClient:
    def __init__(self, name, verbose):
        self.name = name
        self.wsapp = None
        self.wst = None
        self.sendt = None
        self.verbose = verbose
    
    def start(self):
        self.wsapp = websocket.WebSocketApp(f"ws://localhost:10000/connect", header={'name': self.name}, on_message=self.on_message, on_close=self.on_close)
        self.wst = threading.Thread(target=self.wsapp.run_forever)
        self.wst.daemon = True
        self.wst.start()

        self.id = None
        self.game_started = False
        self.last_message = datetime.datetime.utcnow()
        self.last_position = {'X': 0, 'Y': 0}
        self.alive = True

    def stop(self):
        self.wsapp.close()
        if self.wst:
            self.wst.join()
            self.wst = None
        if self.sendt:
            self.sendt.join()
            self.sendt = None
    
    def send(self, data):
        now = datetime.datetime.utcnow()
        data = data | {'PlayerId': self.id, 'TimeStamp': now}
        msg = DateTimeJSONEncoder().encode(data)
        if self.verbose:
            print('send', self.id, msg)
        self.wsapp.send(msg)
        self.last_message = now

    def send_updates(self):
        same_direction = 0
        while True:
            try:
                if not self.game_started or not self.alive:
                    self.send({})
                else:
                    if same_direction == 0:
                        angle = 2 * math.pi * random.random()
                    same_direction = (same_direction + 1) % 10
                    duration = (datetime.datetime.utcnow() - self.last_message).total_seconds()
                    r =  duration * MOVE_SPEED
                    dx = r * math.cos(angle)
                    dy = r * math.sin(angle)
                    new_position = {'X': self.last_position['X'] + dx, 'Y': self.last_position['Y'] + dy}
                    new_direction = {'X': math.cos(angle), 'Y': math.sin(angle)}
                    self.send({'Position': new_position, 'Direction': new_direction})
            except Exception as e:
                if self.verbose:
                    print(e)
                return
            time.sleep(0.05)

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

        self.last_position = message['Players'][self.id]['Position']
        self.alive = message['Players'][self.id]['IsAlive']
        if message['Status'] == 1:
            self.game_started = True
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
N = 9
# clients = [TestClient(f"test {i}", verbose=(i==0)) for i in range(N)]
clients = [TestClient(f"test {i}", verbose=False) for i in range(N)]
print(len(clients), 'clients')
for client in clients:
    client.start()
    # time.sleep(0.1)
while True:
    time.sleep(1)
# time.sleep(3)
# for i, client in reversed(list(enumerate(clients))):
#     print('Stopping client', client.id)
#     client.stop()
#     time.sleep(0.001 * i)
