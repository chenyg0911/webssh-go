package main

import (
	"database/sql"
	"embed"
	"encoding/hex"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
)

//go:embed all:static
var staticFiles embed.FS

var (
	encryptionKey      []byte
	noAuth             bool
	singleUser         bool
	disableDownload    bool
	disableFileBrowser bool
	sessionStore       = make(map[string]string)
	sessionMutex       sync.RWMutex

	users      = make(map[string]User)
	usersMutex sync.RWMutex
	db         *sql.DB
)

const (
	databaseFile      = "webssh.db"
	sessionCookieName = "webssh_session"
)

func main() {
	var (
		enableTLS bool
		certFile  string
		keyFile   string
	)

	flag.BoolVar(&noAuth, "no-auth", false, "Disable authentication")
	flag.BoolVar(&singleUser, "single-user", false, "Enable single-user mode with a 'default' user")
	flag.BoolVar(&enableTLS, "tls", false, "Enable TLS for HTTPS and WSS")
	flag.StringVar(&certFile, "cert-file", "", "Path to TLS certificate file")
	flag.StringVar(&keyFile, "key-file", "", "Path to TLS key file")
	flag.BoolVar(&disableDownload, "disable-download", false, "Disable only the file download functionality")
	flag.BoolVar(&disableFileBrowser, "disable-file-browser", false, "Disable the entire file browser (upload, download, list)")
	flag.Parse()

	log.Printf("Starting WebSSH application...")

	log.Printf("Flags parsed, noAuth: %v, singleUser: %v", noAuth, singleUser)

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

	log.Printf("Initializing database...")
	if err := initDatabase(); err != nil {
		log.Fatal("FATAL: Failed to initialize database:", err)
	}
	log.Printf("Database initialized successfully")

	if noAuth {
		log.Println("Authentication disabled (no-auth mode). All connections are shared.")
		ensureDefaultUserExists("") // No password needed
	} else if singleUser {
		log.Println("Single-user mode enabled (user: default).")
		defaultPassword := os.Getenv("WEBSSH_PASSWORD")
		if defaultPassword == "" {
			defaultPassword = "default123"
		}
		ensureDefaultUserExists(defaultPassword)
	} else {
		log.Println("Multi-user authentication enabled.")
		adminPassword := os.Getenv("WEBSSH_ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "admin123"
		}
		ensureAdminUserExists(adminPassword)
	}

	loadUsersIntoMemory()

	staticFS, _ := fs.Sub(staticFiles, "static")
	staticServer := http.FileServer(http.FS(staticFS))

	// Authentication routes
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/register", handleRegister)
	http.Handle("/logout", authMiddleware(http.HandlerFunc(handleLogout)))

	// Admin routes
	http.Handle("/admin", authMiddleware(http.HandlerFunc(handleAdmin)))
	http.Handle("/admin/page", authMiddleware(http.HandlerFunc(handleAdminPage)))
	http.Handle("/api/admin/approve", authMiddleware(http.HandlerFunc(handleAdminApprove)))
	http.Handle("/api/admin/users/", authMiddleware(http.HandlerFunc(handleAdminUsers)))

	// Core application routes
	http.Handle("/static/", http.StripPrefix("/static/", staticServer))
	http.Handle("/", authMiddleware(http.HandlerFunc(handleRoot)))
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
