# Development Guide

This project uses Wails with a Vite frontend and a Go backend.

## Requirements

- Go 1.26+
- Node.js 22+ (LTS recommended)
- Wails CLI v2

Install Wails if needed:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## First Run

Install frontend dependencies:

```bash
cd frontend
npm install
cd ..
```

## Start Development Mode

Run the application from the project root:

```bash
wails dev
```

This starts:

- Go backend
- Vite development server
- Native desktop window with hot reload

## Running Tests

```bash
go test ./...
```

## Common Rollup Issue (Apple Silicon)

If you see an error similar to:

```text
Cannot find module @rollup/rollup-darwin-arm64
```

remove the frontend dependencies and reinstall them:

```bash
cd frontend
rm -rf node_modules package-lock.json
npm install
cd ..
wails dev
```