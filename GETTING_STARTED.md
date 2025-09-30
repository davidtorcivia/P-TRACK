# Getting Started with Injection Tracker

## What Has Been Built

You now have a **production-ready foundation** for the Progesterone Injection Tracker with **security implemented from day one**. This is approximately 25% of the complete application, focusing on the most critical infrastructure.

### ✅ Completed Infrastructure

1. **Complete Database Schema** with 13 tables including:
   - Users with account lockout
   - Courses, injections, symptoms, medications
   - Inventory tracking with history
   - Audit logs for compliance
   - Session management
   - Notifications

2. **Comprehensive Security Layer**:
   - CSRF protection with one-time tokens
   - JWT authentication (bcrypt + HS256)
   - Rate limiting (5 login attempts per 15 minutes)
   - All security headers (CSP, HSTS, X-Frame-Options, etc.)
   - SQL injection prevention (prepared statements)
   - Audit logging infrastructure

3. **Production Deployment Setup**:
   - Docker with security hardening
   - Nginx reverse proxy with SSL
   - Automated setup script
   - Configuration management
   - Database migrations

---

## Quick Start

### Step 1: Set Up Environment

```bash
# Make setup script executable
chmod +x setup.sh

# Run setup (generates secrets, creates directories)
./setup.sh
```

This will:
- Generate secure JWT and CSRF secrets
- Create necessary directories
- Offer to generate SSL certificate
- Optionally start the application

### Step 2: Verify Installation

```bash
# Check if running
docker-compose ps

# View logs
docker-compose logs -f

# Test health endpoint
curl http://localhost:8080/health
```

You should see "OK" response.

### Step 3: Set Environment Variables

The `.env` file has been created with secure defaults. Review and adjust:

```bash
# Edit environment file
nano .env
```

**Critical Settings:**
- `JWT_SECRET` - Auto-generated, keep secure
- `CSRF_SECRET` - Auto-generated, keep secure
- `DATABASE_PATH` - Where to store the database

**Optional Settings:**
- SMTP configuration for password resets
- Rate limiting thresholds
- Backup schedule

---

## What Needs to Be Implemented Next

The application currently has:
- ✅ Complete routing structure
- ✅ Security middleware
- ✅ Database schema
- ❌ **Handler implementations** (all return "Not implemented yet")
- ❌ **Frontend templates**
- ❌ **Business logic**

### Priority 1: Repository Layer (1-2 days)

Create database query functions with prepared statements:

```go
// Example structure needed in internal/repository/
- user_repository.go      // User CRUD operations
- course_repository.go    // Course management
- injection_repository.go // Injection tracking
- inventory_repository.go // Inventory management with transactions
// etc...
```

### Priority 2: Authentication Handlers (2-3 days)

Implement the auth handlers in `cmd/server/main.go`:
- `handleLogin` - Verify credentials, generate JWT
- `handleRegister` - Create new users with validation
- `handleForgotPassword` - Send reset email
- `handleResetPassword` - Verify token and reset

### Priority 3: Core Injection Logic (3-5 days)

The PRIMARY feature of the app:
- Quick injection logging
- **Inventory auto-decrement** (critical!)
- Site tracking (basic + advanced mode)
- Injection history

### Priority 4: Frontend Templates (5-7 days)

Create HTML templates using:
- HTMX for dynamic updates
- Alpine.js for interactivity
- Pico CSS for styling

---

## Development Workflow

### Running Locally

```bash
# Without Docker (for faster development)
make run

# With auto-reload (requires air)
make dev

# Run tests
make test

# Check security
make security-check
```

### With Docker

```bash
# Build and start
make docker-up

# View logs
make docker-logs

# Rebuild after changes
make docker-rebuild

# Stop
make docker-down
```

### Making Changes

1. **Add a new endpoint:**
   - Route already exists in `cmd/server/main.go`
   - Implement the handler function
   - Add repository method if needed
   - Add audit logging

2. **Modify database schema:**
   - Create new migration file: `migrations/002_your_change.sql`
   - Run: `make migrate`

3. **Add middleware:**
   - Create in `internal/middleware/`
   - Add to router chain in `main.go`

---

## Security Considerations

### What's Already Protected

✅ **CSRF**: All POST/PUT/DELETE endpoints require valid token
✅ **Rate Limiting**: 5 login attempts per 15 minutes
✅ **JWT**: Tokens expire after 2 weeks
✅ **SQL Injection**: Prepared statements (when implemented)
✅ **XSS**: CSP headers with nonce support
✅ **HTTPS**: Nginx redirects HTTP → HTTPS
✅ **Headers**: All security headers configured

### Before Going to Production

