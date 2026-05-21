#!/usr/bin/env bash
set -euo pipefail

# ANSI color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging
LOG_FILE="/var/log/loom-install.log"
NON_INTERACTIVE=false
SKIP_BUILD=false
PORT="8080"
COMMIT=""
DB_PATH="/opt/loom/data/loom.db"
REPO_URL="https://github.com/ubenmackin/loom.git"
BRANCH="main"
TEMP_DIR=""

# ANSI logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') $*" | tee -a "$LOG_FILE"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $(date '+%Y-%m-%d %H:%M:%S') $*" | tee -a "$LOG_FILE"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $(date '+%Y-%m-%d %H:%M:%S') $*" | tee -a "$LOG_FILE"; }
log_error() { echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') $*" | tee -a "$LOG_FILE" >&2; }

# safe_rm - validate path before removing
safe_rm() {
    local path="$1"
    # Resolve symlinks to get the real path before validation
    local real_path
    real_path=$(readlink -f "$path" 2>/dev/null || echo "$path")
    if [[ "$real_path" != /opt/loom* ]]; then
        log_error "safe_rm: refusing to remove path outside /opt/loom: $path (resolved: $real_path)"
        return 1
    fi
    rm -rf "$path"
}

# Cleanup trap
cleanup() {
    if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
        safe_rm "$TEMP_DIR" 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Prompt functions
prompt_yes_no() {
    local prompt="$1"
    local default="${2:-Y}"
    if [[ "$NON_INTERACTIVE" == true ]]; then
        [[ "$default" == "Y" ]] && return 0 || return 1
    fi
    local answer
    read -rp "$prompt [$default] " answer
    answer="${answer:-$default}"
    [[ "$answer" =~ ^[Yy] ]] && return 0 || return 1
}

prompt_with_default() {
    local prompt="$1"
    local default="$2"
    if [[ "$NON_INTERACTIVE" == true ]]; then
        echo "$default"
        return 0
    fi
    local answer
    read -rp "$prompt [$default] " answer
    echo "${answer:-$default}"
}

# Argument parsing
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-build) SKIP_BUILD=true; shift ;;
            --non-interactive) NON_INTERACTIVE=true; shift ;;
            --port) PORT="$2"; shift 2 ;;
            --commit) COMMIT="$2"; shift 2 ;;
            --db-path) DB_PATH="$2"; shift 2 ;;
            --branch) BRANCH="$2"; shift 2 ;;
            -h|--help) show_help; exit 0 ;;
            *) log_error "Unknown argument: $1"; show_help; exit 1 ;;
        esac
    done
}

show_help() {
    cat <<EOF
Usage: install-server.sh [OPTIONS]

Options:
  --skip-build        Skip building from source (use pre-built binary)
  --non-interactive   Skip all prompts, use defaults
  --port PORT         Server port (default: 8080)
  --commit SHA        Git commit to check out (default: latest on branch)
  --db-path PATH      Database file path (default: /opt/loom/data/loom.db)
  --branch BRANCH     Git branch to clone (default: main)
  -h, --help          Show this help message
EOF
}

# OS detection
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        local id="$ID"
        local id_like="${ID_LIKE:-}"

        # Normalize to debian or suse families
        case "$id" in
            debian|ubuntu|linuxmint|pop) OS_FAMILY="debian" ;;
            opensuse*|sles|suse) OS_FAMILY="suse" ;;
            *)
                # Check ID_LIKE
                case "$id_like" in
                    *debian*) OS_FAMILY="debian" ;;
                    *suse*|*opensuse*) OS_FAMILY="suse" ;;
                    *) OS_FAMILY="unknown" ;;
                esac
                ;;
        esac
    else
        OS_FAMILY="unknown"
    fi

    if [[ "$OS_FAMILY" == "unknown" ]]; then
        log_error "Unsupported OS. Loom installer supports Debian/Ubuntu and openSUSE/SLES families."
        exit 1
    fi

    log_info "Detected OS family: $OS_FAMILY"
}

