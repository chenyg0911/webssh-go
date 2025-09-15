# WebSSH-Go

[简体中文](README.zh-CN.md)

A simple, self-hosted WebSSH client written in Go. It allows you to manage and connect to your SSH servers through a modern web browser.

## Features

*   **Web-based SSH Terminal**: Provides a full-featured xterm.js terminal in your browser.
*   **Connection Management**: Save and manage multiple SSH connections, supporting both password and private key authentication.
*   **Secure Connections**: Supports secure connections via HTTPS and WSS.
*   **Optional Authentication**: The web interface authentication can be disabled via a command-line flag.
*   **File Transfer**: Upload files from your computer directly to the SSH server via SFTP, and download files or entire directories (as a zip).
*   **Docker Support**: Easy to deploy using `docker-compose`.
*   **Multi-Platform Support**: Docker images are available for `linux/amd64` and `linux/arm64`. Binaries are provided for `linux`, `windows`, and `darwin` on both `amd64` and `arm64` architectures.

## Quick Start

### Using Docker (Recommended)

This is the easiest way to get started.

1.  **Get the Project:**
    Clone this repository to build from source, or ensure you have the `docker-compose.yml` file if you plan to use a pre-built image.
    ```bash
    git clone <repository_url>
    cd webssh-go
    ```

2.  **Run the Container:**
    *   **Option A: Build from Source**
        This will build a new image using your local code.
        ```bash
        docker-compose up --build
        ```
    *   **Option B: Use a Pre-built Image**
        If you have a pre-built image, modify `docker-compose.yml` by replacing `build: .` with `image: your-image-name:tag`, a pre-build image on docker hub: chenu2/webssh:latest.  then run:
        ```bash
        docker-compose up
        ```

3.  **Access:**
    Open your browser and navigate to `http://localhost:8080`.

### Advanced Docker Configuration

You can edit the `docker-compose.yml` file to change how the application starts.

**Setting Startup Arguments:**

To pass command-line flags like `--no-auth` or `--tls`, uncomment and modify the `command` section in `docker-compose.yml`.

*   **Example: Disable Authentication**
    ```yaml
    command: >
      ./webssh
      --no-auth
    ```

*   **Example: Enable TLS/HTTPS**
    1.  Generate a certificate and key using `openssl` and place them in a `certs` directory.
    2.  Uncomment the `./certs:/app/certs` line under `volumes`.
    3.  Update the `command` section as follows:
    ```yaml
    command: >
      ./webssh
      --tls
      --cert-file /app/certs/cert.pem
      --key-file /app/certs/key.pem
    ```
    After making changes, restart the service with `docker-compose up --build`. Your service should now be available at `https://localhost:8443`.

### Building from Source (Without Docker)

If you prefer to run without Docker.

1.  **Prerequisites:**
    *   Install Go (version 1.21 or later).

2.  **Clone the Repository:**
    ```bash
    git clone <repository_url>
    cd webssh-go
    ```

3.  **Build:**
    ```bash
    go build
    ```

4.  **Run:**
    *   **With Authentication (Default):**
        ```bash
        export WEBSSH_USER="your_username"
        export WEBSSH_PASSWORD="your_password"
        # Generate a key with: openssl rand -hex 32
        export WEBSSH_ENCRYPTION_KEY="your_64_character_hex_key"
        ./webssh
        ```
    *   **Without Authentication:**
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

*   `WEBSSH_USER`: The username for the web interface login.
*   `WEBSSH_PASSWORD`: The password for the web interface login.

*   `WEBSSH_ENCRYPTION_KEY`: (**Required**) A 32-byte (64-character hex string) key used to encrypt and decrypt sensitive connection details (passwords and private keys) in `connections.json`. The application will not start without it.
    *   You can generate a secure key using OpenSSL:
        ```bash
        openssl rand -hex 32
        ```
> **Note:** These variables are only required when authentication is enabled.

### Command-Line Flags

You can view all available flags with `./webssh --help`.

*   `--no-auth`: (boolean, default `false`)
    Disables user authentication for the web interface. If set, `WEBSSH_USER` and `WEBSSH_PASSWORD` are ignored.

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
2.  **Add a Connection**:
    *   On the main page, fill in the details for your SSH server (Name, Host, User, Password or Private Key).
    *   Click "Save Connection".
3.  **Connect**:
    *   Click the "Connect" button next to the saved connection you wish to use.
    *   A new terminal tab will open and establish the SSH session.
4.  **Upload a File**:
    *   In an active terminal tab, click the **folder icon** in the top-right to open the File Browser.
    *   Navigate to the target directory where you want to upload the file.
    *   Click the "Upload File" button in the file browser and select your file.
5.  **Download a File or Directory**:
    *   In the File Browser, find the file or directory you want to download.
    *   Click the "Download" button next to the entry. Directories will be automatically zipped before being downloaded.

## License

This project is licensed under the MIT License.
