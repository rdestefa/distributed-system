import React, {Suspense, useEffect, useState} from 'react';
import Game from './Game';

const Login = () => {
  const [username, setUsername] = useState('');
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [servers, setServers] = useState([]);

  useEffect(() => {
    fetch('http://catalog.cse.nd.edu:9097/query.json')
      .then((response) => response.json())
      .then((data) => setServers(data));
  }, []);

  return (
    <div>
      {!isLoggedIn && servers.length !== 0 && (
        <form onSubmit={() => setIsLoggedIn(true)}>
          <label>
            <h1>Enter your name:</h1>
            <input
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </label>
          <input type="submit" value="Log In" />
        </form>
      )}
      {!isLoggedIn && servers.length === 0 && (
        <h1>No Among Us servers available</h1>
      )}
      {isLoggedIn && (
        <Suspense fallback={<h1>Loading...</h1>}>
          <Game
            username={username}
            servers={servers.filter(
              (server: Record<string, any>) => server.project === 'amongus'
            )}
          />
        </Suspense>
      )}
    </div>
  );
};

export default Login;