# Dependency installation
install_dependencies() {
    log_info "Installing system dependencies..."

    case "$OS_FAMILY" in
        debian)
            apt-get update -y
            apt-get install -y curl git build-essential sqlite3

            # Install Go 1.23+ from official source
            if ! command -v go &>/dev/null || [[ "$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')" < "1.25" ]]; then
                log_info "Installing Go 1.25+..."
                local go_version="1.25.8"
                local go_url="https://go.dev/dl/go${go_version}.linux-amd64.tar.gz"
                local go_sha256="ceb5e041bbc3893846bd1614d76cb4681c91dadee579426cf21a63f2d7e03be6"
                local go_tarball
                go_tarball=$(mktemp /tmp/go-${go_version}.XXXXXX.tar.gz)
                curl -fsSL "$go_url" -o "$go_tarball"
                local actual_sha
                actual_sha=$(sha256sum "$go_tarball" | awk '{print $1}')
                if [[ "$actual_sha" != "$go_sha256" ]]; then
                    log_error "Go binary checksum mismatch! Expected: $go_sha256, Got: $actual_sha"
                    rm -f "$go_tarball"
                    exit 1
                fi
                rm -rf /usr/local/go
                tar -C /usr/local -xz -f "$go_tarball"
                rm -f "$go_tarball"
                # Ensure /usr/local/go/bin is in PATH
                if ! grep -q "/usr/local/go/bin" /etc/profile.d/go.sh 2>/dev/null; then
                    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
                fi
                export PATH=$PATH:/usr/local/go/bin
            fi

            # Install Node.js 20.x from NodeSource
            if ! command -v node &>/dev/null || [[ "$(node -v | grep -oP '\K[0-9]+')" -lt 20 ]]; then
                log_info "Installing Node.js 20.x..."
                local setup_script
                setup_script=$(mktemp /tmp/nodesource-setup.XXXXXX.sh)
                curl -fsSL https://deb.nodesource.com/setup_20.x -o "$setup_script"
                if [[ -s "$setup_script" ]]; then
                    bash "$setup_script"
                else
                    log_error "NodeSource setup script is empty or failed to download"
                    rm -f "$setup_script"
                    exit 1
                fi
                rm -f "$setup_script"
                apt-get install -y nodejs
            fi
            ;;
        suse)
            zypper refresh
            zypper install -y curl git gcc-c++ make sqlite3

            # Install Go
            if ! command -v go &>/dev/null || [[ "$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')" < "1.25" ]]; then
                log_info "Installing Go 1.25+..."
                local go_version="1.25.8"
                local go_url="https://go.dev/dl/go${go_version}.linux-amd64.tar.gz"
                local go_sha256="ceb5e041bbc3893846bd1614d76cb4681c91dadee579426cf21a63f2d7e03be6"
                local go_tarball
                go_tarball=$(mktemp /tmp/go-${go_version}.XXXXXX.tar.gz)
                curl -fsSL "$go_url" -o "$go_tarball"
                local actual_sha
                actual_sha=$(sha256sum "$go_tarball" | awk '{print $1}')
                if [[ "$actual_sha" != "$go_sha256" ]]; then
                    log_error "Go binary checksum mismatch! Expected: $go_sha256, Got: $actual_sha"
                    rm -f "$go_tarball"
                    exit 1
                fi
                rm -rf /usr/local/go
                tar -C /usr/local -xz -f "$go_tarball"
                rm -f "$go_tarball"
                if ! grep -q "/usr/local/go/bin" /etc/profile.d/go.sh 2>/dev/null; then
                    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
                fi
                export PATH=$PATH:/usr/local/go/bin
            fi

            # Install Node.js
            if ! command -v node &>/dev/null || [[ "$(node -v | grep -oP '\K[0-9]+')" -lt 20 ]]; then
                log_info "Installing Node.js 20.x..."
                local setup_script
                setup_script=$(mktemp /tmp/nodesource-setup.XXXXXX.sh)
                curl -fsSL https://rpm.nodesource.com/setup_20.x -o "$setup_script"
                if [[ -s "$setup_script" ]]; then
                    bash "$setup_script"
                else
                    log_error "NodeSource setup script is empty or failed to download"
                    rm -f "$setup_script"
                    exit 1
                fi
                rm -f "$setup_script"
                zypper install -y nodejs
            fi
            ;;
    esac

    log_success "Dependencies installed"
}

# Directory setup
setup_directories() {
    log_info "Setting up directories..."
    mkdir -p /opt/loom/data
    mkdir -p /opt/loom/dist
    mkdir -p /opt/loom/web
    mkdir -p /opt/loom/config
    log_success "Directories created at /opt/loom"
}

# Git clone
clone_repo() {
    log_info "Cloning repository..."

    if [[ -d /opt/loom/.git ]]; then
        log_info "Repository already exists, pulling latest..."
        cd /opt/loom
        git fetch origin
        if [[ -n "$COMMIT" ]]; then
            git checkout "$COMMIT"
        else
            git checkout "$BRANCH"
            git pull origin "$BRANCH"
        fi
    else
        if [[ -n "$COMMIT" ]]; then
            # Full clone for specific commit
            git clone "$REPO_URL" /opt/loom
            cd /opt/loom
            git checkout "$COMMIT"
        else
            # Shallow clone for latest
            git clone --depth 1 --branch "$BRANCH" "$REPO_URL" /opt/loom
        fi
    fi

    log_success "Repository ready"
}

