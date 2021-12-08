# %%
import datetime
import json
import math
import os
import random
import sys
import threading
import time
from pprint import pprint
import traceback

import websocket

# websocket.enableTrace(True)
websocket.enableTrace(False)

# %%

NAVMESH = json.load(open('navmesh.json', 'r'))

MOVE_SPEED = 120.0

TEST_1 = False
TEST_2 = False


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
        self.end = False

    def start(self):
        self.wsapp = websocket.WebSocketApp(f"ws://10.26.247.169:10000/connect", header={
                                            'name': self.name}, on_message=self.on_message, on_close=self.on_close)
        self.wst = threading.Thread(target=self.wsapp.run_forever)
        self.wst.daemon = True
        self.wst.start()

        self.id = None
        self.game_started = False
        self.last_message = datetime.datetime.utcnow()
        self.last_position = None
        self.alive = True
        self.last_server_message = None
        self.drift = 0
        self.end = False
        self.out_test = None
        self.last_state = None
        self.last_position_update = datetime.datetime.now().timestamp() * 1000

    def stop(self):
        self.end = True
        self.wsapp.close()
        self.join()

    def join(self):
        if self.wst:
            self.wst.join()
            self.wst = None
        if self.sendt:
            self.sendt.join()
            self.sendt = None

    def send(self, data, now):
        data = data | {'PlayerId': self.id,
                       'Timestamp': now, 'Drift': self.drift}
        msg = DateTimeJSONEncoder().encode(data)
        self.last_message = now
        self.wsapp.send(msg)

    def send_updates(self):
        same_direction = 0
        while not self.end:
            try:
                if not self.game_started or not self.alive or self.last_position == None:
                    self.send({}, datetime.datetime.utcnow())
                else:
                    now = datetime.datetime.utcnow()
                    giveup = 10
                    if same_direction == 0:
                        angle = 2 * math.pi * (random.randrange(8) / 8.0)
                    move_invalid = True
                    while move_invalid and giveup > 0:
                        same_direction = (same_direction + 1) % 50
                        duration = (now - self.last_message).total_seconds()
                        r = duration * MOVE_SPEED
                        dirX = math.cos(angle)
                        dirY = math.sin(angle)
                        newX = self.last_position['X'] + r * dirX
                        newY = self.last_position['Y'] + r * dirY
                        move_invalid = (newX < 0 or newY < 0 or newX >= len(
                            NAVMESH[0]) or newY >= len(NAVMESH) or NAVMESH[int(newY)][int(newX)] != 1)
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
                               'Direction': new_direction}, now)
                    self.last_position = new_position
                    self.last_position_update = datetime.datetime.now().timestamp() * 1000
            except Exception as e:
                traceback.print_exc()
                print(e)
                return
            time.sleep(0.10)

    def on_message(self, _ws, message):
        try:
            message = json.loads(message)
            if type(message) == str:
                self.id = message
                print(self.name, 'id', self.id)
                self.sendt = threading.Thread(target=self.send_updates)
                self.sendt.daemon = True
                self.sendt.start()
                return

            # print(message['Timestamp'])

            if self.id != None:
                self.alive = message['Players'][self.id]['IsAlive']

                serverTimestamp = datetime.datetime.strptime(
                    message['Timestamp'], "%Y-%m-%dT%H:%M:%S.%f%z").timestamp() * 1000
                self.drift = (message['Players'][self.id]['DriftFactor'] + (
                    serverTimestamp - datetime.datetime.now().timestamp() * 1000))/2

                if message['Status'] == 1:
                    self.game_started = True
                    if self.last_position == None:
                        self.last_position = message['Players'][self.id]['Position']
                        self.last_message = datetime.datetime.utcnow()
                if message['Status'] == 2 or message['Status'] == 3:
                    print(self.name, 'game ended')
                    self.stop()
            else:
                return

            if TEST_1:
                now = datetime.datetime.utcnow()
                if self.last_server_message != None:
                    print((now - self.last_server_message).total_seconds(),
                          file=self.out_test)
                    self.out_test.flush()
                    self.last_server_message = now
                else:
                    self.out_test = open('out-'+self.name+'.txt', 'w')
                    self.last_server_message = now

            if TEST_2:
                now = datetime.datetime.now()
                if not self.out_test:
                    self.out_test = open('out-'+self.name+'.txt', 'w')
                    self.prediction_cooldown = 5
                if self.out_test and self.last_state and self.last_position_update:
                    if self.prediction_cooldown > 0:
                        self.prediction_cooldown -= 1
                    else:
                        for player_id, player_before in self.last_state['Players'].items():
                            if player_id == self.id:
                                continue
                            player_now = message['Players'][player_id]

                            last_position = list(
                                player_before['Position'].values())
                            last_direction = list(
                                player_before['Direction'].values())
                            last_update = datetime.datetime.strptime(
                                player_before['LastHeard'], "%Y-%m-%dT%H:%M:%S.%f%z").timestamp() * 1000
                            new_position = list(
                                player_now['Position'].values())
                            new_direction = list(
                                player_now['Direction'].values())
                            new_update = datetime.datetime.strptime(
                                player_now['LastHeard'], "%Y-%m-%dT%H:%M:%S.%f%z").timestamp() * 1000

                            last_other_drift = player_before['Drift']
                            new_other_drift = player_now['Drift']

                            last_duration = (
                                self.last_position_update - (last_update + last_other_drift - self.drift)) / 1000.0
                            new_duration = (
                                self.last_position_update - (new_update + new_other_drift - self.drift)) / 1000.0
                            # last_duration = (now - (last_update)) / 1000.0
                            # new_duration = (now - (new_update)) / 1000.0

                            last_prediction = [
                                last_position[0] + MOVE_SPEED *
                                last_duration * last_direction[0],
                                last_position[1] + MOVE_SPEED *
                                last_duration * last_direction[1]
                            ]
                            new_prediction = [
                                new_position[0] + MOVE_SPEED *
                                new_duration * new_direction[0],
                                new_position[1] + MOVE_SPEED *
                                new_duration * new_direction[1]
                            ]

                            error = math.sqrt(
                                (last_prediction[0] - new_prediction[0])**2 + (last_prediction[1] - new_prediction[1])**2)
                            dirchange = math.sqrt(
                                (last_direction[0] - new_direction[0])**2 + (last_direction[1] - new_direction[1])**2)

                            print(error, dirchange, file=self.out_test)
                            self.out_test.flush()
                self.last_state = message
        except Exception as e:
            traceback.print_exc()
            print(e)

    def on_close(self, ws, close_status_code, close_msg):
        print('close', self.id, close_status_code, close_msg)


# %%
if len(sys.argv) >= 2:
    N = int(sys.argv[1])
else:
    if TEST_1 or TEST_2:
        N = 10
    else:
        N = 9
if len(sys.argv) >= 3:
    start = int(sys.argv[2])
else:
    start = 0
clients = [TestClient(f"test {i}", verbose=False)
           for i in range(start, start+N)]
print(len(clients), 'clients')
for client in clients:
    client.start()

if TEST_1:
    time.sleep(10)
elif TEST_2:
    time.sleep(120)
else:
    while True:
        time.sleep(10)

# for i, client in reversed(list(enumerate(clients))):
#     print('Stopping client', client.id)
#     client.stop()
#     time.sleep(0.001 * i)
