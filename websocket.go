package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type sftpClientManager struct {
	sftpClient *sftp.Client
	sshClient  *ssh.Client
	mutex      sync.Mutex
}

func (m *sftpClientManager) Get(sshClient *ssh.Client) (*sftp.Client, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.sftpClient != nil && m.sshClient == sshClient {
		// Check if the client is still alive
		if _, err := m.sftpClient.Getwd(); err == nil {
			return m.sftpClient, nil
		}
	}

	// Close existing client if it's old or dead
	if m.sftpClient != nil {
		m.sftpClient.Close()
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}

	m.sftpClient = sftpClient
	m.sshClient = sshClient
	return m.sftpClient, nil
}

func (m *sftpClientManager) Close() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.sftpClient != nil {
		m.sftpClient.Close()
		m.sftpClient = nil
		m.sshClient = nil
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	connID := r.URL.Query().Get("id")
	if connID == "" {
		conn.WriteMessage(websocket.TextMessage, []byte("Connection ID is required"))
		return
	}

	username := getSessionUser(r)
	user, err := getUserByUsernameDB(username)
	if err != nil || user == nil {
		conn.WriteMessage(websocket.TextMessage, []byte("User not found"))
		return
	}

	sshConnDetails, err := getConnectionByIDDB(user.ID, connID)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to get connection details"))
		return
	}

	encryptedPasswordBytes, _ := hex.DecodeString(sshConnDetails.Password)
	decryptedPassword, err := decrypt(encryptedPasswordBytes)
	if err != nil {
		log.Printf("Failed to decrypt password for conn %s: %v", connID, err)
	}

	encryptedKeyBytes, _ := hex.DecodeString(sshConnDetails.Key)
	decryptedKey, _ := decrypt(encryptedKeyBytes)

	var authMethods []ssh.AuthMethod
	if len(decryptedPassword) > 0 {
		authMethods = append(authMethods, ssh.Password(string(decryptedPassword)))
	}
	if len(decryptedKey) > 0 {
		signer, err := ssh.ParsePrivateKey(decryptedKey)
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		} else {
			log.Printf("Failed to parse private key: %v", err)
		}
	}

	config := &ssh.ClientConfig{
		User:            sshConnDetails.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	host := sshConnDetails.Host
	// Check if port is specified, if not, use default port 22
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "22")
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to dial: %s", err)))
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to create session: %s", err)))
		return
	}
	defer session.Close()

	sftpMgr := &sftpClientManager{}
	defer sftpMgr.Close()

	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm-256color", 80, 40, modes); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("request for pseudo terminal failed: %s", err)))
		return
	}

	if err := session.Shell(); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("failed to start shell: %s", err)))
		return
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				return
			}
			msg, _ := json.Marshal(wsMessage{Type: "stdout", Payload: string(buf[:n])})
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			msg, _ := json.Marshal(wsMessage{Type: "stdout", Payload: string(buf[:n])})
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(p, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "data":
			stdin.Write([]byte(msg.Payload))
		case "resize":
			var size struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			}
			if err := json.Unmarshal([]byte(msg.Payload), &size); err == nil {
				session.WindowChange(size.Rows, size.Cols)
			}
		case "list":
			if disableFileBrowser {
				continue
			}
			go handleFileListing(conn, client, sftpMgr, msg.Path)
		case "upload":
			if disableFileBrowser {
				continue
			}
			go handleFileUpload(conn, client, sftpMgr, msg.Filename, msg.Payload, msg.Path)
		case "download":
			if disableFileBrowser || disableDownload {
				continue
			}
			go handleFileDownload(conn, client, sftpMgr, msg.Path)
		}
	}
}

