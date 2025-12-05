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

### Option 1: Docker (Recommended)

The easiest way to run P-TRACK is using Docker with the official image.

1.  **Create a `docker-compose.yml` file:**

    ```yaml
    services:
      app:
        image: ghcr.io/davidtorcivia/p-track:latest
        container_name: injection-tracker
        restart: unless-stopped
        ports:
          - "8080:8080"
        volumes:
          - ./data:/app/data
          - ./backups:/app/backups
        environment:
          - PORT=8080
          - ENVIRONMENT=production
          - JWT_SECRET=${JWT_SECRET}
          - CSRF_SECRET=${CSRF_SECRET}
          - DATABASE_PATH=/app/data/tracker.db
          # Optional settings (defaults shown)
          # - SESSION_DURATION=336h
          # - RATE_LIMIT_REQUESTS=100
          # - LOGIN_RATE_LIMIT=5
          # - BACKUP_ENABLED=true
          # - BACKUP_SCHEDULE=0 2 * * *
    ```

2.  **Create a `.env` file** with your secrets:

    ```bash
    # Generate secure random strings for these:
    # openssl rand -base64 32
    JWT_SECRET=your_secure_random_jwt_secret_here
    CSRF_SECRET=your_secure_random_csrf_secret_here
    ```

3.  **Start the application:**

    ```bash
    docker-compose up -d
    ```

    The application will be available at `http://localhost:8080`.

### Option 2: Local Development

```bash
# Clone the repository
git clone https://github.com/davidtorcivia/P-TRACK.git
cd P-TRACK

# Install dependencies
go mod download

# Set up environment
cp .env.example .env
# Edit .env and add secure secrets as described above

# Run migrations and start server
make run

# Or use auto-reload during development
make dev
```

## Configuration

All configuration is done via environment variables.

### Required Configuration
- `JWT_SECRET`: Secret key for JWT signing (generate with `openssl rand -base64 32`)
- `CSRF_SECRET`: Secret key for CSRF protection (generate with `openssl rand -base64 32`)

### Optional Configuration
- `PORT`: Server port (default: 8080)
- `DATABASE_PATH`: SQLite database path (default: ./data/tracker.db)
- `SESSION_DURATION`: JWT token expiry (default: 336h = 2 weeks)
- `RATE_LIMIT_REQUESTS`: Max requests per window (default: 100)
- `LOGIN_RATE_LIMIT`: Max login attempts per window (default: 5)
- `SMTP_*`: Email configuration for password resets (see docker-compose.yml for full list)

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
2. Enable HTTPS with proper SSL certificates (e.g., using Nginx/Certbot or Caddy in front)
3. Set `ENVIRONMENT=production` in .env
4. Configure firewall rules
5. Enable database backups (enabled by default in Docker)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.