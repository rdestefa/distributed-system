# Simple 'Among Us' Recreation

- React.js Front End
- Go Backend
- Python Testing

# How to run

This project can run from within a preset Conda environment. To access that environment, run the following commands from the project's root directory:

```
conda env create -f environment.yml
conda activate amongus-distsys-project
```

## Frontend

To run the front end React client, execute the following commands from the project's root directory:

```
cd frontend/
npm install
```

On Linux

```
npm start
```

On Windows

```
npm run start-windows
```

You can then access the client from port 9000 on the host machine's machine's IP address.
To change the port number, modify the `package.json` file under `scripts`.

### Backend

To run the Go server, execute the following commands from the project's root directory:

```
cd backend/
CC="/usr/bin/gcc" go run .
```

The server will post its information to the ND CSE name server.

### Test Script

To run the test scripts, execute the following commands from the project's root directory:

```
cd tests
pip install -r requirements.txt
python test.py <n_clients>
```

This test will spawn a set number of clients to connect to the server. The server can, of course, handle 10 user-based clients, and that is what this project was designed for, but this makes it much easier to create a full game quickly for testing purposes. This script, however, does not use the JavaScript client, so it does not query the name server. The exact URL / port for the server will need to be entered into line 44 of the script. Once this is done, run the script with a set number of clients. Then you just need to connect the rest of your user-based clients to the server via the React client and play against the test bots. These are very simple bots that simply run around randomly until hitting a wall or after running in the same direction for a certain period of time, as they purely exist to fill up the lobby so the user can test functionality.

Once connected, test clients are permanently bound to a game, so the user of the script must ensure that _exactly_ 10 clients (including the user) are connected to the server.
