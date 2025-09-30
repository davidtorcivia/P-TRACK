#!/bin/bash

# Injection Tracker Setup Script
# This script sets up the application with secure defaults

set -e

echo "=========================================="
echo "Injection Tracker Setup"
echo "=========================================="
echo ""

# Check if .env already exists
if [ -f .env ]; then
    echo "Warning: .env file already exists."
    read -p "Do you want to overwrite it? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Setup cancelled."
        exit 0
    fi
fi

# Generate secrets
echo "Generating secure secrets..."
JWT_SECRET=$(openssl rand -base64 32)
CSRF_SECRET=$(openssl rand -base64 32)

# Create directories
echo "Creating directories..."
mkdir -p data backups ssl static/css static/js static/icons templates/components templates/layouts templates/pages

# Create .env file
echo "Creating .env file..."
cat > .env <<EOF
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
EOF

echo "✓ .env file created with secure secrets"

# Generate self-signed SSL certificate for development
echo ""
read -p "Generate self-signed SSL certificate for development? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Generating self-signed SSL certificate..."
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout ssl/key.pem \
        -out ssl/cert.pem \
        -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"
    echo "✓ SSL certificate generated"
else
    echo "⚠ SSL certificate not generated. You'll need to provide your own certificate."
fi

# Set proper permissions
echo ""
echo "Setting file permissions..."
chmod 600 .env
chmod 755 setup.sh
[ -f ssl/key.pem ] && chmod 600 ssl/key.pem
[ -f ssl/cert.pem ] && chmod 644 ssl/cert.pem

echo "✓ Permissions set"

# Build and start with Docker Compose
echo ""
read -p "Build and start the application with Docker Compose? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Building Docker image..."
    docker-compose build

    echo ""
    echo "Starting application..."
    docker-compose up -d

    echo ""
    echo "✓ Application started successfully!"
    echo ""
    echo "Access the application at:"
    echo "  - HTTP:  http://localhost:80"
    echo "  - HTTPS: https://localhost:443 (if SSL configured)"
    echo "  - Direct: http://localhost:8080"
else
    echo ""
    echo "To start the application later, run:"
    echo "  docker-compose up -d"
fi

echo ""
echo "=========================================="
echo "Setup Complete!"
echo "=========================================="
echo ""
echo "Important Security Notes:"
echo "1. The JWT and CSRF secrets have been auto-generated"
echo "2. Keep your .env file secure and never commit it to version control"
echo "3. For production, use a proper SSL certificate (Let's Encrypt)"
echo "4. Configure SMTP settings if you want password reset functionality"
echo "5. Review and adjust rate limiting settings as needed"
echo ""
echo "Next steps:"
echo "1. Create your first user account via the registration page"
echo "2. Configure application settings"
echo "3. Set up backup strategy for the database"
echo ""
echo "For more information, see the CLAUDE.md design document"
echo ""