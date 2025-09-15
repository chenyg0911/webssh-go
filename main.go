package main

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

//go:embed all:static
var staticFiles embed.FS

var (
	webUser            string
	webPassword        string
	encryptionKey      []byte
	noAuth             bool
	disableDownload    bool
	disableFileBrowser bool
	sessionStore       = make(map[string]string)
	sessionMutex       sync.RWMutex
	connections        = make(map[string]SSHConnection)
	connectionsMutex   sync.RWMutex
)

const connectionsFile = "connections.json"
const sessionCookieName = "webssh_session"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for now, but you might want to restrict this in production
		return true
	},
}

type SSHConnection struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	Key      string `json:"key,omitempty"`
}

// --- Encryption/Decryption ---

func encrypt(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(encodedData string) ([]byte, error) {
	if encodedData == "" {
		return nil, nil
	}
	data, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

type wsMessage struct {
	Type     string `json:"type"`
	Payload  string `json:"payload,omitempty"`
	Filename string `json:"filename,omitempty"`
	Cols     int    `json:"cols,omitempty"`
	Rows     int    `json:"rows,omitempty"`
	Path     string `json:"path,omitempty"`
}

// --- Session Management ---

func createSession(username string) (string, error) {
	if noAuth {
		return "no-auth-session", nil
	}
	sessionIDBytes := make([]byte, 32)
	if _, err := rand.Read(sessionIDBytes); err != nil {
		return "", err
	}
	sessionID := hex.EncodeToString(sessionIDBytes)

	sessionMutex.Lock()
	sessionStore[sessionID] = username
	sessionMutex.Unlock()

	return sessionID, nil
}

func getSessionUser(r *http.Request) string {
	if noAuth {
		return "no-auth-user"
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}

	sessionMutex.RLock()
	username, ok := sessionStore[cookie.Value]
	sessionMutex.RUnlock()

	if !ok {
		return ""
	}
	return username
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}

	sessionMutex.Lock()
	delete(sessionStore, cookie.Value)
	sessionMutex.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// --- Middleware ---

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if noAuth {
			next.ServeHTTP(w, r)
			return
		}
		if getSessionUser(r) == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- HTTP Handlers ---

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

		if username == webUser && password == webPassword {
			sessionID, err := createSession(username)
			if err != nil {
				http.Error(w, "Failed to create session", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookieName,
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
				MaxAge:   86400, // 24 hours
			})
			http.Redirect(w, r, "/", http.StatusFound)
		} else {
			log.Println("Login failed for user:", username)
			http.Redirect(w, r, "/login?error=1", http.StatusFound)
		}
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

func handleRoot(w http.ResponseWriter, r *http.Request) {
	content, _ := staticFiles.ReadFile("static/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// ... (rest of the functions: load/save connections, websocket handlers, etc.)
// Note: The following functions are unchanged but included for completeness.

func writeWsJSON(ws *websocket.Conn, msgType, payload string) error {
	msg := wsMessage{Type: msgType, Payload: payload}
	marshaledMsg, _ := json.Marshal(msg)
	return ws.WriteMessage(websocket.TextMessage, marshaledMsg)
}

func writeWsError(ws *websocket.Conn, err error) {
	msg := wsMessage{Type: "error", Payload: err.Error()}
	marshaledMsg, _ := json.Marshal(msg)
	// It's fine to ignore the error here, as the connection might be closing.
	_ = ws.WriteMessage(websocket.TextMessage, marshaledMsg)
}

func loadConnections() {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()
	file, err := os.ReadFile(connectionsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Warning: Could not read connections file: %v", err)
		}
		connections = make(map[string]SSHConnection)
		return
	}

	var encryptedConnections map[string]SSHConnection
	if err := json.Unmarshal(file, &encryptedConnections); err != nil {
		log.Printf("Warning: Could not parse connections file: %v. Starting with empty connections.", err)
		connections = make(map[string]SSHConnection)
		return
	}

	connections = make(map[string]SSHConnection)
	for id, conn := range encryptedConnections {
		decryptedPassword, err := decrypt(conn.Password)
		if err != nil {
			log.Printf("Warning: Failed to decrypt password for connection '%s' (%s): %v", conn.Name, id, err)
			continue // Skip this connection
		}
		decryptedKey, err := decrypt(conn.Key)
		if err != nil {
			log.Printf("Warning: Failed to decrypt key for connection '%s' (%s): %v", conn.Name, id, err)
			continue // Skip this connection
		}
		conn.Password = string(decryptedPassword)
		conn.Key = string(decryptedKey)
		connections[id] = conn
	}
	log.Printf("Loaded %d connections.", len(connections))
}

func saveConnections() {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	encryptedConnections := make(map[string]SSHConnection)
	for id, conn := range connections {
		encryptedPassword, _ := encrypt([]byte(conn.Password))
		encryptedKey, _ := encrypt([]byte(conn.Key))
		encryptedConn := conn
		encryptedConn.Password = encryptedPassword
		encryptedConn.Key = encryptedKey
		encryptedConnections[id] = encryptedConn
	}

	data, err := json.MarshalIndent(encryptedConnections, "", "  ")
	if err != nil {
		log.Printf("Error marshaling connections: %v", err)
		return
	}
	os.WriteFile(connectionsFile, data, 0600)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		connectionsMutex.RLock()
		defer connectionsMutex.RUnlock()
		var connList []SSHConnection
		for _, conn := range connections {
			conn.Password = "" // Never send sensitive data to the client
			conn.Key = ""      // Never send sensitive data to the client
			connList = append(connList, conn)
		}
		json.NewEncoder(w).Encode(connList)
	case "POST":
		var conn SSHConnection
		json.NewDecoder(r.Body).Decode(&conn)
		conn.ID = uuid.New().String()
		connectionsMutex.Lock()
		connections[conn.ID] = conn
		connectionsMutex.Unlock()
		saveConnections()
		w.WriteHeader(http.StatusCreated)
	case "DELETE":
		id := r.URL.Query().Get("id")
		connectionsMutex.Lock()
		delete(connections, id)
		connectionsMutex.Unlock()
		saveConnections()
		w.WriteHeader(http.StatusOK)
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if getSessionUser(r) == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	connID := r.URL.Query().Get("id")
	connectionsMutex.RLock()
	sshConn, ok := connections[connID]
	connectionsMutex.RUnlock()
	if !ok {
		http.Error(w, "Connection not found", http.StatusNotFound)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}
	defer ws.Close()

	var auth ssh.AuthMethod
	if sshConn.Key != "" {
		signer, err := ssh.ParsePrivateKey([]byte(sshConn.Key))
		if err != nil {
			writeWsJSON(ws, "status", "Error parsing private key")
			return
		}
		auth = ssh.PublicKeys(signer)
	} else {
		auth = ssh.Password(sshConn.Password)
	}

	config := &ssh.ClientConfig{
		User:            sshConn.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	host := sshConn.Host
	if !strings.Contains(host, ":") {
		host = fmt.Sprintf("%s:22", host)
	}

	sshClient, err := ssh.Dial("tcp", host, config)
	if err != nil {
		writeWsJSON(ws, "status", fmt.Sprintf("SSH connection failed: %v", err))
		return
	}
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		writeWsJSON(ws, "status", "Failed to create SSH session")
		return
	}
	defer session.Close()

	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	session.RequestPty("xterm", 80, 40, modes)

	stdout, _ := session.StdoutPipe()
	stdin, _ := session.StdinPipe()
	session.Shell()

	go func() {
		defer ws.Close()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				return
			}
			writeWsJSON(ws, "stdout", string(buf[:n]))
		}
	}()

	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			return
		}
		var msg wsMessage
		json.Unmarshal(p, &msg)
		switch msg.Type {
		case "data":
			stdin.Write([]byte(msg.Payload))
		case "resize":
			session.WindowChange(msg.Rows, msg.Cols)
		case "upload":
			if !disableFileBrowser {
				go handleFileUpload(sshClient, ws, msg.Path, msg.Filename, msg.Payload)
			}
		case "list":
			if !disableFileBrowser {
				go handleFileListing(sshClient, ws, msg.Path)
			}
		case "download":
			if !disableFileBrowser && !disableDownload {
				go handleFileDownload(sshClient, ws, msg.Path)
			}
		}
	}
}

