#!/usr/bin/env bash
set -euo pipefail

CONFIG_DIR="${HOME}/.james-agent"
CONFIG_FILE="${CONFIG_DIR}/config.json"

echo "=== james-agent setup ==="
echo ""

# Check if config exists
if [ -f "$CONFIG_FILE" ]; then
    echo "Config already exists: $CONFIG_FILE"
    read -rp "Overwrite? [y/N] " overwrite
    if [[ ! "$overwrite" =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi
fi

# Provider
echo ""
echo "--- Provider ---"
read -rp "Provider type [anthropic/openai/openai-compatible] (default: openai-compatible): " PROVIDER_TYPE
PROVIDER_TYPE="${PROVIDER_TYPE:-openai-compatible}"
PROVIDER_TYPE="$(echo "$PROVIDER_TYPE" | tr '[:upper:]' '[:lower:]')"
if [[ "$PROVIDER_TYPE" == "openai_compatible" ]]; then
    PROVIDER_TYPE="openai-compatible"
fi
if [[ "$PROVIDER_TYPE" != "anthropic" && "$PROVIDER_TYPE" != "openai" && "$PROVIDER_TYPE" != "openai-compatible" ]]; then
    echo "Unsupported provider type: $PROVIDER_TYPE"
    echo "Supported values: anthropic, openai, openai-compatible"
    exit 1
fi

DEFAULT_MODEL="claude-sonnet-4-6"
if [[ "$PROVIDER_TYPE" == "openai" || "$PROVIDER_TYPE" == "openai-compatible" ]]; then
    DEFAULT_MODEL="gpt-4o-mini"
fi
read -rp "Model (default: ${DEFAULT_MODEL}): " AGENT_MODEL
AGENT_MODEL="${AGENT_MODEL:-$DEFAULT_MODEL}"

read -rp "API Key: " API_KEY
read -rp "Base URL (leave empty for default): " BASE_URL

# Feishu
echo ""
echo "--- Feishu Channel ---"
read -rp "Enable Feishu? [y/N]: " FEISHU_ENABLED
if [[ "$FEISHU_ENABLED" =~ ^[Yy]$ ]]; then
    FEISHU_ENABLED="true"
    read -rp "App ID: " FEISHU_APP_ID
    read -rp "App Secret: " FEISHU_APP_SECRET
    read -rp "Verification Token (leave empty to skip): " FEISHU_VTOKEN
    read -rp "Webhook port (default: 9876): " FEISHU_PORT
    FEISHU_PORT="${FEISHU_PORT:-9876}"
else
    FEISHU_ENABLED="false"
    FEISHU_APP_ID=""
    FEISHU_APP_SECRET=""
    FEISHU_VTOKEN=""
    FEISHU_PORT="9876"
fi

# Telegram
echo ""
echo "--- Telegram Channel ---"
read -rp "Enable Telegram? [y/N]: " TG_ENABLED
if [[ "$TG_ENABLED" =~ ^[Yy]$ ]]; then
    TG_ENABLED="true"
    read -rp "Bot Token: " TG_TOKEN
else
    TG_ENABLED="false"
    TG_TOKEN=""
fi

# WeCom Bot
echo ""
echo "--- WeCom Bot Channel ---"
read -rp "Enable WeCom bot? [y/N]: " WECOM_ENABLED
if [[ "$WECOM_ENABLED" =~ ^[Yy]$ ]]; then
    WECOM_ENABLED="true"
    read -rp "Token: " WECOM_TOKEN
    read -rp "EncodingAESKey (43 chars): " WECOM_AES_KEY
    read -rp "ReceiveID (optional, leave empty to skip strict check): " WECOM_RECEIVE_ID
    read -rp "Callback port (default: 9886): " WECOM_PORT
    WECOM_PORT="${WECOM_PORT:-9886}"
else
    WECOM_ENABLED="false"
    WECOM_TOKEN=""
    WECOM_AES_KEY=""
    WECOM_RECEIVE_ID=""
    WECOM_PORT="9886"
fi

# Skills
echo ""
echo "--- Skills ---"
read -rp "Enable skills? [Y/n]: " SKILLS_ENABLED
if [[ "$SKILLS_ENABLED" =~ ^[Nn]$ ]]; then
    SKILLS_ENABLED="false"
else
    SKILLS_ENABLED="true"
fi
read -rp "Skills directory (leave empty for default workspace/skills): " SKILLS_DIR

# Write config
mkdir -p "$CONFIG_DIR"

cat > "$CONFIG_FILE" <<EOF_JSON
{
  "agent": {
    "workspace": "${HOME}/.james-agent/workspace",
    "model": "${AGENT_MODEL}",
    "maxTokens": 8192,
    "temperature": 0.7,
    "maxToolIterations": 20
  },
  "provider": {
    "type": "${PROVIDER_TYPE}",
    "apiKey": "${API_KEY}",
    "baseUrl": "${BASE_URL}"
  },
  "channels": {
    "telegram": {
      "enabled": ${TG_ENABLED},
      "token": "${TG_TOKEN}",
      "allowFrom": [],
      "proxy": ""
    },
    "feishu": {
      "enabled": ${FEISHU_ENABLED},
      "appId": "${FEISHU_APP_ID}",
      "appSecret": "${FEISHU_APP_SECRET}",
      "verificationToken": "${FEISHU_VTOKEN}",
      "encryptKey": "",
      "port": ${FEISHU_PORT},
      "allowFrom": []
    },
    "wecom": {
      "enabled": ${WECOM_ENABLED},
      "token": "${WECOM_TOKEN}",
      "encodingAESKey": "${WECOM_AES_KEY}",
      "receiveId": "${WECOM_RECEIVE_ID}",
      "port": ${WECOM_PORT},
      "allowFrom": []
    }
  },
  "tools": {
    "braveApiKey": "",
    "execTimeout": 60,
    "restrictToWorkspace": true
  },
  "skills": {
    "enabled": ${SKILLS_ENABLED},
    "dir": "${SKILLS_DIR}"
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18790
  }
}
EOF_JSON

chmod 600 "$CONFIG_FILE"

echo ""
echo "Config written to: $CONFIG_FILE"
echo ""
echo "Next steps:"
echo "  make onboard    # Initialize workspace"
echo "  make gateway    # Start gateway"
if [ "$FEISHU_ENABLED" = "true" ]; then
    echo "  make tunnel     # Start cloudflared tunnel for Feishu webhook"
fi
if [ "$WECOM_ENABLED" = "true" ]; then
    echo "  Configure callback URL to /wecom/bot"
fi
echo ""
echo "Done."
