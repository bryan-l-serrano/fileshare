# FileShare

A Go-based file sharing server with a simple file browser frontend. Upload files, create folders, and drag-and-drop to organize.

## Quick Start

```bash
go mod download
go run .
```

Visit `http://localhost:8080/files` in your browser.

## Configuration

Edit `config.yaml`:

```yaml
port: 8080
address: 127.0.0.1

mount_points:
  - path: ./uploads
    label: "Uploads"
    max_size_mb: 100
```

- `port` / `address`: Where the server listens.
- `mount_points`: Directories where files are stored. Multiple mount points are supported (only the first is used for uploads).
- `max_size_mb`: Maximum upload size per file (default 100 MB).

## API Endpoints

| Method | Path           | Description            |
|--------|----------------|------------------------|
| GET    | `/files`       | File browser page      |
| GET    | `/api/files`   | List files/folders     |
| POST   | `/api/upload`  | Upload a file          |
| POST   | `/api/mkdir`   | Create a folder        |
| POST   | `/api/move`    | Move a file            |
| POST   | `/api/delete`  | Delete a file          |
| POST   | `/api/rmdir`   | Delete a folder        |
| GET    | `/download/`   | Download a file        |

## Project Structure

```
config.yaml       # Server configuration
config.go         # Config loader
handler.go        # HTTP handlers
main.go           # Entry point
frontend/         # Frontend project
  package.json    # NPM manifest
  public/
    files.html    # File browser page
    css/style.css # Shared styles
    js/files.js   # File browser logic
```
