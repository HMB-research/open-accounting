#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check for required commands
check_dependencies() {
    local missing=()

    for cmd in docker docker-compose curl; do
        if ! command -v $cmd &> /dev/null; then
            missing+=("$cmd")
        fi
    done

    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required commands: ${missing[*]}"
        log_info "Please install Docker and Docker Compose first."
        exit 1
    fi
}

# Generate secure random string
generate_secret() {
    openssl rand -base64 32 2>/dev/null || head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32
}

# Main installation
main() {
    log_info "Open Accounting - One-Line Installer"
    log_info "======================================"

    check_dependencies

    # Create installation directory
    INSTALL_DIR="${INSTALL_DIR:-$HOME/open-accounting}"
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"

    log_info "Installing to: $INSTALL_DIR"

    # Clone or update repository
    if [ -d ".git" ]; then
        log_info "Updating existing installation..."
        git pull
    else
        log_info "Cloning repository..."
        git clone https://github.com/openaccounting/openaccounting.git .
    fi

    # Create .env file if it doesn't exist
    if [ ! -f ".env" ]; then
        log_info "Creating environment configuration..."

        # Generate secrets
        JWT_SECRET=$(generate_secret)
        DB_PASSWORD=$(generate_secret)

        cat > .env << EOF
# Database Configuration
DB_USER=openaccounting
DB_PASSWORD=$DB_PASSWORD
DB_NAME=openaccounting
DATABASE_URL=postgres://openaccounting:$DB_PASSWORD@db:5432/openaccounting?sslmode=disable

# Security
JWT_SECRET=$JWT_SECRET

# Domains (change for production)
API_DOMAIN=api.localhost
APP_DOMAIN=app.localhost
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000

# Let's Encrypt (for production)
ACME_EMAIL=admin@example.com

# Docker
VERSION=latest
EOF

        log_warn "Environment file created with random secrets."
        log_warn "Edit .env file for production settings."
    fi

    # Build and start services
    log_info "Building Docker images..."
    docker-compose build

    log_info "Starting services..."
    docker-compose up -d

    # Wait for database to be ready
    log_info "Waiting for database to be ready..."
    sleep 5

    # Run migrations
    log_info "Running database migrations..."
    docker-compose run --rm migrate

    # Show status
    log_info ""
    log_info "======================================"
    log_info "Open Accounting installed successfully!"
    log_info "======================================"
    log_info ""
    log_info "API is running at: http://localhost:8080"
    log_info "API Health check: http://localhost:8080/health"
    log_info ""
    log_info "Useful commands:"
    log_info "  cd $INSTALL_DIR"
    log_info "  docker-compose logs -f      # View logs"
    log_info "  docker-compose ps           # Check status"
    log_info "  docker-compose down         # Stop services"
    log_info "  docker-compose up -d        # Start services"
    log_info ""
    log_info "Next steps:"
    log_info "  1. Register a user via POST /api/v1/auth/register"
    log_info "  2. Create a tenant organization"
    log_info "  3. Start using the accounting API"
    log_info ""
}

# Run main function
main "$@"
