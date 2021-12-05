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

NAVMESH = json.load(open('navmesh.json', 'r'))

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
        self.wsapp = websocket.WebSocketApp(f"ws://localhost:10000/connect", header={
                                            'name': self.name}, on_message=self.on_message, on_close=self.on_close)
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
                    giveup = 10
                    if same_direction == 0:
                        angle = 2 * math.pi * (random.randrange(8) / 8.0)
                        # print(angle)
                    move_invalid = True
                    while move_invalid and giveup > 0:
                        same_direction = (same_direction + 1) % 50
                        duration = (datetime.datetime.utcnow() -
                                    self.last_message).total_seconds()
                        r = duration * MOVE_SPEED
                        dirX = math.cos(angle)
                        dirY = math.sin(angle)
                        newX = self.last_position['X'] + r * dirX
                        newY = self.last_position['Y'] + r * dirY
                        move_invalid = (NAVMESH[int(newY)][int(newX)] != 1)
                        if move_invalid:
                            same_direction = 0
                            giveup -= 1
                            angle = 2 * math.pi * (random.randrange(8) / 8.0)

                    if giveup == 0:
                        newX = self.last_position['X']
                        newY = self.last_position['Y']
                        dirX = 0
                        dirY = 0

                    new_position = {
                        'X': newX,
                        'Y': newY,
                    }
                    new_direction = {
                        'X': dirX,
                        'Y': dirY,
                    }
                    self.send({'Position': new_position,
                               'Direction': new_direction})
                    self.last_position = new_position
            except Exception as e:
                print(e)
                return
            time.sleep(0.05)

    def on_message(self, _ws, message):
        message = json.loads(message)
        if type(message) == str:
            self.id = message
            print(self.name, 'id', self.id)
            self.sendt = threading.Thread(target=self.send_updates)
            self.sendt.daemon = True
            self.sendt.start()
            return

        if self.id:
            self.last_position = message['Players'][self.id]['Position']
            self.alive = message['Players'][self.id]['IsAlive']
            if message['Status'] == 1:
                self.game_started = True
            if message['Status'] == 2 or message['Status'] == 3:
                print(self.name, 'game ended')
                self.stop()

        if self.verbose:
            print('message', self.id, message)

    def on_close(self, ws, close_status_code, close_msg):
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
