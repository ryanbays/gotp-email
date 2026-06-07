# Variables
APP_NAME := "gotp-email"
APP_DIR := "/opt/" + APP_NAME
BIN := APP_DIR + "/build"
SERVICE_FILE := "/etc/systemd/system/" + APP_NAME + ".service"
CACHE_DIR := APP_DIR + "/cache" 
CONFIG_DIR := "/etc/" + APP_NAME

# Default task
default:
    @just --list

# Build binary
build:
    @echo "Building binary..."
    
    cd src && go mod tidy && sudo go build -o {{BIN}} .
    @cd ..

# Create system user (safe no-op if exists)
user:
    @id otp >/dev/null 2>&1 || sudo useradd -r -s /bin/false otp

# Create directories
dirs:
    @echo "Creating directories..."
    sudo mkdir -p {{APP_DIR}}
    sudo mkdir -p {{CONFIG_DIR}}
    sudo mkdir -p {{CACHE_DIR}}

    sudo chown -R otp:otp {{APP_DIR}}
    sudo chown -R otp:otp {{CACHE_DIR}}

# Install systemd service (symlink style)
service:
    @echo "Installing systemd service..."
    sudo mkdir -p /etc/systemd/system

    sudo rm -f {{SERVICE_FILE}}
    sudo ln -s $(pwd)/service.example {{SERVICE_FILE}}

    sudo systemctl daemon-reload
    sudo systemctl enable {{APP_NAME}}

# Installs config files with overwrite confirmation
config:
    @if [ -f {{CONFIG_DIR}}/rules.json ] || [ -f {{CONFIG_DIR}}/gotp.conf ]; then \
        read -p "Config files already exist. Do you want to overwrite them? (y/n) " answer; \
        if [ "$$answer" == "y" ]; then \
            just config-install \
        else \
            echo "Skipping config installation." \
        fi; \
    fi


# Installs config files without confirmation (used by config task)
config-install:
    @echo "Copying config files to {{CONFIG_DIR}}..."
    sudo rm -f {{CONFIG_DIR}}/rules.json
    sudo rm -f {{CONFIG_DIR}}/gotp.conf

    sudo cp $(pwd)/config/rules.json {{CONFIG_DIR}}/rules.json
    sudo cp $(pwd)/config/gotp.conf {{CONFIG_DIR}}/gotp.conf

    sudo chmod -R go+r {{CONFIG_DIR}}



# Restart service
restart:
    @echo "Restarting service..."
    sudo systemctl restart {{APP_NAME}}

# Check status
status:
    sudo systemctl status {{APP_NAME}} --no-pager

# Full install pipeline
install: user dirs build service config restart
    @echo "Installation complete"

# Logs
logs:
    journalctl -u {{APP_NAME}} -f
