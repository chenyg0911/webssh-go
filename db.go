package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func initDatabase() error {
	dbPath := filepath.Join("data", databaseFile)
	if err := os.MkdirAll("data", 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	createUsersTable := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT NOT NULL UNIQUE,
        password TEXT NOT NULL,
        is_admin BOOLEAN NOT NULL DEFAULT 0,
        is_approved BOOLEAN NOT NULL DEFAULT 0,
        registration_date DATETIME DEFAULT CURRENT_TIMESTAMP
    );`
	if _, err := db.Exec(createUsersTable); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	createConnectionsTable := `
    CREATE TABLE IF NOT EXISTS connections (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER NOT NULL,
        name TEXT NOT NULL,
        host TEXT NOT NULL,
        user TEXT NOT NULL,
        password TEXT,
        key TEXT,
        FOREIGN KEY(user_id) REFERENCES users(id)
    );`
	if _, err := db.Exec(createConnectionsTable); err != nil {
		return fmt.Errorf("failed to create connections table: %w", err)
	}

	return nil
}

func createUserDB(username, password string, isAdmin, isApproved bool) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	_, err = db.Exec("INSERT INTO users (username, password, is_admin, is_approved) VALUES (?, ?, ?, ?)", username, hashedPassword, isAdmin, isApproved)
	if err != nil {
		return fmt.Errorf("username already exists")
	}
	return nil
}

func getUserByUsernameDB(username string) (*User, error) {
	var user User
	err := db.QueryRow("SELECT id, username, password, is_admin, is_approved, registration_date FROM users WHERE username = ?", username).Scan(&user.ID, &user.Username, &user.Password, &user.IsAdmin, &user.IsApproved, &user.RegistrationDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func getAllUsersDB() ([]User, error) {
	rows, err := db.Query("SELECT id, username, password, is_admin, is_approved, registration_date FROM users ORDER BY registration_date DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allUsers []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Password, &user.IsAdmin, &user.IsApproved, &user.RegistrationDate); err != nil {
			return nil, err
		}
		allUsers = append(allUsers, user)
	}
	return allUsers, nil
}

func getPendingUsersDB() ([]User, error) {
	rows, err := db.Query("SELECT id, username, password, is_admin, is_approved, registration_date FROM users WHERE is_approved = 0 ORDER BY registration_date ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pendingUsers []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Password, &user.IsAdmin, &user.IsApproved, &user.RegistrationDate); err != nil {
			return nil, err
		}
		pendingUsers = append(pendingUsers, user)
	}
	return pendingUsers, nil
}

func updateUserApprovalDB(username string, isApproved bool) error {
	_, err := db.Exec("UPDATE users SET is_approved = ? WHERE username = ?", isApproved, username)
	return err
}

func updateUserAdminStatusDB(username string, isAdmin bool) error {
	_, err := db.Exec("UPDATE users SET is_admin = ? WHERE username = ?", isAdmin, username)
	return err
}

func updateUserPasswordDB(username, password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	res, _ := db.Exec("UPDATE users SET password = ? WHERE username = ?", hashedPassword, username)
	_, err = res.RowsAffected() // Check if update was successful
	return err
}

func deleteUserDB(username string) error {
	_, err := db.Exec("DELETE FROM users WHERE username = ?", username)
	return err
}

func createConnectionDB(userID int, name, host, user, password, key string) error {
	_, err := db.Exec("INSERT INTO connections (user_id, name, host, user, password, key) VALUES (?, ?, ?, ?, ?, ?)", userID, name, host, user, password, key)
	return err
}

func getUserConnectionsDB(userID int) ([]SSHConnection, error) {
	rows, err := db.Query("SELECT id, name, host, user FROM connections WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connections []SSHConnection
	for rows.Next() {
		var conn SSHConnection
		if err := rows.Scan(&conn.ID, &conn.Name, &conn.Host, &conn.User); err != nil {
			return nil, err
		}
		connections = append(connections, conn)
	}
	return connections, nil
}

func getConnectionByIDDB(userID int, connID string) (*SSHConnection, error) {
	var conn SSHConnection
	err := db.QueryRow("SELECT id, name, host, user, password, key FROM connections WHERE id = ? AND user_id = ?", connID, userID).Scan(&conn.ID, &conn.Name, &conn.Host, &conn.User, &conn.Password, &conn.Key)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func deleteConnectionDB(userID int, connID string) error {
	res, err := db.Exec("DELETE FROM connections WHERE id = ? AND user_id = ?", connID, userID)
	if err != nil {
		return err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("connection not found or not owned by user")
	}
	return nil
}

func ensureAdminUserExists(adminPassword string) {
	const adminUser = "admin"
	user, err := getUserByUsernameDB(adminUser)
	if err != nil {
		log.Fatalf("Failed to check for admin user: %v", err)
	}
	if user == nil {
		log.Printf("Admin user 'admin' not found, creating...")
		if err := createUserDB(adminUser, adminPassword, true, true); err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		log.Printf("Admin user 'admin' created with the provided or default password.")
	} else {
		// If user exists but doesn't have admin privileges, grant them
		if !user.IsAdmin {
			log.Printf("Existing user 'admin' doesn't have admin privileges, updating...")
			if err := updateUserAdminStatusDB(adminUser, true); err != nil {
				log.Fatalf("Failed to update admin user privileges: %v", err)
			}
			log.Printf("User 'admin' privileges updated successfully.")
		}

		// Also update password if WEBSSH_ADMIN_PASSWORD is set and not the default
		// This allows resetting the admin password via environment variable.
		if adminPassword != "admin123" {
			log.Printf("Updating password for admin user 'admin' from environment variable...")
			if err := updateUserPasswordDB(adminUser, adminPassword); err != nil {
				log.Fatalf("Failed to update admin user password: %v", err)
			}
		}
	}
}

func ensureDefaultUserExists(defaultPassword string) {
	const defaultUser = "default"
	user, err := getUserByUsernameDB(defaultUser)
	if err != nil {
		log.Fatalf("Failed to check for default user: %v", err)
	}
	if user == nil {
		log.Println("Default user not found, creating...")
		// Create the 'default' user as non-admin but approved.
		// If defaultPassword is "", a dummy hash will be created, which is fine for no-auth mode.
		if err := createUserDB(defaultUser, defaultPassword, false, true); err != nil {
			log.Fatalf("Failed to create default user: %v", err)
		}
	} else {
		// If a password is provided (i.e., not in no-auth mode) and it's not the default, update it.
		if defaultPassword != "" && defaultPassword != "default123" {
			log.Printf("Updating password for default user from environment variable...")
			if err := updateUserPasswordDB(defaultUser, defaultPassword); err != nil {
				log.Fatalf("Failed to update default user password: %v", err)
			}
			log.Printf("Default user password updated successfully.")
		}
	}
}

func loadUsersIntoMemory() {
	allUsers, err := getAllUsersDB()
	if err != nil {
		log.Fatalf("Failed to load users into memory: %v", err)
	}
	usersMutex.Lock()
	defer usersMutex.Unlock()
	users = make(map[string]User)
	for _, u := range allUsers {
		users[u.Username] = u
	}
	log.Printf("Loaded %d users into memory cache.", len(users))
}
