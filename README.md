# Gatekeeper

<div align="center">

<img width="600" src="./images/screenshot.webp" alt="Gatekeeper Login Page Screenshot" />

<i>Screenshot of the Gatekeeper login page with the "hearts" and "winxp" plugins</i>

</div>

Gatekeeper is a lightweight, virtual-host-aware reverse proxy and session authentication gateway written in Go. It secures your web applications, internal tools, APIs, and documentation by intercepting incoming requests, displaying a premium login screen for unauthenticated users, and proxying authenticated sessions to target upstreams.

---

## Features

- **Multi-Port Listeners**: Configure and listen on multiple network ports concurrently (e.g., public apps on `:8080` and internal dashboards on `:9090`).
- **Virtual Hosting (`server_name`)**: Route incoming HTTP requests on the same port to different upstream backends based on the request's `Host` header (matching exact hostnames, no wildcards). This directive is optional; if omitted, the server block acts as a catch-all/default fallback server for all unmatched requests on that listener port.
- **Hierarchical Config Scoping**: Define a default global configuration and selectively extend or override settings at the listener and virtual host (server) levels.
- **Visual Login Page Plugins**: Customize the login screen dynamically using built-in interactive animation plugins (`aurora`, `matrix`, `hearts`) scoped globally or per virtual host.
- **Comment-Preserving Config Saves**: Managing configuration settings programmatically via the CLI preserves existing file layout, spacing, and comments.
- **Zero-Downtime Reloads**: Reload the server configuration seamlessly without dropping active connections by sending a `SIGHUP` signal.
- **Graceful Shutdown**: Stops accepting new connections and finishes processing active requests before exiting.
- **Built-in 5xx Pages**: Returns branded HTML error pages for internal failures, unavailable upstreams, and upstream timeouts.
- **Structured Access Logs**: Optionally persists one JSON record per request, including status, size, latency, client address, and authenticated user.
- **Persistent Error Logs**: Optionally duplicates operational and proxy errors to a restricted append-only file while retaining console output.

---

## Configuration Scoping Rules

Gatekeeper merges global settings with listener/server-level configurations using the following inheritance rules:

| Scope Directive | Merge Strategy | Behaviour                                                                                                              |
| :-------------- | :------------- | :--------------------------------------------------------------------------------------------------------------------- |
| `app_name`      | **Fallback**   | Configures the white-labeled portal display name (logo, title, logs). Defaults to `"Gatekeeper"`.                      |
| `server_name`   | **Optional**   | Hostname to match. If omitted, the block acts as a catch-all default fallback for the listener (at most one per port). |
| `users`         | **Union**      | Both global and server-local users are authorized to log in. Local users shadow global users if usernames conflict.    |
| `plugins`       | **Merge**      | Global plugins are active by default. Server-specific plugin maps can override individual plugins (enable/disable).    |
| `auth`          | **Override**   | A server-level `auth` block overrides individual global fields (such as `cookie_name` and `session_ttl`).              |
| `security`      | **Override**   | A server-level `security` block overrides individual global fields such as `secure_cookies`, `same_site`, and `authorize_favicon`. |
| `upstream`      | **Required**   | Configured per server block. Defines the backend destination URL (e.g. `http://localhost:3000`).                       |

---

## Quickstart

1. **Build or Install Gatekeeper**:
   - To build locally:
     ```bash
     go build -o gatekeeper
     ```
   - To install globally:
     ```bash
     go install github.com/velox0/gatekeeper@latest
     ```

2. **Run with Config**:

   ```bash
   gatekeeper -config /path/to/config.yml
   ```

   Default config path: `/etc/gatekeeper/config.yml`

   For local/dev, pass an explicit file:

   ```bash
   sudo ./gatekeeper -config config.example.yml
   ```

   _(Note: Binding to low ports or managing system-wide PID files may require root privileges)._

---

## Configuration File Example

The config file uses standard YAML. Comments are preserved when saved dynamically via the command line.

