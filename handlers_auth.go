package main

import (
	"fmt"
	"log"
	"net/http"
)

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if noAuth {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if r.Method == http.MethodGet {
		content, _ := staticFiles.ReadFile("static/login.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "" || password == "" {
			http.Redirect(w, r, "/login?error=Please fill in all fields", http.StatusFound)
			return
		}

		user, err := getUserByUsernameDB(username)
		if err != nil {
			log.Printf("Login error for user %s: %v", username, err)
			http.Redirect(w, r, "/login?error=Invalid credentials", http.StatusFound)
			return
		}

		if user == nil {
			http.Redirect(w, r, "/login?error=Invalid credentials", http.StatusFound)
			return
		}

		if !user.IsApproved {
			http.Redirect(w, r, "/login?error=Your account is pending admin approval", http.StatusFound)
			return
		}

		if !verifyPassword(password, user.Password) {
			http.Redirect(w, r, "/login?error=Invalid credentials", http.StatusFound)
			return
		}

		sessionID, err := createSession(username)
		if err != nil {
			log.Printf("Failed to create session for user %s: %v", username, err)
			http.Redirect(w, r, "/login?error=Internal error", http.StatusFound)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			MaxAge:   86400, // 24 hours
		})

		log.Printf("User %s logged in successfully", username)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	// Disable registration in single-user or no-auth mode
	if noAuth || singleUser {
		http.Redirect(w, r, "/login?error=Registration is disabled in single-user mode", http.StatusFound)
		return
	}

	if r.Method == http.MethodGet {
		errorMsg := r.URL.Query().Get("error")
		successMsg := r.URL.Query().Get("success")

		htmlContent := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Register - WebSSH</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Poppins:wght@400;500;600&display=swap" rel="stylesheet">
    <link rel="stylesheet" href="/static/login.css">
    <style>
        .info-box {
            background-color: #d1ecf1;
            color: #0c5460;
            border: 1px solid #bee5eb;
            border-radius: 6px;
            padding: 0.75rem;
            margin-bottom: 1rem;
            font-size: 0.9rem;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="login-card">
            <h1>Register</h1>
            <div class="info-box">
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" style="vertical-align: middle; margin-right: 5px;" viewBox="0 0 16 16">
                    <path d="M8 15A7 7 0 1 1 8 1a7 7 0 0 1 0 14zm0 1A8 8 0 1 0 8 0a8 8 0 0 0 0 16z"/>
                    <path d="M7.002 11a1 1 0 1 1 2 0 1 1 0 0 1-2 0zM7.1 4.995a.905.905 0 1 1 1.8 0l-.35 3.507a.552.552 0 0 1-1.1 0L7.1 4.995z"/>
                </svg>
                Note: After registration, an administrator must approve your account before you can log in.
            </div>
            <form method="POST" action="/register">
                <div class="input-group">
                    <label for="username">Username</label>
                    <input type="text" id="username" name="username" required>
                </div>
                <div class="input-group">
                    <label for="password">Password</label>
                    <input type="password" id="password" name="password" required>
                </div>
                <div class="input-group">
                    <label for="confirm_password">Confirm Password</label>
                    <input type="password" id="confirm_password" name="confirm_password" required>
                </div>
                <button type="submit" class="btn-login">Register</button>
            </form>
            <div class="links">
                <a href="/login">Back to Login</a>
            </div>
` + authGetMessageScript(errorMsg, successMsg) + `
        </div>
    </div>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(htmlContent))
		return
	}

	if r.Method == http.MethodPost {
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if username == "" || password == "" {
			http.Redirect(w, r, "/register?error=Please fill in all fields", http.StatusFound)
			return
		}

		if password != confirmPassword {
			http.Redirect(w, r, "/register?error=Passwords do not match", http.StatusFound)
			return
		}

		if err := createUserDB(username, password, false, false); err != nil {
			http.Redirect(w, r, fmt.Sprintf("/register?error=%s", err.Error()), http.StatusFound)
			return
		}

		log.Printf("New user registered: %s", username)
		http.Redirect(w, r, "/login?success=Registration successful! Your account is now pending admin approval. You will be able to login once an administrator approves your account.", http.StatusFound)
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if noAuth {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	clearSession(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func authGetMessageScript(errorMsg, successMsg string) string {
	if errorMsg != "" {
		return `
            <div id="error-message" class="error-message">` + errorMsg + `</div>
            <script>
                document.getElementById('error-message').style.display = 'block';
            </script>`
	}
	if successMsg != "" {
		return `
            <div id="error-message" class="error-message" style="color: #28a745;">` + successMsg + `</div>
            <script>
                document.getElementById('error-message').style.display = 'block';
            </script>`
	}
	return `
            <div id="error-message" class="error-message" style="display: none;"></div>`
}
