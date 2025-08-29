# FastFill Backend

A modern Go backend service for form template management.

## Architecture

```
├── cmd/server/           # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── handlers/        # HTTP handlers (controllers)
│   ├── models/gorm/     # GORM model definitions
│   ├── services/        # Business logic layer
│   ├── storage/         # Cloud storage integration
│   └── database.go      # Database initialization
├── migrations/          # Database migration documentation
└── legacy/             # Legacy configuration files
```

## 🛠️ Prerequisites

- Go 1.23.0 or higher
- MySQL 5.7+ or 8.0+
- Google Cloud Storage account (optional)

## Quick Start

1. **Clone the repository**
   ```bash
   git clone https://github.com/teruyuki/fastfill-backend.git
   cd fastfill-backend
   ```

2. **Set up environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Install dependencies**
   ```bash
   go mod tidy
   ```

4. **Run the server**
   ```bash
   go run cmd/server/main.go
   # or
   make run
   ```

The server will start on `http://localhost:8080` by default.

## 📋 API Endpoints

### Templates
- `GET /api/templates` - Get all templates
- `GET /api/templates/{id}` - Get template by ID
- `POST /api/templates` - Create new template
- `PUT /api/templates/{id}` - Update template
- `DELETE /api/templates/{id}` - Delete template

### File Upload
- `POST /api/upload/svg/{templateId}` - Upload SVG template
- `GET /api/templates/{id}/svg` - Get SVG file info

### Form Submissions
- `POST /api/forms/submit` - Submit form data
- `GET /api/forms/{id}` - Get form submission
- `PUT /api/forms/{id}` - Update form submission
- `DELETE /api/forms/{id}` - Delete form submission
- `GET /api/templates/{id}/forms` - Get submissions by template

### PDF Generation
- `POST /api/generate-pdf` - Generate PDF from template and data
- `POST /api/forms/{id}/generate-pdf` - Generate PDF from submission

### Legacy Support
- `GET /api/form-templates` - Get available form SVG templates
- `POST /api/templates/from-form-svg` - Create template from form SVG

## Development

```bash
# Run the server
make run

# Build binary
make build

# Run tests
make test

# Format code
go fmt ./...

# Run linter
golangci-lint run
```

## 📦 Deployment

### Production Build

```bash
# Build for production
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server cmd/server/main.go
```