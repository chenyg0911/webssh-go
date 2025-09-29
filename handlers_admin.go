package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	// "time" // No longer needed here
)

// handleAdmin is a lightweight handler for checking admin permissions.
func handleAdmin(w http.ResponseWriter, r *http.Request) {
	currentUser, err := getUserByUsernameDB(getSessionUser(r))
	if err != nil || !currentUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	// If the check passes, just return OK.
	// The frontend uses a HEAD request to this endpoint for the icon display.
	// We must return early to prevent any subsequent middleware from altering the response.
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
}

// handleAdminPage serves the full HTML content for the admin panel.
func handleAdminPage(w http.ResponseWriter, r *http.Request) {
	currentUser, err := getUserByUsernameDB(getSessionUser(r))
	if err != nil || !currentUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	pendingUsers, err := getPendingUsersDB()
	if err != nil {
		http.Error(w, "Failed to load pending users", http.StatusInternalServerError)
		return
	}

	allUsers, err := getAllUsersDB()
	if err != nil {
		http.Error(w, "Failed to load all users", http.StatusInternalServerError)
		return
	}

	const tmpl = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Panel - WebSSH</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #f8f9fa;
            color: #343a40;
            margin: 0;
            line-height: 1.6;
        }
        
        .admin-wrapper {
            /* min-height: 100vh; */ /* Removed for better modal integration */
            display: flex;
            flex-direction: column;
        }
        
        .admin-header {
            background-color: #ffffff;
            padding: 1rem 2rem;
            border-bottom: 1px solid #dee2e6;
            box-shadow: 0 2px 4px rgba(0,0,0,.04);
        }
        
        .admin-header-content {
            max-width: 1200px;
            margin: 0 auto;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        
        .admin-header h1 {
            margin: 0;
            font-size: 1.5rem;
            font-weight: 600;
        }
        
        .admin-nav a {
            color: #007bff;
            text-decoration: none;
            font-weight: 500;
        }
        
        .admin-main {
            flex-grow: 1;
            padding: 2rem;
            max-width: 1200px;
            margin: 0 auto;
            width: 100%;
            box-sizing: border-box;
        }
        
        .tab-container {
            background-color: #ffffff;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(0,0,0,.08);
            overflow: hidden;
        }
        
        .tab-nav {
            display: flex;
            border-bottom: 1px solid #dee2e6; /* #dee2e6 */
        }
        
        .tab-button {
            padding: 1rem 1.5rem;
            border: none;
            background: none;
            cursor: pointer;
            font-size: 1rem;
            font-weight: 500;
            color: #6c757d;
            border-bottom: 3px solid transparent; /* transparent */
            transition: color 0.2s, border-color 0.2s;
        }
        
        .tab-button.active {
            color: #007bff; /* #007bff */
            border-bottom-color: #007bff;
        }
        
        .tab-content {
            padding: 2rem;
        }
        
        .user-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        
        .user-table th, .user-table td {
            padding: 0.75rem 1rem;
            text-align: left;
            border-bottom: 1px solid #dee2e6;
        }
        
        .user-table th {
            background-color: #f8f9fa;
            font-weight: 600;
        }
        
        .user-table td .btn {
            margin-right: 0.5rem;
        }
        
        .form-section {
            margin-top: 2rem;
            padding-top: 2rem;
            border-top: 1px solid #dee2e6;
        }
        
        .form-section h3 {
            margin-top: 0;
            font-weight: 600;
        }
        
        .form-group {
            margin-bottom: 1rem;
        }
        
        .form-row {
            display: flex;
            align-items: center;
            margin-bottom: 1rem;
        }
        
        .form-label {
            flex: 0 0 120px;
            text-align: right;
            padding-right: 10px;
            font-weight: 500;
        }
        
        .form-input {
            flex: 1;
            max-width: 400px;
        }
        
        .form-group input[type="text"], .form-group input[type="password"] {
            width: 100%;
            padding: 0.75rem;
            border-radius: 6px;
            border: 1px solid #ced4da;
            box-sizing: border-box;
            transition: border-color 0.2s, box-shadow 0.2s;
        }
        
        .form-group input:focus {
            outline: none;
            border-color: #80bdff;
            box-shadow: 0 0 0 0.2rem rgba(0,123,255,.25);
        }
        
        .checkbox-group {
            display: flex;
            flex-direction: column;
        }
        
        .checkbox-option {
            margin-bottom: 0.5rem;
            display: flex;
            align-items: center;
        }
        
        .checkbox-group input[type="checkbox"] {
            width: 18px;
            height: 18px;
            accent-color: #007bff;
            margin-right: 0.5rem;
        }

        .switch-label {
            font-size: 0.9rem;
            color: #495057;
        }
        
        .checkbox-group label {
            margin-bottom: 0;
            cursor: pointer;
            font-size: 0.9rem;
        }

        /* Toggle Switch Styles */
        .switch {
            position: relative;
            display: inline-block;
            width: 44px;
            height: 24px;
        }

        .switch input {
            opacity: 0;
            width: 0;
            height: 0;
        }

        .slider {
            position: absolute;
            cursor: pointer;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background-color: #ccc;
            transition: .4s;
        }

        .slider:before {
            position: absolute;
            content: "";
            height: 18px;
            width: 18px;
            left: 3px;
            bottom: 3px;
            background-color: white;
            transition: .4s;
        }

        input:checked + .slider { background-color: #28a745; }
        input:focus + .slider { box-shadow: 0 0 1px #28a745; }
        input:checked + .slider:before { transform: translateX(20px); }
        .slider.round { border-radius: 24px; }
        .slider.round:before { border-radius: 50%; }
        
        /* 按钮样式 */
        .btn {
            padding: 0.5rem 1rem;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-weight: 500;
            transition: background-color 0.2s, transform 0.2s;
            text-decoration: none;
            display: inline-block;
        }
        .btn-sm { padding: 0.25rem 0.5rem; font-size: 0.875rem; }
        .btn-primary { background-color: #007bff; color: white; }
        .btn-primary:hover { background-color: #0056b3; }
        .btn-success { background-color: #28a745; color: white; }
        .btn-danger { background-color: #dc3545; color: white; }
        .btn-warning { background-color: #ffc107; color: #212529; }
        .btn-info { background-color: #17a2b8; color: white; }

        #toast {
            position: fixed;
            bottom: 20px;
            left: 50%;
            transform: translateX(-50%);
            background-color: #333;
            color: #fff;
            padding: 10px 20px;
            border-radius: 5px;
            z-index: 1000;
            opacity: 0;
            transition: opacity 0.5s;
        }
        #toast.show { opacity: 1; }
    </style>
</head>
<body>
    <div class="admin-wrapper">
        <header class="admin-header">
            <div class="admin-header-content">
                <div>
                    <h1>Admin Panel</h1>
                </div>
                <nav class="admin-nav">
                    <a href="/logout" class="logout-link">Logout</a>
                </nav>
            </div>
        </header>
        <main class="admin-main">
            <div class="tab-container">
                <div class="tab-nav">
                    <button class="tab-button active" data-action="showTab" data-username="pending">
                        Pending Approvals
                    </button>
                    <button class="tab-button" data-action="showTab" data-username="users">
                        User Management
                    </button>
                </div>
                
                <div id="pending" class="tab-content">
                    <h2>Pending Approvals</h2>
                    <div id="pending-users-list">
                        <table class="user-table">
                            <thead>
                                <tr>
                                    <th>Username</th>
                                    <th>Registration Date</th>
                                    <th>Action</th>
                                </tr>
                            </thead>
                            <tbody>`
	const pendingUsersTmpl = `
                            {{range .PendingUsers}}
            <tr>
                <td>{{.Username}}</td>
                <td>{{.RegistrationDate.Format "Jan 02, 2006 15:04"}}</td>
                <td><button class="btn btn-success" data-action="approveUser" data-username="{{.Username}}">Approve</button></td>
            </tr>
                            {{end}}`
	const tmplPart2 = `
                            </tbody>
                        </table>
                    </div>
                </div>

                <div id="users" class="tab-content" style="display: none;">
                    <h2>All Users</h2>
                    <div class="form-section">
                        <h3>Create New User</h3>
                        <form id="create-user-form">
                            <div class="form-row">
                                <label for="new-username" class="form-label">Username:</label>
                                <div class="form-input">
                                    <input type="text" id="new-username" name="username" required>
                                </div>
                            </div>
                            <div class="form-row">
                                <label for="new-password" class="form-label">Password:</label>
                                <div class="form-input">
                                    <input type="password" id="new-password" name="password" required>
                                </div>
                            </div>
                            <div class="form-row">
                                <div class="form-label">Options:</div>
                                <div class="form-input">
                                    <div class="checkbox-group">
                                        <div class="checkbox-option">
                                            <input type="checkbox" id="new-is-admin" name="is_admin">
                                            <label for="new-is-admin">Admin User</label>
                                        </div>
                                        <div class="checkbox-option">
                                            <input type="checkbox" id="new-is-approved" name="is_approved" checked>
                                            <label for="new-is-approved">Approved</label>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <div class="form-row">
                                <div class="form-label"></div>
                                <div class="form-input">
                                    <button type="submit" class="btn btn-primary">Create User</button>
                                </div>
                            </div>
                        </form>
                    </div>
                    <div id="all-users-list" style="margin-top: 2rem;">
                        <h3>Existing Users</h3>
                        <table class="user-table">
                            <thead>
                                <tr>
                                    <th>Username</th>
                                    <th>Registration Date</th>
                                    <th>Status</th>
                                    <th>Role</th>
                                    <th>Actions</th>
                                </tr>
                            </thead>
                            <tbody>`
	const allUsersTmpl = `
                            {{range .AllUsers}}
            <tr>
                <td>{{.Username}}</td>
                <td>{{.RegistrationDate.Format "Jan 02, 2006 15:04"}}</td>
                <td>
                    {{if .IsApproved}}
                        <span style="color: green;">Approved</span>
                    {{else}}
                        <span style="color: orange;">Pending</span>
                    {{end}}
                </td>
                <td>
                    {{if .IsAdmin}}
                        Admin
                    {{else}}
                        User
                    {{end}}
                </td>
                <td>
                    {{if not .IsApproved}}
                        <button class="btn btn-success btn-sm" data-action="approveUser" data-username="{{.Username}}">Approve</button>
                    {{end}}
                    {{if .IsAdmin}}
                        <button class="btn btn-warning btn-sm" data-action="revokeAdmin" data-username="{{.Username}}">Revoke Admin</button>
                    {{else}}
                        <button class="btn btn-info btn-sm" data-action="makeAdmin" data-username="{{.Username}}">Make Admin</button>
                    {{end}}
                    <button class="btn btn-danger btn-sm" data-action="deleteUser" data-username="{{.Username}}">Delete</button>
                </td>
            </tr>
                            {{end}}`
	const tmplPart3 = `
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
        </main>
    </div>
    <div id="toast"></div>
    {{.Script}}
</body>
</html>`

	// Combine templates
	fullTmpl := tmpl + pendingUsersTmpl + tmplPart2 + allUsersTmpl + tmplPart3
	t, err := template.New("admin").Parse(fullTmpl)
	if err != nil {
		http.Error(w, "Failed to parse admin template", http.StatusInternalServerError)
		return
	}

	data := struct {
		PendingUsers []User
		AllUsers     []User
		Script       template.JS
	}{
		PendingUsers: pendingUsers,
		AllUsers:     allUsers,
		Script:       template.JS(adminGetMessageScript()),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "Failed to execute admin template", http.StatusInternalServerError)
		return
	}
}

func handleAdminApprove(w http.ResponseWriter, r *http.Request) { //line 605
	currentUser, err := getUserByUsernameDB(getSessionUser(r))
	if err != nil || !currentUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 查找用户以确保其存在
	userToApprove, err := getUserByUsernameDB(req.Username)
	if err != nil {
		http.Error(w, "Database error while fetching user", http.StatusInternalServerError)
		return
	}
	if userToApprove == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := updateUserApprovalDB(userToApprove.Username, true); err != nil {
		http.Error(w, "Failed to approve user", http.StatusInternalServerError)
		return
	}

	loadUsersIntoMemory()
	w.WriteHeader(http.StatusOK)
}

func handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	currentUser, err := getUserByUsernameDB(getSessionUser(r))
	if err != nil || !currentUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	username := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")

	switch r.Method {
	case http.MethodPost: // Create user
		var req struct {
			Username   string `json:"username"`
			Password   string `json:"password"`
			IsAdmin    bool   `json:"is_admin"`
			IsApproved bool   `json:"is_approved"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := createUserDB(req.Username, req.Password, req.IsAdmin, req.IsApproved); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		loadUsersIntoMemory()
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		if err := deleteUserDB(username); err != nil {
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}
		loadUsersIntoMemory()
		w.WriteHeader(http.StatusOK)

	case http.MethodPatch: // Update user (e.g., admin status)
		var req struct {
			Action string `json:"action"` // "make_admin", "revoke_admin", "approve"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var isAdmin bool
		switch req.Action {
		case "make_admin":
			isAdmin = true
		case "revoke_admin":
			isAdmin = false
		case "approve":
			if err := updateUserApprovalDB(username, true); err != nil {
				http.Error(w, "Failed to approve user", http.StatusInternalServerError)
				return
			}
			loadUsersIntoMemory()
			w.WriteHeader(http.StatusOK)
			return
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
			return
		}

		if err := updateUserAdminStatusDB(username, isAdmin); err != nil {
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
			return
		}
		loadUsersIntoMemory()
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func adminGetMessageScript() string {
	scriptContent := `
<script>
    window.showTab = function(tabName) {
        document.querySelectorAll('.tab-content').forEach(tab => tab.style.display = 'none');
        document.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
        document.getElementById(tabName).style.display = 'block';
        document.querySelector('.tab-button[data-username="' + tabName + '"]').classList.add('active');
    }

    window.showToast = function(message) {
        const toast = document.getElementById('toast');
        toast.textContent = message;
        toast.classList.add('show');
        setTimeout(() => { toast.classList.remove('show'); }, 3000);
    }

    window.handleAdminAction = async function(url, method, body, successMessage) {
        try {
            const response = await fetch(url, {
                method: method,
                headers: { 'Content-Type': 'application/json' },
                body: body ? JSON.stringify(body) : null
            });
            if (response.ok) {
                window.showToast(successMessage);
                setTimeout(() => window.location.reload(), 1000);
            } else {
                const errorText = await response.text();
                window.showToast('Error: ' + errorText);
            }
        } catch (error) {
            window.showToast('Network error: ' + error.message);
        }
    }

    window.approveUser = function(username) {
        window.handleAdminAction('/api/admin/users/' + username, 'PATCH', { action: 'approve' }, 'User approved successfully!');
    }

    window.deleteUser = function(username) {
        window.handleAdminAction('/api/admin/users/' + username, 'DELETE', null, 'User deleted successfully!');
    }

    window.makeAdmin = function(username) {
        window.handleAdminAction('/api/admin/users/' + username, 'PATCH', { action: 'make_admin' }, 'User promoted to admin!');
    }

    window.revokeAdmin = function(username) {
        window.handleAdminAction('/api/admin/users/' + username, 'PATCH', { action: 'revoke_admin' }, 'Admin status revoked!');
    }

    // Attach event listener only if the form exists
    const createUserForm = document.getElementById('create-user-form');
    if (createUserForm) {
        createUserForm.addEventListener('submit', (e) => {
            e.preventDefault();
            const body = {
                username: document.getElementById('new-username').value,
                password: document.getElementById('new-password').value,
                is_admin: document.getElementById('new-is-admin').checked,
                is_approved: document.getElementById('new-is-approved').checked
            };
            window.handleAdminAction('/api/admin/users/', 'POST', body, 'User created successfully!');
        });
    }

    // Initial tab setup
    document.addEventListener('DOMContentLoaded', () => {
        // This logic is now handled by the main script.js when the modal is opened,
        // or by the script itself if the page is loaded directly.
    });
</script>`
	// The script tag is now added in the main HTML body, so we just return the content.
	// This script will be picked up and executed by the modal logic in script.js
	return scriptContent
}
