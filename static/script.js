document.addEventListener('DOMContentLoaded', () => {
    // DOM Elements
    const mainContainer = document.getElementById('main-container');
    const terminalView = document.getElementById('terminal-view');
    const tabsContainer = document.getElementById('tabs-container');
    const terminalWrapper = document.getElementById('terminal-wrapper');
    const connectionsList = document.getElementById('connections-list');
    const saveButton = document.getElementById('save-connection');
    const adminBtn = document.getElementById('admin-btn');
    // Tab controls
    const fileUploadInput = document.getElementById('file-upload-input');
    const fileBrowserBtn = document.getElementById('file-browser-btn');
    // File Browser Modal
    const fileBrowserModal = document.getElementById('file-browser-modal');
    const fbPathInput = document.getElementById('fb-path-input');
    const fbCdupBtn = document.getElementById('fb-cdup-btn');
    const fbRefreshBtn = document.getElementById('fb-refresh-btn');
    const newTabBtn = document.getElementById('new-tab-btn');
    const fbUploadBtn = document.getElementById('fb-upload-btn');
    const fileBrowserList = document.getElementById('file-browser-list');
    // Admin Panel Modal
    const adminPanelModal = document.getElementById('admin-panel-modal');
    const adminPanelBody = document.getElementById('admin-panel-body');

    const nameInput = document.getElementById('name');
    const hostInput = document.getElementById('host');
    const userInput = document.getElementById('user');
    const passwordInput = document.getElementById('password');
    const keyInput = document.getElementById('key');

    // State Management
    let tabs = [];
    let activeTabId = null;
    let nextTabId = 1;
    let connections = [];
    let features = { download: true, fileBrowser: true }; // Default features
    let currentRemotePath = '';

    // Xterm.js custom theme
    const termTheme = {
        background: '#000000',
        foreground: '#dcdcdc',
        cursor: '#dcdcdc',
        selection: 'rgba(255, 255, 255, 0.3)',
        black: '#000000',
        red: '#cd3131',
        green: '#0dbc79',
        yellow: '#e5e510',
        blue: '#2472c8',
        magenta: '#bc3fbc',
        cyan: '#11a8cd',
        white: '#e5e5e5',
        brightBlack: '#666666',
        brightRed: '#f14c4c',
        brightGreen: '#23d18b',
        brightYellow: '#f5f543',
        brightBlue: '#3b8eea',
        brightMagenta: '#d670d6',
        brightCyan: '#29b8db',
        brightWhite: '#e5e5e5'
    };

    // --- Tab Management ---

    function createNewTab(connection) {
        const tabId = nextTabId++;
        
        const tabEl = document.createElement('div');
        tabEl.id = `tab-${tabId}`;
        tabEl.className = 'tab';
        tabEl.dataset.tabId = tabId;
        tabEl.innerHTML = `
            <span>${connection.name}</span>
            <span class="close-tab" data-tab-id="${tabId}">×</span>
        `;
        tabsContainer.insertBefore(tabEl, tabsContainer.querySelector('.tab-controls'));

        const termContainer = document.createElement('div');
        termContainer.id = `terminal-instance-${tabId}`;
        termContainer.className = 'terminal-instance';
        terminalWrapper.appendChild(termContainer);

        const term = new Terminal({
            cursorBlink: true,
            theme: termTheme 
        });
        const fitAddon = new FitAddon.FitAddon();
        term.loadAddon(fitAddon);
        term.open(termContainer);

        const newTab = {
            id: tabId,
            connection: connection,
            element: tabEl,
            termContainer: termContainer,
            term: term,
            fitAddon: fitAddon,
            socket: null,
            onDataDisposable: null,
        };
        tabs.push(newTab);

        connect(newTab);
        switchToTab(tabId);

        document.body.classList.add('terminal-active');
    }

    function switchToTab(tabId) {
        // No need to check for activeTabId === tabId here, 
        // as we might need to re-assert the view even if the tab is logically the same.
        
        activeTabId = tabId;

        tabs.forEach(tab => {
            const isActive = tab.id === tabId;
            tab.element.classList.toggle('active', isActive);
            // The .hidden class on termContainer is now managed by the parent's display property,
            // but we can keep it for clarity and immediate hiding/showing.
            tab.termContainer.classList.toggle('hidden', !isActive);
        });

        const activeTab = tabs.find(t => t.id === tabId);
        if (activeTab) {
            // Ensure the main terminal view is active
            if (!document.body.classList.contains('terminal-active')) {
                document.body.classList.add('terminal-active');
            }
            
            setTimeout(() => {
                activeTab.term.focus();
                fitTerminal(activeTab);
            }, 1); // A small delay ensures the element is fully visible and sized.
        }
    }

    function closeTab(tabId) {
        const tabIndex = tabs.findIndex(t => t.id === tabId);
        if (tabIndex === -1) return;

        const tabToClose = tabs[tabIndex];

        if (tabToClose.socket && tabToClose.socket.readyState === WebSocket.OPEN) {
            tabToClose.socket.close();
        }
        if (tabToClose.onDataDisposable) {
            tabToClose.onDataDisposable.dispose();
        }
        tabToClose.term.dispose();

        tabToClose.element.remove();
        tabToClose.termContainer.remove();

        tabs.splice(tabIndex, 1);

        if (activeTabId === tabId) {
            if (tabs.length > 0) {
                const newActiveTab = tabs[tabIndex] || tabs[tabIndex - 1] || tabs[0];
                switchToTab(newActiveTab.id);
            } else {
                activeTabId = null;
                document.body.classList.remove('terminal-active');
            }
        }
    }

    // --- WebSocket and Terminal Logic ---

    function fitTerminal(tab) {
        if (!tab || !tab.termContainer || tab.termContainer.offsetParent === null) return;
        try {
            tab.fitAddon.fit();
            if (tab.socket && tab.socket.readyState === WebSocket.OPEN) {
                tab.socket.send(JSON.stringify({ type: 'resize', cols: tab.term.cols, rows: tab.term.rows }));
            }
        } catch (e) {
            console.error("Fit error:", e);
        }
    }

    function connect(tab) {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const socket = new WebSocket(`${protocol}//${location.host}/ws?id=${tab.connection.id}`);
        tab.socket = socket;

        socket.onopen = () => {
            fitTerminal(tab);
            tab.onDataDisposable = tab.term.onData(data => {
                socket.send(JSON.stringify({ type: 'data', payload: data }));
            });
        };

        socket.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg.type && typeof msg.payload !== 'undefined') {
                    switch (msg.type) {
                        case 'stdout':
                            tab.term.write(msg.payload);
                            break;
                        case 'status':
                            tab.term.write(msg.payload);
                            if (socket.readyState === WebSocket.OPEN) {
                                // After a status message (like file upload), refresh file browser if open
                                if (!fileBrowserModal.classList.contains('hidden')) {
                                    requestFileList(currentRemotePath);
                                }
                                socket.send(JSON.stringify({ type: 'data', payload: '\r' }));
                            }
                            break;
                        case 'list':
                            renderFileList(JSON.parse(msg.payload));
                            break;
                        case 'download':
                            handleFileDownload(JSON.parse(msg.payload));
                            break;
                        case 'error':
                            // Display error in terminal and as an alert
                            const errorMsg = `\r\n\x1b[31mSERVER ERROR: ${msg.payload}\x1b[0m\r\n`;
                            tab.term.write(errorMsg);
                            alert(`Server Error: ${msg.payload}`);
                            break;
                        default:
                            console.warn("Unknown message type received:", msg.type);
                            tab.term.write(msg.payload);
                            break;
                    }
                }
            } catch (e) {
                tab.term.write(event.data);
            }
        };

        socket.onclose = () => {
            tab.term.write('\r\n\x1b[31mConnection closed.\x1b[0m\r\n');
        };

        socket.onerror = (err) => {
            console.error('WebSocket error:', err);
            tab.term.write('\r\n\x1b[31mConnection error.\x1b[0m\r\n');
        };
    }

    // --- File Upload ---
    
    function handleFileUpload(event) {
        const file = event.target.files[0];
        if (!file) return;

        const activeTab = tabs.find(t => t.id === activeTabId);
        if (!activeTab || !activeTab.socket || activeTab.socket.readyState !== WebSocket.OPEN) {
            alert("No active connection to upload file to.");
            return;
        }

        const reader = new FileReader();
        reader.onload = (e) => {
            const base64Content = e.target.result.split(',')[1];
            activeTab.socket.send(JSON.stringify({
                type: 'upload',
                filename: file.name,
                payload: base64Content,
                path: currentRemotePath, // Send current remote path
            }));
            activeTab.term.write(`\r\n\x1b[33mUploading ${file.name}...\x1b[0m\r\n`);
        };
        reader.onerror = (e) => {
            console.error("File reading error:", e);
            activeTab.term.write(`\r\n\x1b[31mError reading file: ${file.name}\x1b[0m\r\n`);
        };
        reader.readAsDataURL(file);
        fileUploadInput.value = '';
    }

    function handleFileDownload({ filename, payload }) {
        const byteCharacters = atob(payload);
        const byteNumbers = new Array(byteCharacters.length);
        for (let i = 0; i < byteCharacters.length; i++) {
            byteNumbers[i] = byteCharacters.charCodeAt(i);
        }
        const byteArray = new Uint8Array(byteNumbers);
        const blob = new Blob([byteArray]);

        const link = document.createElement('a');
        link.href = URL.createObjectURL(blob);
        link.download = filename;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    }

    // --- File Browser Logic ---

    function openFileBrowser() {
        const activeTab = tabs.find(t => t.id === activeTabId);
        if (!activeTab) {
            alert("Please open a connection tab first.");
            return;
        }
        fileBrowserModal.classList.remove('hidden');
        requestFileList(''); // Request root/home directory
    }

    function closeFileBrowser() {
        fileBrowserModal.classList.add('hidden');
        fileBrowserList.innerHTML = ''; // Clear content
        currentRemotePath = '';
    }

    async function checkAdminStatus() {
        try {
            // Use HEAD request for a lightweight check of admin permissions
            const response = await fetch('/admin', {
                method: 'HEAD',
                credentials: 'same-origin'
            });
            if (response.ok) {
                adminBtn.style.display = 'block';
            } else {
                adminBtn.style.display = 'none';
            }
        } catch (error) {
            adminBtn.style.display = 'none';
        }
    }

    function requestFileList(path) {
        const activeTab = tabs.find(t => t.id === activeTabId);
        if (activeTab && activeTab.socket && activeTab.socket.readyState === WebSocket.OPEN) {
            // Immediately update the path state to what we are requesting.
            // This prevents race conditions or mismatches if the backend's CWD is different
            // from the requested path (especially when requesting '/').
            currentRemotePath = path;
            fbPathInput.value = path;
            activeTab.socket.send(JSON.stringify({ type: 'list', path: path }));
        }
    }

    function renderFileList({ path, files }) {
        currentRemotePath = path;
        fbPathInput.value = path;
        fileBrowserList.innerHTML = '';

        // Sort files: directories first, then by name
        files.sort((a, b) => {
            if (a.isDir !== b.isDir) {
                return a.isDir ? -1 : 1;
            }
            return a.name.localeCompare(b.name);
        });

        files.forEach(file => {
            const li = document.createElement('div');
            li.className = 'file-item';
            // A more robust way to join path segments, handles root path correctly.
            let newPath = path;
            if (newPath === '/') newPath = ''; // Special case for root to avoid '//filename'
            li.dataset.path = newPath + '/' + file.name;

            li.dataset.isDir = file.isDir;

            const icon = file.isDir ? '📁' : '📄';
            const size = file.isDir ? '' : `(${(file.size / 1024).toFixed(2)} KB)`;

            li.innerHTML = `
                <span>${icon} ${file.name}</span>
                <span>${size}</span>
                ${features.fileBrowser && features.download ? `
                    <div class="file-actions">
                        <button class="btn-download">Download</button>
                    </div>
                ` : ''}
            `;
            fileBrowserList.appendChild(li);
        });
    }

    fileBrowserList.addEventListener('click', (e) => {
        const item = e.target.closest('.file-item');
        if (!item) return;

        const path = item.dataset.path; 
        const isDir = item.dataset.isDir === 'true';

        if (features.fileBrowser && features.download && e.target.classList.contains('btn-download')) {
            const activeTab = tabs.find(t => t.id === activeTabId);
            if (activeTab && activeTab.socket) {
                activeTab.socket.send(JSON.stringify({ type: 'download', path: path }));
            }
        } else if (isDir) {
            // Navigate into directory
            requestFileList(path);
        }
    });

    // --- Admin Panel Modal Logic ---

    async function openAdminPanel() {
        adminPanelBody.innerHTML = '<div class="loading-spinner"></div>';
        adminPanelModal.classList.remove('hidden');

        try {
            // Fetch the actual page content from a dedicated endpoint
            const response = await fetch('/admin/page');
            if (!response.ok) {
                throw new Error(`Failed to load admin panel: ${response.statusText}`);
            }
            const html = await response.text();
            
            // Use DOMParser to avoid script execution issues and safely extract content
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');
            const adminWrapper = doc.querySelector('.admin-wrapper');

            if (adminWrapper) {
                // The Go backend includes a header in the admin panel. 
                // We can hide it as the modal already has a header.
                const header = adminWrapper.querySelector('.admin-header');
                if (header) header.style.display = 'none';

                adminPanelBody.innerHTML = '';
                adminPanelBody.appendChild(adminWrapper);

                // Clean up any previously injected admin scripts to prevent conflicts
                const oldScript = document.getElementById('admin-panel-script');
                if (oldScript) {
                    oldScript.remove();
                }

                // Find the script tag in the fetched HTML and execute it to ensure \n                // functions are available in the global scope.\n                const scriptTags = doc.querySelectorAll('script');\n                console.log('Found ' + scriptTags.length + ' script tags in admin panel');\n                \n                // Instead of eval(), manually define the functions by extracting them\n                // from the script content and assigning them to window properties\n                scriptTags.forEach((scriptTag, index) => {\n                    const scriptContent = scriptTag.textContent;\n                    console.log('Processing admin script #' + index + ':', scriptContent.substring(0, 100) + '...');\n                    \n                    // Create a temporary function that will execute in the global context\n                    try {\n                        // Create a function whose body is the script content\n                        const scriptFunction = new Function(scriptContent + '\\n//# sourceURL=admin-panel-script-' + index + '.js');\n                        scriptFunction.call(window);\n                        console.log('Successfully executed admin script #' + index);\n                    } catch (e) {\n                        console.error('Error executing admin script #' + index + ':', e);\n                    }\n                });\n                \n                // Check if functions are available after execution\n                console.log('Checking if functions are available after script execution...');\n                console.log('window.approveUser exists:', typeof window.approveUser);\n                console.log('window.showTab exists:', typeof window.showTab);\n                console.log('window.deleteUser exists:', typeof window.deleteUser);
            } else {
                throw new Error('Could not find .admin-wrapper in response.');
            }
        } catch (error) {
            console.error('Error opening admin panel:', error);
            adminPanelBody.innerHTML = `<p class="error-message">${error.message}</p>`;
        }
    }

    // Define admin panel functions directly in the script
    window.showTab = function(tabName) {
        document.querySelectorAll('.tab-content').forEach(tab => tab.style.display = 'none');
        document.querySelectorAll('.tab-button').forEach(btn => btn.classList.remove('active'));
        document.getElementById(tabName).style.display = 'block';
        document.querySelector('.tab-button[data-username="' + tabName + '"]').classList.add('active');
    }

    // Custom confirm modal function
    window.showCustomConfirm = function(message, onConfirm) {
        // Remove any existing confirm modals
        const existingModal = document.getElementById('custom-confirm-modal');
        if (existingModal) {
            existingModal.remove();
        }
        
        // Create modal overlay
        const overlay = document.createElement('div');
        overlay.style.position = 'fixed';
        overlay.style.top = '0';
        overlay.style.left = '0';
        overlay.style.width = '100%';
        overlay.style.height = '100%';
        overlay.style.backgroundColor = 'rgba(0, 0, 0, 0.5)';
        overlay.style.zIndex = '9998';
        overlay.id = 'custom-confirm-overlay';
        
        // Create modal content
        const modal = document.createElement('div');
        modal.id = 'custom-confirm-modal';
        modal.style.position = 'fixed';
        modal.style.top = '50%';
        modal.style.left = '50%';
        modal.style.transform = 'translate(-50%, -50%)';
        modal.style.backgroundColor = 'white';
        modal.style.padding = '20px';
        modal.style.borderRadius = '8px';
        modal.style.boxShadow = '0 4px 12px rgba(0, 0, 0, 0.3)';
        modal.style.zIndex = '9999';
        modal.style.minWidth = '300px';
        modal.style.textAlign = 'center';
        
        modal.innerHTML = `
            <p style="margin-top: 0; margin-bottom: 15px;">${message}</p>
            <button id="confirm-yes" class="btn btn-danger" style="margin-right: 10px;">Yes</button>
            <button id="confirm-no" class="btn btn-secondary">No</button>
        `;
        
        // Add to document
        document.body.appendChild(overlay);
        document.body.appendChild(modal);
        
        // Handle confirm action
        document.getElementById('confirm-yes').addEventListener('click', function() {
            overlay.remove();
            modal.remove();
            onConfirm();
        });
        
        // Handle cancel action
        document.getElementById('confirm-no').addEventListener('click', function() {
            overlay.remove();
            modal.remove();
        });
        
        // Also close if clicking on overlay
        overlay.addEventListener('click', function() {
            overlay.remove();
            modal.remove();
        });
        
        // Prevent closing when clicking inside the modal
        modal.addEventListener('click', function(e) {
            e.stopPropagation();
        });
    }

    window.showToast = function(message) {
        // Try to find the toast element in the document
        let toast = document.getElementById('toast');
        
        // If toast element doesn't exist, create it
        if (!toast) {
            // Create toast element dynamically and add to body
            toast = document.createElement('div');
            toast.id = 'toast';
            toast.style.position = 'fixed';
            toast.style.bottom = '20px';
            toast.style.left = '50%';
            toast.style.transform = 'translateX(-50%)';
            toast.style.backgroundColor = '#333';
            toast.style.color = 'white';
            toast.style.padding = '10px 20px';
            toast.style.borderRadius = '5px';
            toast.style.zIndex = '1000';
            toast.style.opacity = '0';
            toast.style.transition = 'opacity 0.5s';
            document.body.appendChild(toast);
        }
        
        // Update and show the toast
        toast.textContent = message;
        toast.classList.add('show');
        
        // Clear any existing timeout
        if (window.toastTimeout) {
            clearTimeout(window.toastTimeout);
        }
        
        // Set new timeout to hide the toast
        window.toastTimeout = setTimeout(() => {
            toast.classList.remove('show');
        }, 3000);
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
                
                // Update the UI based on the operation type
                if (method === 'DELETE' && url.includes('/api/admin/users/')) {
                    // For delete operations, remove the user row from the table
                    const username = url.split('/').pop();
                    // Remove the row containing the deleted user
                    const rows = document.querySelectorAll('.user-table tbody tr');
                    for (let row of rows) {
                        const usernameCell = row.querySelector('td');
                        if (usernameCell && usernameCell.textContent.trim() === username) {
                            row.remove();
                            break; // Exit after removing the correct row
                        }
                    }
                } else if (method === 'PATCH' && body && body.action) {
                    const username = url.split('/').pop();
                    
                    // Handle approve action - needs to update both "Pending Approvals" and "All Users" tables
                    if (body.action === 'approve') {
                        // Remove the user from the "Pending Approvals" table
                        const pendingTable = document.querySelector('#pending .user-table tbody');
                        if (pendingTable) {
                            const pendingRows = pendingTable.querySelectorAll('tr');
                            for (let row of pendingRows) {
                                const usernameCell = row.querySelector('td');
                                if (usernameCell && usernameCell.textContent.trim() === username) {
                                    row.remove(); // Remove from pending table
                                    break;
                                }
                            }
                        }
                        
                        // Add the user to "All Users" table with approved status (or update if already exists)
                        const allUsersTable = document.querySelector('#users .user-table tbody');
                        if (allUsersTable) {
                            // Check if user already exists in all users table
                            let userExists = false;
                            const allUserRows = allUsersTable.querySelectorAll('tr');
                            for (let row of allUserRows) {
                                const usernameCell = row.querySelector('td');
                                if (usernameCell && usernameCell.textContent.trim() === username) {
                                    // Update existing row
                                    const statusCell = row.cells[2];
                                    const actionsCell = row.cells[4];
                                    if (statusCell && actionsCell) {
                                        statusCell.innerHTML = '<span style="color: green;">Approved</span>';
                                        // Remove approve button if it exists
                                        const approveBtn = actionsCell.querySelector(`button[data-action="approveUser"][data-username="${username}"]`);
                                        if (approveBtn) approveBtn.remove();
                                    }
                                    userExists = true;
                                    break;
                                }
                            }
                            
                            // If user doesn't exist in all users table, add them
                            if (!userExists) {
                                // Create a new row for the approved user
                                const newRow = document.createElement('tr');
                                
                                // Format current date to match backend format
                                const now = new Date();
                                const formattedDate = now.toLocaleString('en-US', {
                                    year: 'numeric',
                                    month: 'short',
                                    day: 'numeric',
                                    hour: '2-digit',
                                    minute: '2-digit'
                                }).replace(',', '');
                                
                                newRow.innerHTML = `
                                    <td>${username}</td>
                                    <td>${formattedDate}</td>
                                    <td><span style="color: green;">Approved</span></td>
                                    <td>User</td>
                                    <td>
                                        <button class="btn btn-info btn-sm" data-action="makeAdmin" data-username="${username}">Make Admin</button> 
                                        <button class="btn btn-danger btn-sm" data-action="deleteUser" data-username="${username}">Delete</button>
                                    </td>
                                `;
                                
                                allUsersTable.appendChild(newRow);
                            }
                        }
                    } else {
                        // Handle make_admin and revoke_admin actions (for existing users in the all users table)
                        const rows = document.querySelectorAll('.user-table tbody tr');
                        for (let row of rows) {
                            const usernameCell = row.querySelector('td');
                            if (usernameCell && usernameCell.textContent.trim() === username) {
                                // Update the status/approval/admin columns based on action
                                const statusCell = row.cells[2]; // Assuming status is in 3rd column
                                const roleCell = row.cells[3]; // Assuming role is in 4th column
                                const actionsCell = row.cells[4]; // Assuming actions is in 5th column
                                
                                if (statusCell && roleCell && actionsCell) {
                                    if (body.action === 'make_admin') {
                                        roleCell.textContent = 'Admin';
                                        // Update buttons in actions cell
                                        actionsCell.innerHTML = actionsCell.innerHTML
                                            .replace(`data-action="makeAdmin"`, `data-action="revokeAdmin"`)
                                            .replace('Make Admin', 'Revoke Admin')
                                            .replace('btn-info', 'btn-warning');
                                    } else if (body.action === 'revoke_admin') {
                                        roleCell.textContent = 'User';
                                        // Update buttons in actions cell
                                        actionsCell.innerHTML = actionsCell.innerHTML
                                            .replace(`data-action="revokeAdmin"`, `data-action="makeAdmin"`)
                                            .replace('Revoke Admin', 'Make Admin')
                                            .replace('btn-warning', 'btn-info');
                                    }
                                }
                                break;
                            }
                        }
                    }
                } else if (method === 'POST') {
                    // For POST (create user), add the new user to the appropriate table(s)
                    if (body && body.username) {
                        // Format current date to match backend format
                        const now = new Date();
                        const formattedDate = now.toLocaleString('en-US', {
                            year: 'numeric',
                            month: 'short',
                            day: 'numeric',
                            hour: '2-digit',
                            minute: '2-digit'
                        }).replace(',', '');
                        
                        // Status based on body.is_approved
                        const statusHtml = body.is_approved 
                            ? '<span style="color: green;">Approved</span>' 
                            : '<span style="color: orange;">Pending</span>';
                        
                        // Role based on body.is_admin
                        const roleText = body.is_admin ? 'Admin' : 'User';
                        
                        // Action buttons based on status and role
                        let actionButtons = '';
                        if (!body.is_approved) {
                            actionButtons += `<button class="btn btn-success btn-sm" data-action="approveUser" data-username="${body.username}">Approve</button> `;
                        }
                        if (body.is_admin) {
                            actionButtons += `<button class="btn btn-warning btn-sm" data-action="revokeAdmin" data-username="${body.username}">Revoke Admin</button> `;
                        } else {
                            actionButtons += `<button class="btn btn-info btn-sm" data-action="makeAdmin" data-username="${body.username}">Make Admin</button> `;
                        }
                        actionButtons += `<button class="btn btn-danger btn-sm" data-action="deleteUser" data-username="${body.username}">Delete</button>`;
                        
                        // If user is not approved, add to "Pending Approvals" table
                        if (!body.is_approved) {
                            const pendingTable = document.querySelector('#pending .user-table tbody');
                            if (pendingTable) {
                                const newRow = document.createElement('tr');
                                newRow.innerHTML = `
                                    <td>${body.username}</td>
                                    <td>${formattedDate}</td>
                                    <td><button class="btn btn-success" data-action="approveUser" data-username="${body.username}">Approve</button></td>
                                `;
                                pendingTable.appendChild(newRow);
                            }
                        }
                        
                        // Always add to "All Users" table
                        const allUsersTable = document.querySelector('#users .user-table tbody');
                        if (allUsersTable) {
                            const newRow = document.createElement('tr');
                            newRow.innerHTML = `
                                <td>${body.username}</td>
                                <td>${formattedDate}</td>
                                <td>${statusHtml}</td>
                                <td>${roleText}</td>
                                <td>${actionButtons}</td>
                            `;
                            allUsersTable.appendChild(newRow);
                        }
                        
                        // Clear form inputs after successful creation
                        const form = document.querySelector('#create-user-form');
                        if (form) {
                            form.reset();
                        }
                    }
                }
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

    // Also handle form submission separately
    adminPanelBody.addEventListener('submit', async (e) => {
        if (e.target.id === 'create-user-form') {
            e.preventDefault();
            
            const form = e.target;
            const body = {
                username: form.querySelector('#new-username').value,
                password: form.querySelector('#new-password').value,
                is_admin: form.querySelector('#new-is-admin').checked,
                is_approved: form.querySelector('#new-is-approved').checked
            };
            
            await window.handleAdminAction('/api/admin/users/', 'POST', body, 'User created successfully!');
        }
    });

    // --- Admin Panel Event Delegation ---
    adminPanelBody.addEventListener('click', (e) => {
        const target = e.target.closest('button');
        if (!target) return;

        const action = target.dataset.action;
        const username = target.dataset.username;

        console.log('Admin panel button clicked:', { action, username, target });
        console.log('Function exists:', action, typeof window[action]);

        // Check if a function with the name 'action' exists on the window object
        if (action && typeof window[action] === 'function') {
            // Special case for delete confirmation - use custom modal instead of browser confirm
            if (action === 'deleteUser') {
                // Create a custom confirmation modal
                showCustomConfirm(`Are you sure you want to delete user ${username}?`, () => {
                    window[action](username);
                });
            } else {
                window[action](username);
            }
        } else {
            console.warn(`Admin function ${action} not found or not a function`);
        }
    });

    // Also handle form submission separately
    adminPanelBody.addEventListener('submit', (e) => {
        if (e.target.id === 'create-user-form') {
            e.preventDefault();
            // The logic for this is inside the dynamically loaded script, which adds its own listener.
            // This is fine, but we could also centralize it here if needed.
        }
    });

    // --- API Calls and Event Listeners ---

    async function loadFeatures() {
        try {
            const response = await fetch('/api/features');
            features = await response.json();
            // Hide UI elements based on features
            if (!features.fileBrowser) {
                fileBrowserBtn.style.display = 'none';
            }
            if (!features.fileBrowser) { // Also hide upload button inside modal
                fbUploadBtn.style.display = 'none';
            }
        } catch (e) {
            console.error("Failed to load server features:", e);
        }
    }

    async function loadConnections() {
        try {
            const response = await fetch('/api/connections');
            connections = await response.json() || [];
            connectionsList.innerHTML = '';
            connections.forEach(conn => {
                const li = document.createElement('li');
                li.innerHTML = `
                    <span>${conn.name} <small>(${conn.user}@${conn.host})</small></span>
                    <div class="action-buttons">
                        <button class="btn btn-secondary" data-id="${conn.id}">Connect</button>
                        <button class="btn btn-danger" data-id="${conn.id}">Delete</button>
                    </div>
                `;
                connectionsList.appendChild(li);
            });
        } catch (e) {
            console.error("Failed to load connections:", e);
        }
    }

    async function deleteConnection(id) {
        await fetch(`/api/connections?id=${id}`, { method: 'DELETE' });
        loadConnections();
    }

    saveButton.addEventListener('click', async () => {
        const connection = {
            name: nameInput.value,
            host: hostInput.value,
            user: userInput.value,
            password: passwordInput.value,
            key: keyInput.value,
        };
        await fetch('/api/connections', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(connection),
        });
        [nameInput, hostInput, userInput, passwordInput, keyInput].forEach(i => i.value = '');
        loadConnections();
    });

    connectionsList.addEventListener('click', (event) => {
        const button = event.target.closest('button');
        if (!button) return;
        
        const connId = button.dataset.id;
        const connection = connections.find(c => c.id == connId);

        if (button.classList.contains('btn-secondary')) { // Connect
            if (connection) createNewTab(connection);
        }
        if (button.classList.contains('btn-danger')) { // Delete
            if (confirm(`Are you sure you want to delete "${connection.name}"?`)) {
                deleteConnection(connId);
            }
        }
    });

    tabsContainer.addEventListener('click', (event) => {
        const tabEl = event.target.closest('.tab');
        if (event.target.classList.contains('close-tab')) {
            closeTab(parseInt(event.target.dataset.tabId));
        } else if (tabEl) {
            switchToTab(parseInt(tabEl.dataset.tabId));
        }
    });

    newTabBtn.addEventListener('click', () => {
        activeTabId = null;
        document.body.classList.remove('terminal-active');
        tabs.forEach(tab => tab.element.classList.remove('active'));
    });

    fileUploadInput.addEventListener('change', handleFileUpload);

    fileBrowserBtn.addEventListener('click', openFileBrowser);
    
    // Generic modal close handler
    document.querySelectorAll('.modal .close-modal').forEach(btn => {
        btn.addEventListener('click', (e) => {
            // Find the parent modal and hide it
            e.target.closest('.modal').classList.add('hidden');
        });
    });

    // Close admin panel when clicking the close button
    adminPanelModal.querySelector('.close-modal').addEventListener('click', () => {
        // Clean up admin panel scripts when closing modal
        const adminScripts = document.querySelectorAll('[id^="admin-panel-script"]');
        adminScripts.forEach(script => script.remove());
        adminPanelModal.classList.add('hidden');
    });

    fbPathInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            requestFileList(e.target.value);
        }
    });

    fbUploadBtn.addEventListener('click', () => {
        fileUploadInput.click(); // Trigger the hidden file input
    });

    fbRefreshBtn.addEventListener('click', () => {
        requestFileList(currentRemotePath);
    });

    fbCdupBtn.addEventListener('click', () => {
        if (currentRemotePath && currentRemotePath !== '/') {
            const lastSlashIndex = currentRemotePath.lastIndexOf('/');
            
            if (lastSlashIndex > 0) { 
                // Path is like '/home/user', parent is '/home'
                requestFileList(currentRemotePath.substring(0, lastSlashIndex));
            } else if (lastSlashIndex === 0) { 
                // Path is like '/root' or '/home', parent is the root directory '/'
                requestFileList('/');
            }
        }
    });

    adminBtn.addEventListener('click', openAdminPanel);


    window.addEventListener('resize', () => {
        const activeTab = tabs.find(t => t.id === activeTabId);
        if (activeTab) {
            fitTerminal(activeTab);
        }
    });

    // --- Initial Load ---
    function setInitialView() {
        // This function ensures the correct view is displayed on load.
        // If there are no open tabs, show the connection manager.
        if (tabs.length === 0) {
            document.body.classList.remove('terminal-active');
            activeTabId = null;
        } else {
            // If there are tabs (e.g., from a persisted state in the future),
            // ensure the terminal view is active.
            document.body.classList.add('terminal-active');
            if (activeTabId === null) {
                switchToTab(tabs[0].id);
            }
        }
    }

    // Load connections and then set the initial view.
    loadFeatures()
        .then(loadConnections)
        .then(() => {
		checkAdminStatus();
	})
        .then(setInitialView);
});