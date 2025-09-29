package main

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
)

func handleRoot(w http.ResponseWriter, r *http.Request) {
	content, _ := staticFiles.ReadFile("static/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	username := getSessionUser(r)
	if username == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	user, err := getUserByUsernameDB(username)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		connections, err := getUserConnectionsDB(user.ID)
		if err != nil {
			http.Error(w, "Failed to get connections", http.StatusInternalServerError)
			return
		}

		// Remove sensitive information before sending to client
		for i := range connections {
			connections[i].Password = ""
			connections[i].Key = ""
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(connections)

	case http.MethodPost:
		var conn SSHConnection
		if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Encrypt sensitive information
		encryptedPassword, err := encrypt([]byte(conn.Password))
		if err != nil {
			http.Error(w, "Failed to encrypt password", http.StatusInternalServerError)
			return
		}
		encryptedKey, err := encrypt([]byte(conn.Key))
		if err != nil {
			http.Error(w, "Failed to encrypt key", http.StatusInternalServerError)
			return
		}

		if err := createConnectionDB(user.ID, conn.Name, conn.Host, conn.User, hex.EncodeToString(encryptedPassword), hex.EncodeToString(encryptedKey)); err != nil {
			http.Error(w, "Failed to create connection", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		connID := r.URL.Query().Get("id")
		if connID == "" {
			http.Error(w, "Connection ID required", http.StatusBadRequest)
			return
		}

		if err := deleteConnectionDB(user.ID, connID); err != nil {
			http.Error(w, "Failed to delete connection", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleFeatures(w http.ResponseWriter, r *http.Request) {
	features := map[string]bool{
		"download":    !disableDownload && !disableFileBrowser,
		"fileBrowser": !disableFileBrowser,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(features)
}


