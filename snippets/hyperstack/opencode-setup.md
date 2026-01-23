# OpenCode Setup on Hyperstack with Ollama

This document outlines the steps to install and configure OpenCode to work with local Ollama models on a remote A100-80GB server.

## Prerequisites

- SSH access to the remote server (e.g., `ubuntu@IPHERE`)
- Ollama installed on the remote server

## Installation Steps

### 0. Install Ollama

Install Ollama on the remote server:

```bash
ssh ubuntu@IPHERE "curl -fsSL https://ollama.ai/install.sh | sh"
```

Start Ollama service:

```bash
ssh ubuntu@IPHERE "sudo systemctl start ollama && sudo systemctl status ollama"
```

### 1. Kill Unattended Upgrade Lock

If the package manager is locked by unattended-upgrades:

```bash
ssh ubuntu@IPHERE "sudo kill -9 3523 && sleep 2"
```

### 2. Verify Node.js is Installed

```bash
ssh ubuntu@IPHERE "npm --version && node --version"
```

If not installed, install Node.js:

```bash
ssh ubuntu@IPHERE "sudo apt install -y nodejs npm"
```

### 3. Download and Install OpenCode CLI

Get the latest release from GitHub releases and install to `/usr/local/bin`:

```bash
ssh ubuntu@IPHERE "cd /tmp && wget -q https://github.com/anomalyco/opencode/releases/download/v1.1.32/opencode-linux-x64.tar.gz && tar -xzf opencode-linux-x64.tar.gz && sudo mv opencode /usr/local/bin/ && opencode --version"
```

This installs OpenCode v1.1.32. Check [GitHub releases](https://github.com/anomalyco/opencode/releases) for the latest version.

### 4. Create OpenCode Configuration Directory

```bash
ssh ubuntu@IPHERE "mkdir -p ~/.config/opencode"
```

### 5. Configure OpenCode to Use Ollama

Create `~/.config/opencode/opencode.json` with Ollama as the provider:

```bash
ssh ubuntu@IPHERE "cat > ~/.config/opencode/opencode.json << 'EOF'
{
  \"provider\": {
    \"ollama\": {
      \"npm\": \"@ai-sdk/openai-compatible\",
      \"name\": \"Ollama\",
      \"api\": \"http://localhost:11434/v1\",
      \"models\": {
        \"qwen3-coder:30b-a3b-q4_K_M\": {
          \"name\": \"Qwen3 Coder 30B A3B Q4_K_M\"
        },
        \"qwen2.5-coder:32b\": {
          \"name\": \"Qwen2.5 Coder 32B\"
        },
        \"mistral-large\": {
          \"name\": \"Mistral Large 123B\"
        },
        \"deepseek-coder\": {
          \"name\": \"Deepseek Coder\"
        }
      }
    }
  }
}
EOF"
```

**Key configuration fields:**
- `npm`: AI SDK package for OpenAI-compatible APIs
- `api`: Ollama endpoint (defaults to localhost:11434)
- `models`: Map of model IDs to display names

### 6. Clean Up Disk Space (if needed)

Check disk usage:

```bash
ssh ubuntu@IPHERE "df -h / && ollama list"
```

For A100-80GB, ensure sufficient space (~120GB free). Remove unused models if needed:

```bash
ssh ubuntu@IPHERE "ollama rm model_name"
```

### 7. Pull Models into Ollama

Pull Qwen3 Coder 30B (~20GB):

```bash
ssh ubuntu@IPHERE "ollama pull qwen3-coder:30b-a3b-q4_K_M"
```

Pull Qwen2.5 Coder 32B (~19GB):

```bash
ssh ubuntu@IPHERE "ollama pull qwen2.5-coder:32b"
```

Pull Mistral Large (~73GB):

```bash
ssh ubuntu@IPHERE "ollama pull mistral-large"
```

Pull Deepseek Coder (776MB - lightweight option):

```bash
ssh ubuntu@IPHERE "ollama pull deepseek-coder"
```

Verify models are available:

```bash
ssh ubuntu@IPHERE "ollama list"
```

## Using OpenCode

Navigate to your project and start OpenCode:

```bash
ssh ubuntu@IPHERE
cd ~/git/aitest
opencode
```

Then:
1. Press Tab to enter Plan mode (recommended for new features)
2. Use `/models` command to select which Ollama model to use
3. Ask OpenCode to help with your code

## Configuration Details

### Provider Configuration Schema

The `provider` object in `opencode.json` uses this structure:

```json
{
  "provider": {
    "provider_id": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Display Name",
      "api": "http://localhost:11434/v1",
      "models": {
        "model_id": {
          "name": "Model Display Name"
        }
      }
    }
  }
}
```

### Available Models

#### Qwen3 Coder 30B A3B Q4_K_M
- **Size**: ~20GB (optimized quantization)
- **Strengths**: Advanced coding capabilities, good tool calling, specialized for programming
- **Tool Support**: Function calling support
- **Use Case**: Primary coding assistant for complex tasks

#### Qwen2.5 Coder 32B
- **Size**: ~19GB
- **Strengths**: Optimized for function calling and coding, better than earlier versions
- **Tool Support**: Strong function calling support
- **Use Case**: Reliable coding assistant for general-purpose tasks

#### Mistral Large 123B
- **Size**: 73GB
- **Strengths**: Excellent tool/function calling support, strong reasoning, balanced for coding
- **Tool Support**: Native function calling
- **Use Case**: Most capable model for complex reasoning and coding tasks

#### Deepseek Coder
- **Size**: 776MB (very lightweight)
- **Strengths**: Specialized for coding tasks, fast inference
- **Tool Support**: Function calling support
- **Use Case**: Fast responses for code generation and analysis on limited resources

### Ollama Endpoint

OpenCode communicates with Ollama via the OpenAI-compatible API:
- Default: `http://localhost:11434/v1`
- Ollama models endpoint: `http://localhost:11434/api/tags`

## Troubleshooting

### Configuration Error: "Unrecognized key"

Ensure the config uses correct structure:
- Top-level key should be `provider` (not `providers`)
- Provider ID (e.g., `ollama`) is a key under `provider` object
- Each provider has `npm`, `name`, `api`, and `models` fields

### Out of Disk Space

Check available space:
```bash
df -h /
```

Remove models:
```bash
ollama rm model_name
```

For A100-80GB, budget ~120GB for models + system (account for OS and dependencies).

### Ollama Connection Issues

Verify Ollama is running and accessible:

```bash
curl -s http://localhost:11434/api/tags | jq '.models[].name'
```

If using remote SSH, ensure:
- Ollama is listening on 0.0.0.0 (not just localhost)
- Port 11434 is not firewalled
- Use SSH port forwarding: `ssh -L 11434:localhost:11434 ubuntu@server`

## References

- [OpenCode Documentation](https://opencode.ai/docs)
- [OpenCode Providers Configuration](https://opencode.ai/docs/providers)
- [OpenCode GitHub](https://github.com/anomalyco/opencode)
- [Ollama Documentation](https://ollama.ai)