# Build
build_project() {
    if [[ "$SKIP_BUILD" == true ]]; then
        log_info "Skipping build (--skip-build)"
        return 0
    fi

    log_info "Building Loom server..."
    cd /opt/loom

    # Go build with atomic replace
    log_info "Building Go binary..."
    export PATH=$PATH:/usr/local/go/bin
    go mod download
    make build

    # Atomic binary replace
    local temp_bin
    temp_bin=$(mktemp /opt/loom/dist/loom-server.XXXXXX)
    cp dist/loom-server "$temp_bin"
    mv "$temp_bin" /opt/loom/dist/loom-server
    chmod +x /opt/loom/dist/loom-server

    # Frontend build
    log_info "Building frontend..."
    cd /opt/loom/web
    npm install
    npm run build

    # Copy web dist
    rm -rf /opt/loom/web/dist
    cp -r dist /opt/loom/web/dist

    log_success "Build complete"
}

# System user
create_system_user() {
    log_info "Creating system user 'loom'..."

    if id loom &>/dev/null; then
        log_info "User 'loom' already exists"
    else
        useradd --system --no-create-home --shell /usr/sbin/nologin loom
        log_success "User 'loom' created"
    fi

    chown -R loom:loom /opt/loom
    log_success "Ownership set"
}

# Database init
init_database() {
    log_info "Initializing database..."

    # Run server briefly to trigger migrations and seed defaults
    timeout 5 /opt/loom/dist/loom-server --db-path "$DB_PATH" --port 0 --web-dir /opt/loom/web/dist 2>&1 | tee -a "$LOG_FILE" || true

    # Verify DB file was created
    if [[ -f "$DB_PATH" ]]; then
        log_success "Database initialized at $DB_PATH"
    else
        log_warn "Database file not found at $DB_PATH — will be created on first real start"
    fi
}

# Systemd service
install_systemd_service() {
    log_info "Installing systemd service..."

    cat > /etc/systemd/system/loom-server.service <<'SERVICEEOF'
[Unit]
Description=Loom — Agent-First JIT Kanban Board
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=loom
Group=loom
WorkingDirectory=/opt/loom
ExecStart=/opt/loom/dist/loom-server --db-path /opt/loom/data/loom.db --port 8080 --web-dir /opt/loom/web/dist
Restart=on-failure
RestartSec=10s
Environment=LOOM_DB_PATH=/opt/loom/data/loom.db
Environment=LOOM_PORT=8080
Environment=LOOM_WEB_DIR=/opt/loom/web/dist

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/opt/loom/data
ReadOnlyPaths=/opt/loom/web /opt/loom/dist

# Resource limits
LimitNOFILE=65536
MemoryMax=512M

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=loom-server

[Install]
WantedBy=multi-user.target
SERVICEEOF

    # Replace port in service file if non-default
    if [[ "$PORT" != "8080" ]]; then
        sed -i "s/--port 8080/--port $PORT/g" /etc/systemd/system/loom-server.service
        sed -i "s/LOOM_PORT=8080/LOOM_PORT=$PORT/g" /etc/systemd/system/loom-server.service
    fi

    # Replace db path if non-default
    if [[ "$DB_PATH" != "/opt/loom/data/loom.db" ]]; then
        sed -i "s|/opt/loom/data/loom.db|$DB_PATH|g" /etc/systemd/system/loom-server.service
    fi

    systemctl daemon-reload
    systemctl enable loom-server.service

    log_success "Systemd service installed and enabled"
}

# Service start
start_service() {
    log_info "Starting Loom server..."
    systemctl start loom-server.service
    sleep 2

    if systemctl is-active --quiet loom-server.service; then
        log_success "Loom server is running"
    else
        log_warn "Loom server may not have started. Check with: systemctl status loom-server"
    fi
}

# Installation summary
show_summary() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  Loom Installation Complete!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "  Binary:    /opt/loom/dist/loom-server"
    echo "  Web:       /opt/loom/web/dist"
    echo "  Database:  $DB_PATH"
    echo "  Config:    /opt/loom/config"
    echo "  Port:      $PORT"
    echo "  Service:   loom-server.service"
    echo ""
    echo "  Useful commands:"
    echo "    systemctl status loom-server    # Check service status"
    echo "    systemctl restart loom-server   # Restart service"
    echo "    journalctl -u loom-server -f    # View logs"
    echo "    curl http://localhost:$PORT/api/board  # Test API"
    echo ""
    echo "  Access the board at: http://<server-ip>:$PORT"
    echo ""
}

# Main
main() {
    parse_args "$@"

    log_info "Starting Loom installation..."

    detect_os
    install_dependencies
    setup_directories
    clone_repo
    build_project
    create_system_user
    init_database
    install_systemd_service
    start_service
    show_summary

    log_success "Installation complete!"
}

main "$@"
