# WebSSH-Go

[English](README.md)

一个使用 Go 语言编写的自托管、多用户的 WebSSH 客户端。它允许您通过现代 web 浏览器管理和连接到 SSH 服务器，并拥有持久化的数据库后端。

## 功能

*   **Web 端 SSH 终端**: 在浏览器中提供功能齐全的 xterm.js 终端。
*   **多用户支持**: 用户可以注册并管理自己的私有 SSH 连接。
*   **管理面板**: 管理员可以管理用户、批准新注册以及将用户提升为管理员。
*   **持久化存储**: 用户和连接数据存储在持久化的 SQLite 数据库中。
*   **安全凭证存储**: SSH 密码和私钥在静态时被加密。
*   **安全连接**: 支持通过 HTTPS 和 WSS 进行安全连接。
*   **可选认证**: 可以通过命令行标志禁用 web 界面的认证。
*   **文件传输**: 通过 SFTP 将文件从您的计算机直接上传到 SSH 服务器，并支持下载文件或整个目录（打包为 zip）。
*   **Docker 支持**: 使用 `docker-compose` 轻松部署。
*   **多平台支持**: 提供 `linux/amd64` 和 `linux/arm64` 的 Docker 镜像。同时为 `linux`、`windows` 和 `darwin` 系统提供 `amd64` 和 `arm64` 架构的二进制文件。

## 快速开始

### 使用 Docker (推荐)

这是最简单的入门方法。

1.  **获取项目:**
    克隆此仓库。
    ```bash
    git clone https://github.com/chenu/webssh-go.git
    cd webssh-go
    ```

2.  **配置 `docker-compose.yml`:**
    *   打开 `docker-compose.yml`。
    *   **关键步骤**：设置 `WEBSSH_ENCRYPTION_KEY`。使用 `openssl rand -hex 32` 生成一个安全密钥，并用它替换 `your_64_character_hex_key_here`。
    *   （可选）更改初始的 `WEBSSH_ADMIN_USER` 和 `WEBSSH_ADMIN_PASSWORD`。

3.  **使用 Docker Compose 运行:**
    ```bash
    docker-compose up --build
    ```

4.  **访问:**
    *   打开浏览器并访问 `http://localhost:8080`。
    *   使用您设置的管理员凭据登录（默认为 `admin`/`your_secure_admin_password`）。
    *   新用户可以注册，注册后需要等待管理员批准。

### Docker 高级配置

要传递 `--no-auth` 或 `--tls` 等命令行标志，请取消 `docker-compose.yml` 中 `command` 部分的注释并进行修改。

*   **示例: 启用 TLS/HTTPS**
    1.  创建一个 `certs` 目录，并将您的 `cert.pem` 和 `key.pem` 文件放入其中。
    2.  在 `docker-compose.yml` 中，取消 `ports` 下 `8443:8443` 映射的注释。
    3.  取消 `volumes` 下 `./certs:/app/certs` 映射的注释。
    4.  取消注释并更新 `command` 部分：
    ```yaml
    command: >
      ./webssh
      --tls
      --cert-file /app/certs/cert.pem
      --key-file /app/certs/key.pem
    ```
    修改后，使用 `docker-compose up --build` 重启服务。您的服务现在应该在 `https://localhost:8443` 上可用。

### 从源码构建 (不使用 Docker)

如果您想在没有 Docker 的情况下运行。

1.  **前提条件:**
    *   安装 Go (版本 1.24 或更高)。

2.  **克隆仓库:**
    ```bash
    git clone https://github.com/chenu/webssh-go.git
    cd webssh-go
    ```

3.  **编译:**
    ```bash
    go build
    ```

4.  **运行:**
    *   **启用多用户认证 (默认):**
        ```bash
        # 使用 openssl rand -hex 32 生成一个密钥
        export WEBSSH_ENCRYPTION_KEY="your_64_character_hex_key"
        # 设置初始管理员凭据 (默认为 admin/admin123)
        export WEBSSH_ADMIN_USER="admin"
        export WEBSSH_ADMIN_PASSWORD="your_secure_password"
        ./webssh
        ```
    *   **禁用认证:**
        ```bash
        ./webssh --no-auth
        ```
    *   **启用 TLS/HTTPS:**
        ```bash
        ./webssh --tls --cert-file /path/to/cert.pem --key-file /path/to/key.pem
        ```