func handleFileListing(sshClient *ssh.Client, ws *websocket.Conn, path string) {
	if disableFileBrowser {
		writeWsError(ws, fmt.Errorf("file browser is disabled by the server administrator"))
		return
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		writeWsError(ws, fmt.Errorf("could not start SFTP session: %w", err))
		return
	}
	defer sftpClient.Close()

	requestedPath := path
	if requestedPath == "" {
		requestedPath, err = sftpClient.Getwd()
		if err != nil {
			writeWsError(ws, fmt.Errorf("could not get working directory: %w", err))
			return
		}
	}

	files, err := sftpClient.ReadDir(requestedPath)
	if err != nil {
		writeWsError(ws, fmt.Errorf("could not read directory '%s': %w", requestedPath, err))
		return
	}

	type fileInfo struct {
		Name  string    `json:"name"`
		Size  int64     `json:"size"`
		IsDir bool      `json:"isDir"`
		Mod   time.Time `json:"mod"`
	}

	var fileList []fileInfo
	for _, f := range files {
		fileList = append(fileList, fileInfo{
			Name:  f.Name(),
			Size:  f.Size(),
			IsDir: f.IsDir(),
			Mod:   f.ModTime(),
		})
	}

	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"path":  requestedPath, // CRITICAL FIX: Return the path that was requested by the client.
		"files": fileList,
	})

	writeWsJSON(ws, "list", string(payloadBytes))
}

