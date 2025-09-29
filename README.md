# WebSSH-Go

[简体中文](README.zh-CN.md)

A self-hosted, multi-user WebSSH client written in Go. It allows you to manage and connect to your SSH servers through a modern web browser, with a persistent database backend.

## Features

*   **Web-based SSH Terminal**: Provides a full-featured xterm.js terminal in your browser.
*   **Multi-User Support**: Users can register and manage their own private SSH connections.
*   **Admin Panel**: Administrators can manage users, approve new registrations, and promote users to admins.
*   **Persistent Storage**: User and connection data are stored in a persistent SQLite database.
*   **Secure Credential Storage**: SSH passwords and private keys are encrypted at rest.
*   **Secure Connections**: Supports secure connections via HTTPS and WSS.
*   **Optional Authentication**: The web interface authentication can be disabled via a command-line flag.
*   **File Transfer**: Upload files from your computer directly to the SSH server via SFTP, and download files or entire directories (as a zip).
*   **Docker Support**: Easy to deploy using `docker-compose`.
*   **Multi-Platform Support**: Docker images are available for `linux/amd64` and `linux/arm64`. Binaries are provided for `linux`, `windows`, and `darwin` on both `amd64` and `arm64` architectures.

## Quick Start

### Using Docker (Recommended)

This is the easiest way to get started.

1.  **Get the Project:**
    Clone this repository.
    ```bash
    git clone https://github.com/chenu/webssh-go.git
    cd webssh-go
    ```

2.  **Configure `docker-compose.yml`:**
    *   Open `docker-compose.yml`.
    *   **Crucially**, set the `WEBSSH_ENCRYPTION_KEY`. Generate a secure key with `openssl rand -hex 32` and replace `your_64_character_hex_key_here` with it.
    *   Optionally, set the `WEBSSH_ADMIN_PASSWORD`. The admin username is fixed to `admin`.

3.  **Run with Docker Compose:**
    ```bash
    docker-compose up --build
    ```

4.  **Access:**
    *   Open your browser and navigate to `http://localhost:8080`.
    *   Log in with the admin credentials (the username is `admin`, and the password is what you set in `WEBSSH_ADMIN_PASSWORD`).
    *   New users can register and will await admin approval.

### Advanced Docker Configuration

To pass command-line flags like `--no-auth` or `--tls`, uncomment and modify the `command` section in `docker-compose.yml`.

*   **Example: Enable TLS/HTTPS**
    1.  Create a `certs` directory and place your `cert.pem` and `key.pem` files inside.
    2.  Uncomment the `ports` mapping for `8443:8443` in `docker-compose.yml`.
    3.  Uncomment the `volumes` mapping for `./certs:/app/certs`.
    4.  Uncomment and update the `command` section:
    ```yaml
    command: >
      ./webssh
      --tls
      --cert-file /app/certs/cert.pem
      --key-file /app/certs/key.pem
    ```
    Restart the service with `docker-compose up --build`. Your service will be available at `https://localhost:8443`.

### Building from Source (Without Docker)

If you prefer to run without Docker.

1.  **Prerequisites:**
    *   Install Go (version 1.24 or later).

2.  **Clone the Repository:**
    ```bash
    git clone https://github.com/chenu/webssh-go.git
    cd webssh-go
    ```

3.  **Build:**
    ```bash
    go build
    ```

4.  **Run:**
    *   **With Multi-User Authentication (Default):**
        ```bash
        # Generate a key with: openssl rand -hex 32
        export WEBSSH_ENCRYPTION_KEY="your_64_character_hex_key_here"
        # Set the admin password (username is 'admin'). Defaults to 'admin123'.
        export WEBSSH_ADMIN_PASSWORD="your_secure_password"
        ./webssh
        ```
    *   **With Single-User Password Mode:**
        `export WEBSSH_PASSWORD="your_secure_password" ./webssh --single-user`

    *   **With No Authentication:**
        ```bash
        ./webssh --no-auth
        ```
    *   **With TLS/HTTPS:**
        ```bash
        ./webssh --tls --cert-file /path/to/cert.pem --key-file /path/to/key.pem
        ```

5.  **Access:**
    *   For HTTP, visit `http://localhost:8080`.
    *   For HTTPS, visit `https://localhost:8443`.

## Configuration

WebSSH-Go can be configured via environment variables and command-line flags.

### Environment Variables

*   `WEBSSH_ADMIN_PASSWORD`: (Multi-User Mode) The password for the `admin` account. If set, it will be used to create or reset the admin password on startup. Defaults to `admin123`.
*   `WEBSSH_PASSWORD`: (Single-User Password Mode) The password for the `default` account. Effective when using the `--single-user` flag. Defaults to `default123`.
*   `WEBSSH_ENCRYPTION_KEY`: (**Required**) A 32-byte (64-character hex string) key used to encrypt and decrypt sensitive connection details (passwords and private keys) in the database. The application will not start without it. **This key cannot be changed later**, or all encrypted connection info will become unreadable.
    *   You can generate a secure key using OpenSSL:
        ```bash
        openssl rand -hex 32
        ```

### Command-Line Flags

You can view all available flags with `./webssh --help`.
*   `--no-auth`: (boolean, default `false`)
    Enables "no-authentication mode". Disables all login and authentication, providing direct access to the application. Ideal for environments with existing external authentication (e.g., a reverse proxy). This flag takes precedence over `--single-user`.
*   `--single-user`: (boolean, default `false`)
    Enables "single-user password mode". The system has only one fixed user, `default`, whose password is set via the `WEBSSH_PASSWORD` environment variable. Registration and admin features are disabled.

*   `--tls`: (boolean, default `false`)
    Enables TLS to serve over HTTPS/WSS.

*   `--cert-file`: (string)
    Path to the TLS certificate file (e.g., `cert.pem`). Required when `--tls` is enabled.

*   `--key-file`: (string)
    Path to the TLS private key file (e.g., `key.pem`). Required when `--tls` is enabled.

*   `--disable-download`: (boolean, default `false`)
    Disables only the file and directory download functionality. Uploading and listing files will still be possible.

*   `--disable-file-browser`: (boolean, default `false`)
    Disables the entire file browser functionality (upload, download, and listing). This option takes precedence over `--disable-download`.

### Generating a TLS Certificate for Testing

To quickly test the HTTPS functionality, you can generate a self-signed certificate using `openssl`:

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
```

This will create `key.pem` and `cert.pem` files in the current directory.

## Usage

1.  **Login**: If authentication is enabled, log in with your configured credentials.
    *   **Admins**: Use the admin credentials (e.g., `admin`/`your_secure_password`).
    *   **New Users**: Click "Register", create an account, and wait for an administrator to approve it from the Admin Panel.
2.  **Admin Panel**: Admins will see an "Admin Panel" button. From there, you can approve pending registrations and manage existing users.
3.  **Add a Connection**:
    *   On the main page, fill in the details for your SSH server (Name, Host, User, Password or Private Key).
    *   Click "Save Connection".
4.  **Connect**:
    *   Click the "Connect" button next to the saved connection you wish to use.
    *   A new terminal tab will open and establish the SSH session.
5.  **Upload a File**:
    *   In an active terminal tab, click the **folder icon** in the top-right to open the File Browser.
    *   Navigate to the target directory where you want to upload the file.
    *   Click the "Upload File" button in the file browser and select your file.
6.  **Download a File or Directory**:
    *   In the File Browser, find the file or directory you want to download.
    *   Click the "Download" button next to the entry. Directories will be automatically zipped before being downloaded.

## License

This project is licensed under the MIT License.
