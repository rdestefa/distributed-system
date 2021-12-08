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
