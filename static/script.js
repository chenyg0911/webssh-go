document.addEventListener('DOMContentLoaded', () => {
    // DOM Elements
    const mainContainer = document.getElementById('main-container');
    const terminalView = document.getElementById('terminal-view');
    const tabsContainer = document.getElementById('tabs-container');
    const terminalWrapper = document.getElementById('terminal-wrapper');
    const connectionsList = document.getElementById('connections-list');
    const saveButton = document.getElementById('save-connection');
    // Tab controls
    const fileUploadInput = document.getElementById('file-upload-input');
    const fileBrowserBtn = document.getElementById('file-browser-btn');
    // File Browser Modal
    const fileBrowserModal = document.getElementById('file-browser-modal');
    const closeModalBtn = document.querySelector('.close-modal');
    const fbPathInput = document.getElementById('fb-path-input');
    const fbCdupBtn = document.getElementById('fb-cdup-btn');
    const fbRefreshBtn = document.getElementById('fb-refresh-btn');
    const newTabBtn = document.getElementById('new-tab-btn');
    const fbUploadBtn = document.getElementById('fb-upload-btn');
    const fileBrowserList = document.getElementById('file-browser-list');

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
    closeModalBtn.addEventListener('click', closeFileBrowser);

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
        .then(setInitialView);
});