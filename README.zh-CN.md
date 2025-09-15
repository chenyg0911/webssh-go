# WebSSH-Go

[English](README.md)

一个使用 Go 语言编写的简单的自托管 WebSSH 客户端。它允许您通过现代 web 浏览器管理和连接到 SSH 服务器。

## 功能

*   **Web 端 SSH 终端**: 在浏览器中提供功能齐全的 xterm.js 终端。
*   **连接管理**: 保存和管理多个 SSH 连接，支持密码和密钥认证。
*   **安全连接**: 支持通过 HTTPS 和 WSS 进行安全连接。
*   **可选认证**: 可以通过命令行标志禁用 web 界面的认证。
*   **文件传输**: 通过 SFTP 将文件从您的计算机直接上传到 SSH 服务器，并支持下载文件或整个目录（打包为 zip）。
*   **Docker 支持**: 使用 `docker-compose` 轻松部署。
*   **多平台支持**: 提供 `linux/amd64` 和 `linux/arm64` 的 Docker 镜像。同时为 `linux`、`windows` 和 `darwin` 系统提供 `amd64` 和 `arm64` 架构的二进制文件。

## 快速开始

### 使用 Docker (推荐)

这是最简单的入门方法。

1.  **获取项目:**
    克隆此仓库以从源码构建，或者如果您计划使用预构建的镜像，请确保您有 `docker-compose.yml` 文件。
    ```bash
    git clone <repository_url>
    cd webssh-go
    ```

2.  **运行容器:**
    *   **选项 A: 从源码构建**
        这将使用您本地的代码构建一个新镜像。
        ```bash
        docker-compose up --build
        ```
    *   **选项 B: 使用预构建的镜像**
        如果您有一个预构建的镜像，请修改 `docker-compose.yml`，将 `build: .` 替换为 `image: your-image-name:tag`，docker hub的预构建镜像: chenu2/webssh:latest。然后运行：
        ```bash
        docker-compose up
        ```

3.  **访问:**
    打开浏览器并访问 `http://localhost:8080`。

### Docker 高级配置

您可以编辑 `docker-compose.yml` 文件来更改应用程序的启动方式。

**设置启动参数:**

要传递 `--no-auth` 或 `--tls` 等命令行标志，请取消 `docker-compose.yml` 中 `command` 部分的注释并进行修改。

*   **示例: 禁用认证**
    ```yaml
    command: >
      ./webssh
      --no-auth
    ```

*   **示例: 启用 TLS/HTTPS**
    1.  使用 `openssl` 生成证书和密钥，并将它们放在一个 `certs` 目录中。
    2.  取消 `volumes` 下的 `./certs:/app/certs` 行的注释。
    3.  更新 `command` 部分，如下所示：
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
    *   安装 [Go](https://golang.org/doc/install) (版本 1.21 或更高)。

2.  **克隆仓库:**
    ```bash
    git clone <repository_url>
    cd webssh-go
    ```

3.  **编译:**
    ```bash
    go build
    ```

4.  **运行:**
    *   **启用认证 (默认):**
        ```bash
        export WEBSSH_USER="your_username"
        export WEBSSH_PASSWORD="your_password"
        # 使用 openssl rand -hex 32 生成一个密钥
        export WEBSSH_ENCRYPTION_KEY="your_64_character_hex_key"
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

*   `WEBSSH_USER`: 用于 web 界面登录的用户名。
*   `WEBSSH_PASSWORD`: 用于 web 界面登录的密码。

*   `WEBSSH_ENCRYPTION_KEY`: (**必需**) 一个 32 字节（64 个十六进制字符）的密钥，用于加密和解密 `connections.json` 文件中敏感的连接信息（密码和私钥）。没有此密钥，应用程序将无法启动。
    *   您可以使用 OpenSSL 生成一个安全的密钥：
        ```bash
        openssl rand -hex 32
        ```

> **注意:** 只有在启用认证时才需要这些变量。

### 命令行标志

您可以使用 `./webssh --help` 查看所有可用的标志。

*   `--no-auth`: (布尔值, 默认为 `false`)
    禁用 web 界面的用户认证。设置此标志后，`WEBSSH_USER` 和 `WEBSSH_PASSWORD` 将被忽略。

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
2.  **添加连接**:
    *   在主页上，填写 SSH 服务器的详细信息（名称、主机、用户、密码或私钥）。
    *   点击“保存连接”。
3.  **连接**:
    *   点击您想连接的已保存连接旁边的“连接”按钮。
    *   一个新的终端标签页将打开并建立 SSH 会话。
4.  **上传文件**:
    *   在活动的终端标签页中，点击右上角的“上传”按钮。
    *   在活动的终端标签页中，点击右上角的 **文件夹图标** 打开文件浏览器。
    *   导航到您想要上传文件的目标目录。
    *   点击文件浏览器中的 "Upload File" 按钮并选择文件。
5.  **下载文件或目录**:
    *   在文件浏览器中，找到您想要下载的文件或目录。
    *   点击条目旁边的 "Download" 按钮。目录将被自动打包为 `.zip` 文件后下载。

## 许可证

该项目根据 MIT 许可证授权。
