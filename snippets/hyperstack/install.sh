#!/bin/bash
set -e

# OpenCode + Codex + Ollama Installation Script
# Usage: ./install.sh [user@host] [opencode_version]
# Example: ./install.sh ubuntu@38.128.233.181 v1.1.32

HOST="${1:-ubuntu@38.128.233.181}"
OPENCODE_VERSION="${2:-v1.1.32}"
CODEX_VERSION="${3:-latest}"

echo "Installing OpenCode, Codex, and Ollama on $HOST"
echo "OpenCode version: $OPENCODE_VERSION"
echo "Codex version: $CODEX_VERSION"

# Function to run remote commands
run_remote() {
    ssh "$HOST" "$1"
}

# 1. Install Ollama
echo "Step 1: Installing Ollama..."
run_remote "curl -fsSL https://ollama.ai/install.sh | sh" || echo "Ollama may already be installed"

# 2. Start Ollama service
echo "Step 2: Starting Ollama service..."
run_remote "sudo systemctl start ollama && sudo systemctl status ollama" || echo "Starting Ollama..."

# 2.5. Configure Ollama to use /ephemeral for models (if available)
echo "Step 2.5: Configuring Ollama models directory..."
run_remote "if [ -d /ephemeral ]; then
  sudo mkdir -p /ephemeral/ollama/models
  sudo chown ollama:ollama /ephemeral/ollama /ephemeral/ollama/models
  sudo mkdir -p /etc/systemd/system/ollama.service.d
  sudo tee /etc/systemd/system/ollama.service.d/override.conf > /dev/null << 'EOF'
[Service]
Environment=\"OLLAMA_MODELS=/ephemeral/ollama/models\"
Environment=\"OLLAMA_GPU_OVERHEAD=2000\"
Environment=\"OLLAMA_NUM_PARALLEL=4\"
EOF
  sudo systemctl daemon-reload
  sudo systemctl restart ollama
  sleep 3
  echo 'Ollama configured to use /ephemeral'
else
  echo 'No /ephemeral directory found, using default'
fi"

# 3. Kill unattended upgrade lock if it exists
echo "Step 3: Clearing package manager lock..."
run_remote "sudo pkill -f unattended-upgrade || true"
sleep 2

# 4. Install Node.js 20 (required for Codex), npm, and Python
echo "Step 4: Installing Node.js 20, npm, and Python..."
run_remote "curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash - && sudo apt install -y nodejs python3 python3-pip"

# 5. Download and install OpenCode CLI
echo "Step 5: Installing OpenCode CLI ($OPENCODE_VERSION)..."
run_remote "cd /tmp && wget -q https://github.com/anomalyco/opencode/releases/download/$OPENCODE_VERSION/opencode-linux-x64.tar.gz && tar -xzf opencode-linux-x64.tar.gz && sudo mv opencode /usr/local/bin/ && opencode --version"

# 6. Create OpenCode config directory
echo "Step 6: Creating OpenCode config directory..."
run_remote "mkdir -p ~/.config/opencode"

# 7. Create OpenCode configuration (qwen3-coder models)
echo "Step 7: Creating OpenCode configuration..."
run_remote "cat > ~/.config/opencode/opencode.json << 'EOF'
{
  \"provider\": {
    \"ollama\": {
      \"npm\": \"@ai-sdk/openai-compatible\",
      \"name\": \"Ollama\",
      \"api\": \"http://localhost:11434/v1\",
      \"models\": {
        \"qwen3-coder:latest\": {
          \"name\": \"Qwen3 Coder Latest (Higher Quality)\"
        },
        \"qwen3-coder:30b-a3b-q4_K_M\": {
          \"name\": \"Qwen3 Coder 30B A3B Q4_K_M (Quantized)\"
        }
      }
    }
  }
}
EOF"

# 8. Pull qwen3-coder models
echo "Step 8: Pulling qwen3-coder models (this may take a while)..."
run_remote "ollama pull qwen3-coder:latest"
run_remote "ollama pull qwen3-coder:30b-a3b-q4_K_M"

# 9. Install Codex CLI
echo "Step 9: Installing Codex CLI..."
run_remote "sudo npm install -g @openai/codex || npm install -g @openai/codex"

# 10. Create Codex config to use Ollama
echo "Step 10: Configuring Codex to use Ollama..."
run_remote "mkdir -p ~/.codex && cat > ~/.codex/config.json << 'EOF'
{
  \"provider\": \"ollama\",
  \"model\": \"qwen3-coder:30b-a3b-q4_K_M\",
  \"endpoint\": \"http://localhost:11434/v1\"
}
EOF"

# 11. Install Aider (LLM-powered code editing)
echo "Step 11: Installing Aider..."
run_remote "pip3 install --user aider-chat && echo 'export PATH=~/.local/bin:\$PATH' >> ~/.bashrc"

# 12. Configure Aider to use Ollama
echo "Step 12: Configuring Aider for Ollama..."
run_remote "mkdir -p ~/.aider && cat > ~/.aider/.aider.conf.yml << 'EOF'
model: ollama/qwen2.5-coder:14b-instruct
openai-api-base: http://localhost:11434/v1
EOF"

# 13. Verify installation
echo "Step 13: Verifying installation..."
run_remote "opencode --version && ollama list && codex --version && ~/.local/bin/aider --version"

echo ""
echo "✓ Installation complete!"
echo ""
echo "To use OpenCode:"
echo "  ssh $HOST"
echo "  cd ~/your/project"
echo "  opencode"
echo ""
echo "To use Codex:"
echo "  ssh $HOST"
echo "  codex --oss --local-provider ollama"
echo ""
echo "To use Aider (LLM-powered code editing):"
echo "  ssh $HOST"
echo "  cd ~/your/project"
echo "  aider"
echo "  aider <filename>"
echo ""
echo "Available models for all tools:"
echo "  - qwen2.5-coder:14b-instruct (fast, ~9GB) - default"
echo "  - qwen3-coder:30b-a3b-q4_K_M (slower, ~18GB)"
