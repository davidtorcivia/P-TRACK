# Injection Tracker Setup Script for Windows
# This script sets up the application with secure defaults

# Stop on errors
$ErrorActionPreference = "Stop"

Write-Host "=========================================="
Write-Host "Injection Tracker Setup"
Write-Host "=========================================="
Write-Host ""

# Check if .env already exists
if (Test-Path .env) {
    Write-Host "Warning: .env file already exists."
    $choice = Read-Host "Do you want to overwrite it? (y/N)"
    if ($choice -ne 'y') {
        Write-Host "Setup cancelled."
        exit
    }
}

# Generate secrets using OpenSSL
Write-Host "Generating secure secrets..."
$JWT_SECRET = (openssl rand -base64 32) -join ""
$CSRF_SECRET = (openssl rand -base64 32) -join ""

# Create directories
Write-Host "Creating directories..."
New-Item -ItemType Directory -Force -Path "data", "backups", "ssl", "static/css", "static/js", "static/icons", "templates/components", "templates/layouts", "templates/pages"

# Create .env file
Write-Host "Creating .env file..."
$envContent = @"
# Server Configuration
PORT=8080
ENVIRONMENT=production

# Security (Auto-generated)
JWT_SECRET=${JWT_SECRET}
CSRF_SECRET=${CSRF_SECRET}
SESSION_DURATION=336h

# Database
DATABASE_PATH=./data/tracker.db

# Rate Limiting
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1m
LOGIN_RATE_LIMIT=5
LOGIN_RATE_WINDOW=15m

# SMTP (Optional - configure if needed)
SMTP_ENABLED=false
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=your-email@gmail.com

# Backup Configuration
BACKUP_ENABLED=true
BACKUP_SCHEDULE=0 2 * * *
BACKUP_RETENTION_DAYS=30

# Security Headers
CSP_ENABLED=true
HSTS_ENABLED=true
"@
$envContent | Out-File -FilePath .env -Encoding utf8

Write-Host "✓ .env file created with secure secrets"

# Generate self-signed SSL certificate for development
Write-Host ""
$generateSsl = Read-Host "Generate self-signed SSL certificate for development? (y/N)"
if ($generateSsl -eq 'y') {
    Write-Host "Generating self-signed SSL certificate..."
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout ssl/key.pem -out ssl/cert.pem -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"
    Write-Host "✓ SSL certificate generated"
} else {
    Write-Host "⚠ SSL certificate not generated. You'll need to provide your own certificate."
}

# In Windows, file permissions are managed differently and `chmod` is not directly applicable.
# The default permissions are generally sufficient for this use case.
Write-Host ""
Write-Host "Setting file permissions is handled by Windows ACLs and is typically not required for this script."

# Build and start with Docker Compose
Write-Host ""
$startDocker = Read-Host "Build and start the application with Docker Compose? (y/N)"
if ($startDocker -eq 'y') {
    Write-Host "Building Docker image..."
    docker-compose build

    Write-Host ""
    Write-Host "Starting application..."
    docker-compose up -d

    Write-Host ""
    Write-Host "✓ Application started successfully!"
    Write-Host ""
    Write-Host "Access the application at:"
    Write-Host "  - HTTP:  http://localhost:80"
    Write-Host "  - HTTPS: https://localhost:443 (if SSL configured)"
    Write-Host "  - Direct: http://localhost:8080"
} else {
    Write-Host ""
    Write-Host "To start the application later, run:"
    Write-Host "  docker-compose up -d"
}

Write-Host ""
Write-Host "=========================================="
Write-Host "Setup Complete!"
Write-Host "=========================================="
Write-Host ""
Write-Host "Important Security Notes:"
Write-Host "1. The JWT and CSRF secrets have been auto-generated"
Write-Host "2. Keep your .env file secure and never commit it to version control"
Write-Host "3. For production, use a proper SSL certificate (Let's Encrypt)"
Write-Host "4. Configure SMTP settings if you want password reset functionality"
Write-Host "5. Review and adjust rate limiting settings as needed"
Write-Host ""
Write-Host "Next steps:"
Write-Host "1. Create your first user account via the registration page"
Write-Host "2. Configure application settings"
Write-Host "3. Set up backup strategy for the database"
Write-Host ""
Write-Host "For more information, see the CLAUDE.md design document"
Write-Host ""