5.  **访问:**
    *   对于 HTTP, 访问 `http://localhost:8080`。
    *   对于 HTTPS, 访问 `https://localhost:8443`。

## 配置

WebSSH-Go 可以通过环境变量和命令行标志进行配置。

### 环境变量

*   `WEBSSH_ADMIN_PASSWORD`: (多用户模式) `admin` 账户的密码。如果设置，程序启动时会用它来创建或重置管理员密码。默认为 `admin123`。
*   `WEBSSH_PASSWORD`: (单用户密码模式) `default` 账户的密码。当使用 `--single-user` 标志时生效。默认为 `default123`。
*   `WEBSSH_ENCRYPTION_KEY`: (**必需**) 一个 32 字节（64 个十六进制字符）的密钥，用于加密和解密数据库中敏感的连接信息（密码和私钥）。没有此密钥，应用程序将无法启动,后续不可更改，否则加密的连接信息将无法解密。
    *   您可以使用 OpenSSL 生成一个安全的密钥：
        ```bash
        openssl rand -hex 32
        ```

### 命令行标志

您可以使用 `./webssh --help` 查看所有可用的标志。
*   `--no-auth`: (布尔值, 默认为 `false`)
    启用“完全无认证模式”。禁用所有登录和认证，直接访问应用。适用于已有外部认证（如反向代理）的场景。此标志优先于 `--single-user`。
*   `--single-user`: (布尔值, 默认为 `false`)
    启用“单用户密码模式”。系统只有一个固定用户 `default`，其密码通过 `WEBSSH_PASSWORD` 环境变量设置。注册和管理功能被禁用。
    
*   `--tls`: (布尔值, 默认为 `false`)
    启用 TLS 以通过 HTTPS/WSS 提供服务。

*   `--cert-file`: (字符串)
    TLS 证书文件的路径 (例如, `cert.pem`)。当 `--tls` 启用时为必需。

*   `--key-file`: (字符串)
    TLS 私钥文件的路径 (例如, `key.pem`)。当 `--tls` 启用时为必需。

*   `--disable-download`: (布尔值, 默认为 `false`)
    仅禁用文件和目录的下载功能。文件上传和列表功能仍然可用。

*   `--disable-file-browser`: (布尔值, 默认为 `false`)
    禁用整个文件浏览器功能（包括上传、下载和列表）。此选项的优先级高于 `--disable-download`。

### 生成用于测试的 TLS 证书

要快速测试 HTTPS 功能，您可以使用 `openssl` 生成一个自签名证书：

```bash
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
```

这将在当前目录中创建 `key.pem` 和 `cert.pem` 文件。

## 使用方法

1.  **登录**: 如果启用了认证，请使用您配置的凭据登录。
    *   **管理员**: 使用管理员凭据登录 (例如, `admin`/`your_secure_password`)。
    *   **新用户**: 点击“注册”，创建一个账户，然后等待管理员在管理面板中批准。
2.  **管理面板**: 管理员会看到一个“Admin Panel”按钮。在这里，您可以批准待处理的注册并管理现有用户。
3.  **添加连接**:
    *   在主页上，填写 SSH 服务器的详细信息（名称、主机、用户、密码或私钥）。
    *   点击“保存连接”。
4.  **连接**:
    *   点击您想连接的已保存连接旁边的“连接”按钮。
    *   一个新的终端标签页将打开并建立 SSH 会话。
5.  **上传文件**:
    *   在活动的终端标签页中，点击右上角的 **文件夹图标** 打开文件浏览器。
    *   导航到您想要上传文件的目标目录。
    *   点击文件浏览器中的 "Upload File" 按钮并选择文件。
6.  **下载文件或目录**:
    *   在文件浏览器中，找到您想要下载的文件或目录。
    *   点击条目旁边的 "Download" 按钮。目录将被自动打包为 `.zip` 文件后下载。

## 许可证

该项目根据 MIT 许可证授权。
