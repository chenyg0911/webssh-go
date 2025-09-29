package main

import "time"

type User struct {
	ID               int       `json:"id"`
	Username         string    `json:"username"`
	Password         string    `json:"-"` // Never expose password hash
	IsAdmin          bool      `json:"is_admin"`
	IsApproved       bool      `json:"is_approved"`
	RegistrationDate time.Time `json:"registration_date"`
}

type SSHConnection struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password,omitempty"`
	Key      string `json:"key,omitempty"`
}

type wsMessage struct {
	Type     string `json:"type"`
	Payload  string `json:"payload,omitempty"`
	Filename string `json:"filename,omitempty"`
	Path     string `json:"path,omitempty"`
	Cols     int    `json:"cols,omitempty"`
	Rows     int    `json:"rows,omitempty"`
}

type FileEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
}
