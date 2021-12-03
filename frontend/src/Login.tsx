import React, {Suspense, useState} from 'react';
import Game from './Game';

const Login = () => {
  const [username, setUsername] = useState('');
  const [isLoggedIn, setIsLoggedIn] = useState(false);

  return (
    <div>
      {!isLoggedIn && (
        <form onSubmit={() => setIsLoggedIn(true)}>
          <label>
            Enter username:
            <input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </label>
          <input type="submit" value="Log In" />
        </form>
      )}
      {isLoggedIn && (
        <Suspense fallback={<h1>Loading...</h1>}>
          <Game username={username} />
        </Suspense>
      )}
    </div>
  );
};

export default Login;
