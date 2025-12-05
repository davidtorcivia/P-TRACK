# Progesterone Injection Tracker

A secure, mobile-first web application for tracking progesterone injections, medication inventory, and related symptoms. Designed for family use with shared patient data and multi-user access.

## Features

### Core Features
- **Quick Injection Logging**: Log injections with just 2 taps
- **Advanced Site Tracking**: Visual heat map to track injection sites
- **Inventory Management**: Automatic inventory tracking with low-stock alerts
- **Symptom Tracking**: Monitor pain, symptoms, and reactions
- **Medication Management**: Track pills and supplements
- **Data Visualization**: Calendar views and charts
- **PWA Support**: Install as a mobile app
- **Multi-User**: Family members can share access

### Security Features
- **JWT Authentication**: Secure session management with 2-week expiry
- **Password Security**: bcrypt hashing with cost factor 12
- **CSRF Protection**: Protection against cross-site request forgery
- **Rate Limiting**: Prevents brute force attacks
- **Security Headers**: CSP, HSTS, X-Frame-Options, etc.
- **Audit Logging**: All actions logged for accountability
- **Input Validation**: All user input sanitized
- **SQL Injection Prevention**: Prepared statements only

## Quick Start

### Prerequisites
- Docker and Docker Compose
- OR Go 1.21+ and SQLite

### Option 1: Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/davidtorcivia/P-TRACK.git
cd P-TRACK

# Run setup script
chmod +x setup.sh
./setup.sh

# The application will be available at:
# http://localhost:8080
```

### Option 2: Local Development

```bash
# Install dependencies
go mod download

# Set up environment
cp .env.example .env
# Edit .env and add secure secrets:
# JWT_SECRET=$(openssl rand -base64 32)
# CSRF_SECRET=$(openssl rand -base64 32)

# Run migrations and start server
make run

# Or use auto-reload during development
make dev
```

## Configuration

All configuration is done via environment variables. See `.env.example` for available options.

### Required Configuration
- `JWT_SECRET`: Secret key for JWT signing (generate with `openssl rand -base64 32`)
- `CSRF_SECRET`: Secret key for CSRF protection (generate with `openssl rand -base64 32`)

### Optional Configuration
- `PORT`: Server port (default: 8080)
- `DATABASE_PATH`: SQLite database path (default: ./data/tracker.db)
- `SESSION_DURATION`: JWT token expiry (default: 336h = 2 weeks)
- `RATE_LIMIT_REQUESTS`: Max requests per window (default: 100)
- `LOGIN_RATE_LIMIT`: Max login attempts per window (default: 5)
- `SMTP_*`: Email configuration for password resets

## Security

### Best Practices
1. **Password Requirements**: Minimum 8 characters
2. **Rate Limiting**: Login attempts limited to 5 per 15 minutes
3. **Session Management**: Tokens expire after 2 weeks
4. **HTTPS Only**: HTTP redirects to HTTPS (in production)
5. **Secure Headers**: CSP, HSTS, X-Frame-Options, etc.
6. **CSRF Protection**: All state-changing operations protected
7. **Audit Logging**: All actions logged with user, IP, and timestamp
8. **Input Sanitization**: All user input validated and sanitized
9. **SQL Injection Prevention**: Prepared statements only
10. **XSS Prevention**: Content Security Policy enforced

### Production Deployment
For production deployment:
1. Use strong, randomly generated secrets
2. Enable HTTPS with proper SSL certificates (Let's Encrypt)
3. Set `ENVIRONMENT=production` in .env
4. Configure firewall rules
5. Enable database backups
6. Monitor logs for security events
7. Keep dependencies updated

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Browser/PWA                   │
│  ┌────────────┐  ┌──────────┐  ┌──────────────┐ │
│  │HTMX+Alpine │  │ Service  │  │ IndexedDB    │ │
│  │  (~30kb)   │  │ Worker   │  │ (offline)    │ │
│  └────────────┘  └──────────┘  └──────────────┘ │
└─────────────────────────────────────────────────┘
                      ↓ HTTPS/JWT
┌─────────────────────────────────────────────────┐
│              Docker Container                   │
│  ┌────────────────────────────────────────────┐ │
│  │         Nginx (Reverse Proxy)              │ │
│  │  - Rate Limiting                           │ │
│  │  - SSL Termination                         │ │
│  │  - Security Headers                        │ │
│  └────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────┐ │
│  │           Go HTTP Server                   │ │
│  │  - JWT Authentication                      │ │
│  │  - CSRF Protection                         │ │
│  │  - Business Logic                          │ │
│  │  - HTMX Handlers                           │ │
│  └────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────┐ │
│  │         SQLite Database (WAL)              │ │
│  └────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

## Database

The application uses SQLite with Write-Ahead Logging (WAL) for better concurrency. The database includes:
- Users and authentication
- Courses (treatment cycles)
- Injections with site tracking
- Symptom logs
- Medications and logs
- Inventory management
- Audit logs
- Notifications

## Development

### Available Commands

```bash
make help           # Show all available commands
make build          # Build the application
make run            # Run locally
make test           # Run tests
make test-coverage  # Run tests with coverage report
make docker-build   # Build Docker image
make docker-up      # Start with Docker Compose
make docker-down    # Stop Docker containers
make lint           # Run linters
make fmt            # Format code
make security-check # Run security analysis
```

### Project Structure

```
.
├── cmd/
│   └── server/         # Main application entry point
├── internal/
│   ├── auth/           # Authentication (JWT, bcrypt)
│   ├── config/         # Configuration management
│   ├── database/       # Database connection and migrations
│   ├── handlers/       # HTTP handlers
│   ├── middleware/     # HTTP middleware (CSRF, rate limiting, etc.)
│   ├── models/         # Data models
│   ├── repository/     # Database queries
│   └── services/       # Business logic
├── migrations/         # Database migrations
├── static/            # Static assets (CSS, JS, icons)
├── templates/         # HTML templates
├── data/              # SQLite database (gitignored)
├── backups/           # Database backups (gitignored)
└── docker-compose.yml # Docker Compose configuration
```

## Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run security checks
make security-check
```

## Backup and Recovery

### Automatic Backups
Backups are created daily at 2 AM by default (configurable via `BACKUP_SCHEDULE`).

### Manual Backup
```bash
make backup
```

### Restore from Backup
```bash
cp backups/tracker-YYYYMMDD-HHMMSS.db data/tracker.db
```