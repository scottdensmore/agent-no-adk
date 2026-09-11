package a2a

import (
	"net/http"
)

func (h *Handler) handleWebUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(webUIHTML))
}

const webUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>SRE Incident Triage Agent — Web Playground</title>
  <script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
  <style>
    :root {
      --bg: #0d1117;
      --surface: #161b22;
      --surface-hover: #21262d;
      --border: #30363d;
      --primary: #58a6ff;
      --primary-hover: #388bfd;
      --text: #c9d1d9;
      --text-bright: #f0f6fc;
      --text-muted: #8b949e;
      --crit: #f85149;
      --warn: #d29922;
      --info: #3fb950;
    }

    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      background-color: var(--bg);
      color: var(--text);
      display: flex;
      flex-direction: column;
      height: 100vh;
      overflow: hidden;
    }

    header {
      background: var(--surface);
      border-bottom: 1px solid var(--border);
      padding: 14px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      flex-shrink: 0;
    }

    .title-area {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .title-area h1 {
      font-size: 1.15rem;
      font-weight: 600;
      color: var(--text-bright);
    }

    .badges {
      display: flex;
      gap: 8px;
    }

    .badge {
      font-size: 0.75rem;
      padding: 3px 8px;
      border-radius: 12px;
      font-weight: 500;
      border: 1px solid var(--border);
      background: var(--surface-hover);
      color: var(--text-muted);
    }
    .badge.highlight {
      border-color: rgba(88, 166, 255, 0.4);
      color: var(--primary);
    }

    .quick-prompts {
      background: var(--surface);
      border-bottom: 1px solid var(--border);
      padding: 10px 24px;
      display: flex;
      gap: 8px;
      overflow-x: auto;
      flex-shrink: 0;
    }

    .quick-btn {
      background: var(--surface-hover);
      border: 1px solid var(--border);
      color: var(--text);
      padding: 6px 12px;
      border-radius: 6px;
      font-size: 0.8rem;
      cursor: pointer;
      white-space: nowrap;
      transition: all 0.15s ease;
    }
    .quick-btn:hover {
      background: var(--border);
      color: var(--text-bright);
    }
    .quick-btn.crit { border-left: 3px solid var(--crit); }
    .quick-btn.warn { border-left: 3px solid var(--warn); }
    .quick-btn.info { border-left: 3px solid var(--info); }

    main {
      flex: 1;
      overflow-y: auto;
      padding: 24px;
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    .message {
      display: flex;
      gap: 14px;
      max-width: 900px;
      margin: 0 auto;
      width: 100%;
    }

    .avatar {
      width: 34px;
      height: 34px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 1rem;
      flex-shrink: 0;
      font-weight: bold;
    }
    .avatar.user { background: #1f6feb; color: #fff; }
    .avatar.agent { background: #238636; color: #fff; }

    .bubble {
      flex: 1;
      background: var(--surface);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 16px 20px;
      line-height: 1.6;
      font-size: 0.95rem;
      overflow-wrap: break-word;
    }
    .message.user .bubble {
      background: #1b283b;
      border-color: #2b456b;
    }

    .bubble h1, .bubble h2, .bubble h3 {
      color: var(--text-bright);
      margin: 14px 0 8px 0;
    }
    .bubble h1:first-child, .bubble h2:first-child, .bubble h3:first-child {
      margin-top: 0;
    }
    .bubble p { margin-bottom: 10px; }
    .bubble ul, .bubble ol { margin-left: 20px; margin-bottom: 10px; }
    .bubble li { margin-bottom: 4px; }
    .bubble pre {
      background: #0d1117;
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 12px;
      overflow-x: auto;
      margin: 12px 0;
    }
    .bubble code {
      background: rgba(110, 118, 129, 0.4);
      padding: 2px 6px;
      border-radius: 4px;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
      font-size: 0.88em;
    }
    .bubble pre code {
      background: none;
      padding: 0;
    }

    footer {
      background: var(--surface);
      border-top: 1px solid var(--border);
      padding: 16px 24px;
      flex-shrink: 0;
    }

    .input-container {
      max-width: 900px;
      margin: 0 auto;
      display: flex;
      gap: 12px;
      position: relative;
    }

    textarea {
      flex: 1;
      background: var(--bg);
      border: 1px solid var(--border);
      color: var(--text-bright);
      border-radius: 8px;
      padding: 12px 14px;
      font-size: 0.95rem;
      resize: none;
      height: 52px;
      font-family: inherit;
      outline: none;
      transition: border-color 0.2s;
    }
    textarea:focus {
      border-color: var(--primary);
    }

    button#send-btn {
      background: var(--primary);
      color: #0d1117;
      border: none;
      border-radius: 8px;
      padding: 0 20px;
      font-weight: 600;
      font-size: 0.95rem;
      cursor: pointer;
      transition: background 0.15s;
      display: flex;
      align-items: center;
      gap: 6px;
    }
    button#send-btn:hover:not(:disabled) {
      background: var(--primary-hover);
    }
    button#send-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .spinner {
      display: inline-block;
      width: 16px;
      height: 16px;
      border: 2px solid rgba(0,0,0,0.2);
      border-radius: 50%;
      border-top-color: #0d1117;
      animation: spin 0.8s linear infinite;
    }
    @keyframes spin {
      to { transform: rotate(360deg); }
    }
  </style>
</head>
<body>

  <header>
    <div class="title-area">
      <h1>🛡️ SRE Incident Triage Agent</h1>
      <div class="badges">
        <span class="badge highlight">A2A v1.0</span>
        <span class="badge">gemini-3.8-flash</span>
        <span class="badge">Cloud Trace & Logging</span>
      </div>
    </div>
    <div>
      <a href="/.well-known/agent-card.json" target="_blank" class="badge">Agent Card ↗</a>
    </div>
  </header>

  <div class="quick-prompts">
    <button class="quick-btn crit" onclick="quickSend('Triage the incident in sample_logs/db_exhaustion.log')">🚨 Triage DB Exhaustion (CRITICAL)</button>
    <button class="quick-btn warn" onclick="quickSend('Triage the incident in sample_logs/gateway_timeout.log')">⚠️ Triage Gateway Timeout (WARNING)</button>
    <button class="quick-btn info" onclick="quickSend('Triage the incident in sample_logs/normal_startup.log')">ℹ️ Verify Normal Startup (INFO)</button>
    <button class="quick-btn" onclick="quickSend('What are your SRE triage capabilities?')">🔍 Capabilities</button>
  </div>

  <main id="chat-stream">
    <div class="message agent">
      <div class="avatar agent">🤖</div>
      <div class="bubble">
        <p><strong>Welcome to the SRE Incident Triage Agent Playground!</strong></p>
        <p>I autonomously parse service logs, classify incident severity (<code>CRITICAL</code>, <code>WARNING</code>, <code>INFO</code>), detect anomalies & deadlocks, mask sensitive credentials (<code>[REDACTED]</code>), and generate standardized postmortem reports.</p>
        <p>Click a quick triage button above or enter a prompt below to start.</p>
      </div>
    </div>
  </main>

  <footer>
    <div class="input-container">
      <textarea id="prompt-input" placeholder="Ask the SRE agent or provide a log file path (e.g., sample_logs/db_exhaustion.log)..." onkeydown="handleKeyDown(event)"></textarea>
      <button id="send-btn" onclick="sendMessage()">Send</button>
    </div>
  </footer>

  <script>
    const chatStream = document.getElementById('chat-stream');
    const promptInput = document.getElementById('prompt-input');
    const sendBtn = document.getElementById('send-btn');
    let isWaiting = false;

    function renderMarkdown(text) {
      if (window.marked && window.marked.parse) {
        return window.marked.parse(text);
      }
      const div = document.createElement('div');
      div.innerText = text;
      return div.innerHTML.replace(/\n/g, '<br>');
    }

    function appendMessage(role, content) {
      const msgDiv = document.createElement('div');
      msgDiv.className = 'message ' + role;

      const avatar = document.createElement('div');
      avatar.className = 'avatar ' + role;
      avatar.innerText = role === 'user' ? '👤' : '🤖';

      const bubble = document.createElement('div');
      bubble.className = 'bubble';
      bubble.innerHTML = renderMarkdown(content);

      msgDiv.appendChild(avatar);
      msgDiv.appendChild(bubble);
      chatStream.appendChild(msgDiv);
      chatStream.scrollTop = chatStream.scrollHeight;
      return bubble;
    }

    function quickSend(prompt) {
      promptInput.value = prompt;
      sendMessage();
    }

    function handleKeyDown(e) {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
      }
    }

    async function sendMessage() {
      const text = promptInput.value.trim();
      if (!text || isWaiting) return;

      appendMessage('user', text);
      promptInput.value = '';
      isWaiting = true;
      sendBtn.disabled = true;
      sendBtn.innerHTML = '<span class="spinner"></span> Triaging...';

      const loadingBubble = appendMessage('agent', '_Triaging telemetry with Gemini 3.8 Flash..._');

      try {
        const response = await fetch('/a2a/invoke', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({
            jsonrpc: "2.0",
            id: "web-" + Date.now(),
            method: "message/send",
            params: {
              message: {
                parts: [{ text: text }]
              }
            }
          })
        });

        const data = await response.json();
        if (data.result && data.result.parts && data.result.parts[0]) {
          loadingBubble.innerHTML = renderMarkdown(data.result.parts[0].text);
        } else if (data.error) {
          loadingBubble.innerHTML = renderMarkdown('⚠️ **Triage Error (' + data.error.code + '):** ' + data.error.message);
        } else {
          loadingBubble.innerHTML = renderMarkdown('⚠️ Unexpected response format from agent.');
        }
      } catch (err) {
        loadingBubble.innerHTML = renderMarkdown('⚠️ **Network/Transport Error:** ' + err.message);
      } finally {
        isWaiting = false;
        sendBtn.disabled = false;
        sendBtn.innerText = 'Send';
        chatStream.scrollTop = chatStream.scrollHeight;
        promptInput.focus();
      }
    }
  </script>
</body>
</html>
`