```yaml
# ─── Global scope (inherited by all listeners/servers unless overridden) ───
app_name: Gatekeeper # Custom display name for logs and login UI
auth:
  cookie_name: gatekeeper_session
  session_ttl: 24h0m0s
security:
  secure_cookies: true
  same_site: strict
  authorize_favicon: false # if true, unauthenticated GET /favicon.ico is proxied upstream
users:
  - username: global-admin
    password_hash: $2a$10$LcE5wk0JkOscgj2S3p3cq.4f1GS4cW8XrSDDAAweQD29OWKLvt08K # bcrypt hash for "test"
plugins:
  aurora: false
  hearts: true
  matrix: false

# ─── Listeners & Virtual Hosts ───
listeners:
  # Example 1: Catch-all listener (no server_name). Matches any host header on port :8080.
  - listen: ":8080"
    servers:
      - upstream:
          target: http://localhost:3000

  # Example 2: Virtual Host routing (with server_name). Only matches matching Host header on port :9090.
  - listen: ":9090"
    servers:
      - server_name: app.local
        upstream:
          target: http://localhost:4000
        plugins:
          matrix: true # Overrides global: enable matrix, disable hearts
          hearts: false

      - server_name: admin.local
        upstream:
          target: http://localhost:5000
        users:
          - username: admin-user # Local user only allowed on admin.local
            password_hash: $2a$10$LcE5wk0JkOscgj2S3p3cq.4f1GS4cW8XrSDDAAweQD29OWKLvt08K
```

---

## Command Line Interface (CLI)

Gatekeeper includes interactive management tools to adjust users and login page plugins without editing configuration files manually. You can get command-line help by appending `help` to any command (e.g., `./gatekeeper help`, `./gatekeeper user help`, or `./gatekeeper plugin help`).

### User Management

User commands modify the config file, require root privileges, and prompt securely for the password interactively (requiring 2-time confirmation).

- **Add User**:
  ```bash
  sudo ./gatekeeper user add <username>
  ```

- **Update Password**:
  ```bash
  sudo ./gatekeeper user update <username>
  ```
- **Remove User**:
  ```bash
  sudo ./gatekeeper user remove <username>
  ```

### Plugin Management

Manage visual login page effects.

- **List Plugins**:
  ```bash
  ./gatekeeper plugin list
  ```
- **Enable Plugin**:
  ```bash
  ./gatekeeper plugin enable <name>
  ```
- **Disable Plugin**:
  ```bash
  ./gatekeeper plugin disable <name>
  ```

### Configuration Management

Manage configuration initialization and background service setup instructions tailored to your machine's OS.

- **Initialize Default Config**: Generates a default configuration template at `~/.gatekeeper/config.yml` (if it does not exist already):
  ```bash
  ./gatekeeper config init
  ```
- **Generate Background Service Config**: Prints configuration files and commands to register Gatekeeper as a system daemon (`systemd` service for Linux, `launchd` plist for macOS):
  ```bash
  ./gatekeeper config service
  ```

### Scope Selection Prompt

When executing management commands, Gatekeeper will list all configured server blocks and prompt you to specify the target scope(s):

```
Available scopes:
  [0] global (applies to all server blocks)
  [1] :8080 → app.local
  [2] :8080 → docs.local
  [3] :9090 → admin.local

Select scope(s) (e.g. 0, 1, 1-3, 1,3):
```

You can enter multiple scopes separated by commas (e.g. `1,3`), ranges (e.g. `1-3`), or `0` for the global configuration. Once saved, Gatekeeper automatically signals any running daemon to reload the configuration.

---

## Signal Handling & Daemon Control

### Access Logs

Set `GATEKEEPER_ACCESS_LOG` to a file path to append structured JSON access logs. Set it to `-` to write access records to standard output; leave it unset to disable access logging.

```bash
GATEKEEPER_ACCESS_LOG=/var/log/gatekeeper/access.log gatekeeper -config /etc/gatekeeper/config.yml
```

### Error Logs

Set `GATEKEEPER_ERROR_LOG` to a file path to append runtime and proxy errors while continuing to emit them to standard error.

```bash
GATEKEEPER_ERROR_LOG=/var/log/gatekeeper/error.log gatekeeper -config /etc/gatekeeper/config.yml
```

Gatekeeper writes its process ID (PID) to a file on startup.

- **PID File Path**: Defaults to `/var/run/gatekeeper.pid` (configure via the `-pid` command flag).
- **Reload Configuration**: Send `SIGHUP` to trigger a safe config hot-reload. The config is reloaded into memory, and newly configured users/plugins take effect immediately without dropping current client connections:
  ```bash
  sudo kill -HUP $(cat /var/run/gatekeeper.pid)
  ```
- **Graceful Shutdown**: Send `SIGINT` or `SIGTERM` to instruct the gateway to finish existing requests, cleanup the PID file, and shutdown safely:
  ```bash
  sudo kill -TERM $(cat /var/run/gatekeeper.pid)
  ```
