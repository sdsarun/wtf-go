package sample

import (
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	html := `
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8" />
  <title>Login</title>

  <!-- React CDN -->
  <script src="https://unpkg.com/react@18/umd/react.development.js"></script>
  <script src="https://unpkg.com/react-dom@18/umd/react-dom.development.js"></script>

  <!-- Babel (to use JSX in browser) -->
  <script src="https://unpkg.com/@babel/standalone/babel.min.js"></script>

  <style>
    body {
      font-family: Arial;
      background: #f4f4f4;
      display: flex;
      justify-content: center;
      align-items: center;
      height: 100vh;
    }
    .box {
      background: white;
      padding: 20px;
      border-radius: 8px;
      width: 300px;
    }
    input {
      width: 100%;
      margin-bottom: 10px;
      padding: 8px;
    }
    button {
      width: 100%;
      padding: 10px;
      background: blue;
      color: white;
      border: none;
    }
  </style>
</head>

<body>
  <div id="root"></div>

  <script type="text/babel">
    function App() {
      const [email, setEmail] = React.useState("");
      const [password, setPassword] = React.useState("");

      const handleLogin = () => {
        alert("Email: " + email + ", Password: " + password);
      };

      return (
        <div className="box">
          <h2>Login</h2>
          <input
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <input
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <button onClick={handleLogin}>Login</button>
        </div>
      );
    }

    ReactDOM.createRoot(document.getElementById("root")).render(<App />);
  </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func SimpleBE() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	log.Println("Server running on :4000")
	log.Fatal(http.ListenAndServe(":4000", mux))
}