func handleFileDownload(sshClient *ssh.Client, ws *websocket.Conn, path string) {
	if disableFileBrowser || disableDownload {
		writeWsError(ws, fmt.Errorf("file download is disabled by the server administrator"))
		return
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		writeWsError(ws, fmt.Errorf("could not start SFTP session: %w", err))
		return
	}
	defer sftpClient.Close()

	srcFile, err := sftpClient.Open(path)
	if err != nil {
		writeWsError(ws, fmt.Errorf("could not open remote file '%s': %w", path, err))
		return
	}
	defer srcFile.Close()

	// Check if it's a directory
	stat, err := srcFile.Stat()
	if err != nil {
		writeWsError(ws, fmt.Errorf("could not stat remote file '%s': %w", path, err))
		return
	}
	if stat.IsDir() {
		go zipDirectory(sshClient, ws, path)
	} else {
		content, err := io.ReadAll(srcFile)
		if err != nil {
			writeWsError(ws, fmt.Errorf("could not read remote file '%s': %w", path, err))
			return
		}
		base64Content := base64.StdEncoding.EncodeToString(content)
		filename := filepath.Base(path)
		writeWsJSON(ws, "download", fmt.Sprintf(`{"filename": "%s", "payload": "%s"}`, filename, base64Content))
	}
}

func handleFileUpload(sshClient *ssh.Client, ws *websocket.Conn, path, filename, base64Content string) {
	if disableFileBrowser {
		writeWsError(ws, fmt.Errorf("file browser is disabled by the server administrator"))
		return
	}

	sendStatus := func(message string) {
		writeWsJSON(ws, "status", fmt.Sprintf("\r\n%s\r\n", message))
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sendStatus(fmt.Sprintf("Error: Could not start SFTP session: %v", err))
		return
	}
	defer sftpClient.Close()
	fileContent, _ := base64.StdEncoding.DecodeString(base64Content)

	remotePath := filename
	if path != "" {
		remotePath = sftpClient.Join(path, filename)
	}

	dstFile, err := sftpClient.Create(remotePath)
	if err != nil {
		sendStatus(fmt.Sprintf("Error: Could not create remote file %s: %v", filename, err))
		return
	}
	defer dstFile.Close()
	dstFile.Write(fileContent)
	sendStatus(fmt.Sprintf("File '%s' uploaded successfully.", filename))
}