func handleFileListing(ws *websocket.Conn, sshClient *ssh.Client, sftpMgr *sftpClientManager, path string) {
	sftpClient, err := sftpMgr.Get(sshClient)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to get SFTP client: %v", err))
		return
	}

	if path == "" {
		path, _ = sftpClient.Getwd()
	}

	files, err := sftpClient.ReadDir(path)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to list files in %s: %v", path, err))
		return
	}

	var fileEntries []FileEntry
	for _, f := range files {
		fileEntries = append(fileEntries, FileEntry{
			Name:  f.Name(),
			Size:  f.Size(),
			IsDir: f.IsDir(),
		})
	}

	payload, _ := json.Marshal(map[string]interface{}{"path": path, "files": fileEntries})
	msg, _ := json.Marshal(wsMessage{Type: "list", Payload: string(payload)})
	ws.WriteMessage(websocket.TextMessage, msg)
}

func handleFileUpload(ws *websocket.Conn, sshClient *ssh.Client, sftpMgr *sftpClientManager, filename, base64Content, remotePath string) {
	sftpClient, err := sftpMgr.Get(sshClient)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to get SFTP client: %v", err))
		return
	}

	content, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to decode file content: %v", err))
		return
	}

	if remotePath == "" {
		remotePath, _ = sftpClient.Getwd()
	}
	dstPath := filepath.Join(remotePath, filename)

	f, err := sftpClient.Create(dstPath)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to create remote file: %v", err))
		return
	}
	defer f.Close()

	if _, err := f.Write(content); err != nil {
		sendError(ws, fmt.Sprintf("Failed to write to remote file: %v", err))
		return
	}

	sendStatus(ws, fmt.Sprintf("\r\n\x1b[32mFile '%s' uploaded successfully to %s.\x1b[0m\r\n", filename, remotePath))
}

func handleFileDownload(ws *websocket.Conn, sshClient *ssh.Client, sftpMgr *sftpClientManager, path string) {
	sftpClient, err := sftpMgr.Get(sshClient)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to get SFTP client: %v", err))
		return
	}

	stat, err := sftpClient.Stat(path)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to stat path '%s': %v", path, err))
		return
	}

	if stat.IsDir() {
		handleDirectoryDownload(ws, sftpClient, path)
		return
	}

	srcFile, err := sftpClient.Open(path)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to open remote file: %v", err))
		return
	}
	defer srcFile.Close()

	content, err := io.ReadAll(srcFile)
	if err != nil {
		sendError(ws, fmt.Sprintf("Failed to read remote file: %v", err))
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"filename": filepath.Base(path),
		"payload":  base64.StdEncoding.EncodeToString(content),
	})
	msg, _ := json.Marshal(wsMessage{Type: "download", Payload: string(payload)})
	ws.WriteMessage(websocket.TextMessage, msg)
}

func handleDirectoryDownload(ws *websocket.Conn, sftpClient *sftp.Client, dirPath string) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	walker := sftpClient.Walk(dirPath)
	for walker.Step() {
		if walker.Err() != nil {
			continue
		}
		path := walker.Path()
		if path == dirPath {
			continue
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			continue
		}

		info := walker.Stat()
		if info.IsDir() {
			zipWriter.Create(relPath + "/")
		} else {
			file, err := sftpClient.Open(path)
			if err != nil {
				continue
			}
			f, err := zipWriter.Create(relPath)
			if err != nil {
				file.Close()
				continue
			}
			io.Copy(f, file)
			file.Close()
		}
	}

	zipWriter.Close()

	payload, _ := json.Marshal(map[string]string{
		"filename": filepath.Base(dirPath) + ".zip",
		"payload":  base64.StdEncoding.EncodeToString(buf.Bytes()),
	})
	msg, _ := json.Marshal(wsMessage{Type: "download", Payload: string(payload)})
	ws.WriteMessage(websocket.TextMessage, msg)
}

func sendError(ws *websocket.Conn, message string) {
	log.Println("SFTP/WS Error:", message)
	msg, _ := json.Marshal(wsMessage{Type: "error", Payload: message})
	ws.WriteMessage(websocket.TextMessage, msg)
}

func sendStatus(ws *websocket.Conn, message string) {
	msg, _ := json.Marshal(wsMessage{Type: "status", Payload: message})
	ws.WriteMessage(websocket.TextMessage, msg)
}