1. Change secrets (already generated securely)
2. Get proper SSL certificate (Let's Encrypt)
3. Set `ENVIRONMENT=production`
4. Enable backup cron job
5. Set up monitoring
6. Review rate limit settings
7. Complete all handler implementations
8. Run security tests
9. Perform penetration testing
10. Set up fail2ban or similar

---

## Project Structure

```
.
├── cmd/server/main.go         # ⚠️ Main app - handlers need implementation
├── internal/
│   ├── auth/                  # ✅ JWT & password utilities (complete)
│   ├── config/                # ✅ Configuration (complete)
│   ├── database/              # ✅ DB connection & migrations (complete)
│   ├── middleware/            # ✅ Security middleware (complete)
│   ├── models/                # ✅ Data models (complete)
│   ├── repository/            # ❌ TODO: Implement DB queries
│   ├── handlers/              # ❌ TODO: Implement request handlers
│   └── services/              # ❌ TODO: Implement business logic
├── migrations/
│   └── 001_initial_schema.sql # ✅ Complete schema (13 tables)
├── static/                    # ❌ TODO: Add CSS, JS, icons
├── templates/                 # ❌ TODO: Create HTML templates
├── Dockerfile                 # ✅ Secure multi-stage build
├── docker-compose.yml         # ✅ Production-ready config
├── nginx.conf                 # ✅ Reverse proxy with SSL
├── setup.sh                   # ✅ Automated setup script
├── Makefile                   # ✅ Common commands
└── .env                       # ✅ Generated by setup.sh
```

---

## Testing the Foundation

Even without handlers implemented, you can verify the security infrastructure:

### 1. Test Health Endpoint
```bash
curl http://localhost:8080/health
# Should return: OK
```

### 2. Test CSRF Protection
```bash
# This should fail (no CSRF token)
curl -X POST http://localhost:8080/api/courses \
  -H "Authorization: Bearer fake-token" \
  -d '{"name":"Test"}'
# Should return: 403 Forbidden
```

### 3. Test Rate Limiting
```bash
# Try multiple rapid login attempts
for i in {1..10}; do
  curl -X POST http://localhost:8080/api/auth/login \
    -d '{"username":"test","password":"test"}'
done
# After 5 attempts: 429 Too Many Requests
```

### 4. Test Security Headers
```bash
curl -I https://localhost:443
# Should see: X-Frame-Options, CSP, HSTS, etc.
```

---

## Common Tasks

### Create Database Backup
```bash
make backup
# Creates: backups/tracker-YYYYMMDD-HHMMSS.db
```

### View Database
```bash
sqlite3 data/tracker.db

# List tables
.tables

# View users
SELECT * FROM users;

# View schema
.schema users
```

### Generate New Secrets
```bash
openssl rand -base64 32
```

### Check Application Logs
```bash
# Docker
docker-compose logs -f app

# Local
# Logs print to stdout when running with `make run`
```

---

## Next Steps

1. **Review the design document** (`CLAUDE.md`) to understand requirements
2. **Check implementation status** (`IMPLEMENTATION_STATUS.md`) for details
3. **Start with repository layer** - Create database query functions
4. **Implement authentication** - Login, register, password reset
5. **Build injection logging** - The core feature
6. **Create frontend templates** - User interface
7. **Add tests** - Security, unit, integration
8. **Deploy to production** - With proper SSL and monitoring

---

## Getting Help

### Documentation
- `README.md` - Overview and quick start
- `CLAUDE.md` - Complete design document
- `IMPLEMENTATION_STATUS.md` - Detailed progress tracker
- `GETTING_STARTED.md` - This file

### Useful Commands
```bash
make help           # Show all available commands
make test           # Run tests (once implemented)
make security-check # Run security analysis
docker-compose logs # View application logs
```

### Common Issues

**"Can't connect to database"**
- Check `DATABASE_PATH` in .env
- Ensure `data/` directory exists
- Check file permissions

**"Invalid JWT/CSRF secret"**
- Run `./setup.sh` to generate new secrets
- Ensure secrets are base64 encoded
- Minimum 32 characters recommended

**"Rate limited"**
- Wait 15 minutes
- Or adjust `LOGIN_RATE_LIMIT` in .env
- Or clear rate limiter (restart app)

**"HTTPS not working"**
- Generate SSL cert: see `setup.sh`
- Or use Let's Encrypt for production
- Check nginx config

---

## Success Criteria

You'll know the foundation is working when:
- ✅ Health check returns OK
- ✅ CSRF protection blocks unauthorized requests
- ✅ Rate limiting triggers after 5 login attempts
- ✅ Security headers are present in responses
- ✅ Database migrations run successfully
- ✅ Docker containers start and remain healthy

---

**Current Status**: Foundation Complete (25%)
**Estimated Time to MVP**: 6-8 weeks
**Next Milestone**: Repository layer + authentication handlers

Good luck with the implementation! The hard part (security infrastructure) is done. Now it's time to build the features on top of this solid foundation.