func zipDirectory(sshClient *ssh.Client, ws *websocket.Conn, remotePath string) {
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		writeWsError(ws, fmt.Errorf("could not start SFTP session for zipping: %w", err))
		return
	}
	defer sftpClient.Close()

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	walker := sftpClient.Walk(remotePath)
	for walker.Step() {
		if walker.Err() != nil {
			writeWsError(ws, fmt.Errorf("failed during directory walk: %w", walker.Err()))
			return
		}

		// Get the relative path for the zip file structure
		relPath, err := filepath.Rel(remotePath, walker.Path())
		if err != nil {
			writeWsError(ws, fmt.Errorf("could not get relative path for '%s': %w", walker.Path(), err))
			return
		}
		// For the root directory itself
		if relPath == "." {
			continue
		}

		info := walker.Stat()
		if info.IsDir() {
			// Create a directory entry in the zip file
			_, err := zipWriter.Create(relPath + "/")
			if err != nil {
				writeWsError(ws, fmt.Errorf("could not create directory in zip: %w", err))
				return
			}
		} else {
			// Create a file entry in the zip file
			fileWriter, err := zipWriter.Create(relPath)
			if err != nil {
				writeWsError(ws, fmt.Errorf("could not create file in zip: %w", err))
				return
			}
			// Open the remote file and copy its content
			srcFile, err := sftpClient.Open(walker.Path())
			if err == nil {
				io.Copy(fileWriter, srcFile)
				srcFile.Close()
			}
		}
	}

	zipWriter.Close()

	base64Content := base64.StdEncoding.EncodeToString(buf.Bytes())
	zipFilename := filepath.Base(remotePath) + ".zip"
	writeWsJSON(ws, "download", fmt.Sprintf(`{"filename": "%s", "payload": "%s"}`, zipFilename, base64Content))
}

func handleFeatures(w http.ResponseWriter, r *http.Request) {
	features := map[string]bool{
		"download":    !disableDownload && !disableFileBrowser,
		"fileBrowser": !disableFileBrowser,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(features)
}

func main() {
	var (
		enableTLS bool
		certFile  string
		keyFile   string
	)

	flag.BoolVar(&noAuth, "no-auth", false, "Disable authentication")
	flag.BoolVar(&enableTLS, "tls", false, "Enable TLS for HTTPS and WSS")
	flag.StringVar(&certFile, "cert-file", "", "Path to TLS certificate file")
	flag.StringVar(&keyFile, "key-file", "", "Path to TLS key file")
	flag.BoolVar(&disableDownload, "disable-download", false, "Disable only the file download functionality")
	flag.BoolVar(&disableFileBrowser, "disable-file-browser", false, "Disable the entire file browser (upload, download, list)")
	flag.Parse()

	if !noAuth {
		webUser = os.Getenv("WEBSSH_USER")
		webPassword = os.Getenv("WEBSSH_PASSWORD")
		if webUser == "" || webPassword == "" {
			log.Fatal("FATAL: Environment variables WEBSSH_USER and WEBSSH_PASSWORD must be set when authentication is enabled.")
		}
	} else {
		log.Println("Authentication is disabled.")
	}

	if enableTLS && (certFile == "" || keyFile == "") {
		log.Fatal("FATAL: --cert-file and --key-file must be provided when --tls is enabled.")
	}

	keyHex := os.Getenv("WEBSSH_ENCRYPTION_KEY")
	if keyHex == "" {
		log.Fatal("FATAL: Environment variable WEBSSH_ENCRYPTION_KEY must be set to a 64-character hex string (32 bytes).")
	}
	var err error
	encryptionKey, err = hex.DecodeString(keyHex)
	if err != nil || len(encryptionKey) != 32 {
		log.Fatal("FATAL: WEBSSH_ENCRYPTION_KEY must be a valid 64-character hex string (32 bytes).")
	}

	loadConnections()

	staticFS, _ := fs.Sub(staticFiles, "static")
	staticServer := http.FileServer(http.FS(staticFS))

	http.HandleFunc("/login", handleLogin)
	http.Handle("/static/", http.StripPrefix("/static/", staticServer))
	http.Handle("/", authMiddleware(http.HandlerFunc(handleRoot)))
	http.Handle("/logout", authMiddleware(http.HandlerFunc(handleLogout)))
	http.Handle("/api/connections", authMiddleware(http.HandlerFunc(handleConnections)))
	http.Handle("/ws", authMiddleware(http.HandlerFunc(handleWebSocket)))
	http.Handle("/api/features", authMiddleware(http.HandlerFunc(handleFeatures)))

	if enableTLS {
		log.Println("Server started with TLS on :8443")
		if err := http.ListenAndServeTLS(":8443", certFile, keyFile, nil); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("Server started on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err)
		}
	}
}
