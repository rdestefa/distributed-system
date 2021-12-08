# Simple 'Among Us' Recreation

- React.js Front End
- Go Backend
- Python Testing

# How to run

```
conda env create -f environment.yml
conda activate amongus-distsys-project
```

## Frontend

```
cd frontend/
npm install
npm start
```

### Backend

```
cd backend/
CC="/usr/bin/gcc" go run .
```

### Test Script

```
cd tests
pip install -r requirements.txt
python test.py N
```

N is the number of clients to run.
