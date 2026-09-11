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
  <title>SRE Incident Triage Agent — Playground & Inspector</title>
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
      --running: #e3b341;
      --panel-width: 440px;
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

    /* Header */
    header {
      background: var(--surface);
      border-bottom: 1px solid var(--border);
      padding: 12px 20px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      flex-shrink: 0;
      z-index: 10;
    }

    .title-area {
      display: flex;
      align-items: center;
      gap: 12px;
    }

    .title-area h1 {
      font-size: 1.1rem;
      font-weight: 600;
      color: var(--text-bright);
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .badges {
      display: flex;
      gap: 8px;
      align-items: center;
    }

    .badge {
      font-size: 0.75rem;
      padding: 3px 8px;
      border-radius: 12px;
      font-weight: 500;
      border: 1px solid var(--border);
      background: var(--surface-hover);
      color: var(--text-muted);
      text-decoration: none;
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }
    .badge.highlight {
      border-color: rgba(88, 166, 255, 0.4);
      color: var(--primary);
    }
    .badge.clickable {
      cursor: pointer;
      user-select: none;
      transition: all 0.15s ease;
    }
    .badge.clickable:hover {
      background: var(--border);
      color: var(--text-bright);
    }

    .header-actions {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    /* Main Split Layout */
    .app-layout {
      flex: 1;
      display: flex;
      overflow: hidden;
      position: relative;
    }

    /* Chat Pane */
    .chat-pane {
      flex: 1;
      display: flex;
      flex-direction: column;
      min-width: 0;
      overflow: hidden;
      background: var(--bg);
    }

    .quick-prompts {
      background: var(--surface);
      border-bottom: 1px solid var(--border);
      padding: 8px 20px;
      display: flex;
      gap: 8px;
      overflow-x: auto;
      flex-shrink: 0;
    }

    .quick-btn {
      background: var(--surface-hover);
      border: 1px solid var(--border);
      color: var(--text);
      padding: 5px 10px;
      border-radius: 6px;
      font-size: 0.78rem;
      cursor: pointer;
      white-space: nowrap;
      transition: all 0.15s ease;
      text-decoration: none;
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }
    .quick-btn:hover {
      background: var(--border);
      color: var(--text-bright);
    }
    .quick-btn.crit { border-left: 3px solid var(--crit); }
    .quick-btn.warn { border-left: 3px solid var(--warn); }
    .quick-btn.info { border-left: 3px solid var(--info); }

    main#chat-stream {
      flex: 1;
      overflow-y: auto;
      padding: 20px;
      display: flex;
      flex-direction: column;
      gap: 18px;
    }

    .message {
      display: flex;
      gap: 12px;
      max-width: 860px;
      width: 100%;
      margin: 0 auto;
    }

    .avatar {
      width: 32px;
      height: 32px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 0.95rem;
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
      padding: 14px 18px;
      line-height: 1.6;
      font-size: 0.92rem;
      overflow-wrap: break-word;
      min-width: 0;
    }
    .message.user .bubble {
      background: #1b283b;
      border-color: #2b456b;
    }

    .bubble h1, .bubble h2, .bubble h3 {
      color: var(--text-bright);
      margin: 12px 0 6px 0;
    }
    .bubble h1:first-child, .bubble h2:first-child, .bubble h3:first-child {
      margin-top: 0;
    }
    .bubble p { margin-bottom: 8px; }
    .bubble p:last-child { margin-bottom: 0; }
    .bubble ul, .bubble ol { margin-left: 18px; margin-bottom: 8px; }
    .bubble li { margin-bottom: 3px; }
    .bubble pre {
      background: #0d1117;
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 10px;
      overflow-x: auto;
      margin: 10px 0;
    }
    .bubble code {
      background: rgba(110, 118, 129, 0.3);
      padding: 2px 5px;
      border-radius: 4px;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
      font-size: 0.85em;
    }
    .bubble pre code {
      background: none;
      padding: 0;
    }

    .live-status-indicator {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 0.82rem;
      color: var(--text-muted);
      margin-bottom: 8px;
      padding: 4px 8px;
      background: rgba(88, 166, 255, 0.08);
      border: 1px solid rgba(88, 166, 255, 0.2);
      border-radius: 4px;
      width: fit-content;
    }

    footer {
      background: var(--surface);
      border-top: 1px solid var(--border);
      padding: 14px 20px;
      flex-shrink: 0;
    }

    .input-container {
      max-width: 860px;
      margin: 0 auto;
      display: flex;
      gap: 10px;
      position: relative;
    }

    textarea {
      flex: 1;
      background: var(--bg);
      border: 1px solid var(--border);
      color: var(--text-bright);
      border-radius: 8px;
      padding: 10px 12px;
      font-size: 0.92rem;
      resize: none;
      height: 48px;
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
      padding: 0 18px;
      font-weight: 600;
      font-size: 0.92rem;
      cursor: pointer;
      transition: background 0.15s;
      display: flex;
      align-items: center;
      gap: 6px;
      white-space: nowrap;
    }
    button#send-btn:hover:not(:disabled) {
      background: var(--primary-hover);
    }
    button#send-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    /* Side Panel */
    aside.side-panel {
      width: var(--panel-width);
      background: var(--surface);
      border-left: 1px solid var(--border);
      display: flex;
      flex-direction: column;
      flex-shrink: 0;
      transition: width 0.2s ease, transform 0.2s ease;
      overflow: hidden;
    }

    aside.side-panel.collapsed {
      width: 0;
      border-left: none;
      visibility: hidden;
    }

    .side-header {
      border-bottom: 1px solid var(--border);
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 10px;
      background: #13171d;
      flex-shrink: 0;
    }

    .side-tabs {
      display: flex;
      gap: 4px;
    }

    .side-tab {
      background: none;
      border: none;
      border-bottom: 2px solid transparent;
      color: var(--text-muted);
      padding: 10px 12px;
      font-size: 0.82rem;
      font-weight: 600;
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 6px;
      transition: color 0.15s ease;
    }
    .side-tab:hover {
      color: var(--text-bright);
    }
    .side-tab.active {
      color: var(--primary);
      border-bottom-color: var(--primary);
    }

    .tab-count {
      background: var(--surface-hover);
      border: 1px solid var(--border);
      border-radius: 10px;
      padding: 1px 6px;
      font-size: 0.7rem;
      color: var(--text);
    }

    .icon-btn {
      background: none;
      border: none;
      color: var(--text-muted);
      cursor: pointer;
      padding: 6px;
      border-radius: 4px;
      font-size: 0.9rem;
      display: inline-flex;
      align-items: center;
      justify-content: center;
    }
    .icon-btn:hover {
      background: var(--surface-hover);
      color: var(--text-bright);
    }

    .tab-content {
      flex: 1;
      overflow-y: auto;
      padding: 14px;
      display: none;
    }
    .tab-content.active {
      display: block;
    }

    /* Tool Call Cards */
    .tool-card {
      background: var(--bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      margin-bottom: 12px;
      padding: 12px;
      transition: border-color 0.2s;
    }
    .tool-card.running {
      border-color: var(--running);
    }
    .tool-card.success {
      border-color: rgba(63, 185, 80, 0.4);
    }
    .tool-card.error {
      border-color: rgba(248, 81, 73, 0.5);
    }

    .tool-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 8px;
    }

    .tool-name-wrap {
      display: flex;
      align-items: center;
      gap: 6px;
      font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
      font-weight: 600;
      font-size: 0.86rem;
      color: var(--text-bright);
    }

    .tool-status-badge {
      font-size: 0.68rem;
      padding: 2px 6px;
      border-radius: 10px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }
    .tool-status-badge.running {
      background: rgba(227, 179, 65, 0.2);
      color: var(--running);
      border: 1px solid rgba(227, 179, 65, 0.4);
    }
    .tool-status-badge.success {
      background: rgba(63, 185, 80, 0.2);
      color: var(--info);
      border: 1px solid rgba(63, 185, 80, 0.4);
    }
    .tool-status-badge.error {
      background: rgba(248, 81, 73, 0.2);
      color: var(--crit);
      border: 1px solid rgba(248, 81, 73, 0.4);
    }

    .section-label {
      font-size: 0.72rem;
      text-transform: uppercase;
      color: var(--text-muted);
      font-weight: 600;
      margin-top: 8px;
      margin-bottom: 4px;
    }

    pre.tool-json {
      background: #090d12;
      border: 1px solid rgba(48, 54, 61, 0.6);
      border-radius: 6px;
      padding: 8px;
      font-size: 0.78rem;
      color: #79c0ff;
      max-height: 180px;
      overflow-y: auto;
      white-space: pre-wrap;
      word-break: break-all;
    }
    pre.tool-json.response {
      color: #7ee787;
    }

    /* Events List */
    .event-card {
      background: var(--bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 10px;
      margin-bottom: 10px;
      font-size: 0.8rem;
    }

    .event-top {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 6px;
    }

    .event-tag {
      font-size: 0.7rem;
      padding: 2px 6px;
      border-radius: 4px;
      font-weight: 600;
      background: var(--surface-hover);
      color: var(--primary);
    }
    .event-time {
      font-size: 0.7rem;
      color: var(--text-muted);
    }

    /* Telemetry Tab */
    .telemetry-box {
      background: var(--bg);
      border: 1px solid var(--border);
      border-radius: 8px;
      padding: 14px;
      margin-bottom: 14px;
    }

    .telem-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 6px 0;
      border-bottom: 1px solid rgba(48, 54, 61, 0.4);
      font-size: 0.82rem;
    }
    .telem-row:last-child {
      border-bottom: none;
    }
    .telem-k {
      color: var(--text-muted);
    }
    .telem-v {
      color: var(--text-bright);
      font-weight: 500;
    }
    .telem-v.mono {
      font-family: ui-monospace, SFMono-Regular, monospace;
      font-size: 0.78rem;
    }

    .empty-state {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: 40px 20px;
      text-align: center;
      color: var(--text-muted);
      gap: 10px;
    }
    .empty-state .icon {
      font-size: 2rem;
    }
    .empty-state .title {
      font-weight: 600;
      color: var(--text);
      font-size: 0.9rem;
    }
    .empty-state .sub {
      font-size: 0.8rem;
      line-height: 1.4;
    }

    .spinner {
      display: inline-block;
      width: 14px;
      height: 14px;
      border: 2px solid rgba(0,0,0,0.2);
      border-radius: 50%;
      border-top-color: #0d1117;
      animation: spin 0.8s linear infinite;
    }
    .spinner.blue {
      border: 2px solid rgba(88, 166, 255, 0.2);
      border-top-color: var(--primary);
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
        <span class="badge">OpenTelemetry Trace</span>
      </div>
    </div>
    <div class="header-actions">
      <button class="badge highlight clickable" onclick="toggleSidePanel()" id="toggle-panel-btn">◧ Side Panel</button>
      <a href="/.well-known/agent-card.json" target="_blank" class="badge clickable">Agent Card ↗</a>
    </div>
  </header>

  <div class="app-layout">
    <!-- Left Column: Chat -->
    <div class="chat-pane">
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
            <p><strong>Welcome to the SRE Incident Triage Agent Playground & Inspector!</strong></p>
            <p>I autonomously parse server logs, classify incident severity (<code>CRITICAL</code>, <code>WARNING</code>, <code>INFO</code>), detect anomalies & deadlocks, mask sensitive credentials (<code>[REDACTED]</code>), and generate standardized postmortem reports.</p>
            <p>Watch tool executions, raw SSE event streams, and agent telemetry in the <strong>Side Panel</strong> on the right as I reason.</p>
          </div>
        </div>
      </main>

      <footer>
        <div class="input-container">
          <textarea id="prompt-input" placeholder="Ask the SRE agent or provide a log file path (e.g., sample_logs/db_exhaustion.log)..." onkeydown="handleKeyDown(event)"></textarea>
          <button id="send-btn" onclick="sendMessage()">Send</button>
        </div>
      </footer>
    </div>

    <!-- Right Column: Side Panel -->
    <aside class="side-panel" id="side-panel">
      <div class="side-header">
        <div class="side-tabs">
          <button class="side-tab active" id="tab-btn-tools" onclick="switchTab('tools')">🛠️ Tool Calls <span class="tab-count" id="tool-count">0</span></button>
          <button class="side-tab" id="tab-btn-events" onclick="switchTab('events')">⚡ Events <span class="tab-count" id="event-count">0</span></button>
          <button class="side-tab" id="tab-btn-telemetry" onclick="switchTab('telemetry')">📊 Telemetry</button>
        </div>
        <button class="icon-btn" onclick="clearTraces()" title="Clear Tool & Event Traces">🗑️</button>
      </div>

      <!-- Tab 1: Tools -->
      <div class="tab-content active" id="tab-pane-tools">
        <div id="tools-list">
          <div class="empty-state" id="tools-empty">
            <div class="icon">🛠️</div>
            <div class="title">No Tool Calls Yet</div>
            <div class="sub">Triage an incident to watch the agent execute <code>read_log_file</code>, <code>parse_log_snippet</code>, and <code>format_incident_report</code> live.</div>
          </div>
        </div>
      </div>

      <!-- Tab 2: Events -->
      <div class="tab-content" id="tab-pane-events">
        <div id="events-list">
          <div class="empty-state" id="events-empty">
            <div class="icon">⚡</div>
            <div class="title">No Event Frames Yet</div>
            <div class="sub">Raw SSE frames from <code>POST /run_sse</code> will stream here in real time.</div>
          </div>
        </div>
      </div>

      <!-- Tab 3: Telemetry -->
      <div class="tab-content" id="tab-pane-telemetry">
        <div class="telemetry-box">
          <div class="telem-row"><span class="telem-k">Agent Name:</span> <span class="telem-v">sre-triage-agent</span></div>
          <div class="telem-row"><span class="telem-k">Model:</span> <span class="telem-v">gemini-3.8-flash</span></div>
          <div class="telem-row"><span class="telem-k">Engine:</span> <span class="telem-v">google.golang.org/genai</span></div>
          <div class="telem-row"><span class="telem-k">Status:</span> <span class="telem-v" id="telem-status" style="color: var(--info);">READY</span></div>
          <div class="telem-row"><span class="telem-k">Session ID:</span> <span class="telem-v mono" id="telem-session">-</span></div>
          <div class="telem-row"><span class="telem-k">Tool Calls:</span> <span class="telem-v" id="telem-tool-calls">0</span></div>
          <div class="telem-row"><span class="telem-k">Events Count:</span> <span class="telem-v" id="telem-event-count">0</span></div>
          <div class="telem-row"><span class="telem-k">Tracing:</span> <span class="telem-v">Cloud Trace (OTel)</span></div>
          <div class="telem-row"><span class="telem-k">Protocol:</span> <span class="telem-v">A2A JSON-RPC & ADK SSE</span></div>
        </div>
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <a href="/.well-known/agent-card.json" target="_blank" class="quick-btn">📄 Agent Card (/.well-known/agent-card.json)</a>
          <a href="/apps/sre-triage-agent/app-info" target="_blank" class="quick-btn">ℹ️ App Info (/apps/.../app-info)</a>
          <a href="/list-apps" target="_blank" class="quick-btn">📋 List Apps (/list-apps)</a>
          <button class="quick-btn" onclick="resetSession()">🔄 Reset Session ID</button>
        </div>
      </div>
    </aside>
  </div>

  <script>
    const chatStream = document.getElementById('chat-stream');
    const promptInput = document.getElementById('prompt-input');
    const sendBtn = document.getElementById('send-btn');
    const toolsList = document.getElementById('tools-list');
    const eventsList = document.getElementById('events-list');
    const toolCountBadge = document.getElementById('tool-count');
    const eventCountBadge = document.getElementById('event-count');
    const telemStatus = document.getElementById('telem-status');
    const telemSession = document.getElementById('telem-session');
    const telemToolCalls = document.getElementById('telem-tool-calls');
    const telemEventCount = document.getElementById('telem-event-count');
    const sidePanel = document.getElementById('side-panel');

    let isWaiting = false;
    let totalToolCalls = 0;
    let totalEvents = 0;
    let sessionId = generateSessionId();

    telemSession.innerText = sessionId.substring(0, 13) + '...';

    function generateSessionId() {
      return 'sre-sess-' + Math.random().toString(36).substring(2, 11) + '-' + Date.now().toString(36);
    }

    function resetSession() {
      sessionId = generateSessionId();
      telemSession.innerText = sessionId.substring(0, 13) + '...';
      clearTraces();
    }

    function toggleSidePanel() {
      sidePanel.classList.toggle('collapsed');
    }

    function switchTab(tabName) {
      document.querySelectorAll('.side-tab').forEach(t => t.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(p => p.classList.remove('active'));

      const tabBtn = document.getElementById('tab-btn-' + tabName);
      const pane = document.getElementById('tab-pane-' + tabName);
      if (tabBtn) tabBtn.classList.add('active');
      if (pane) pane.classList.add('active');
    }

    function clearTraces() {
      totalToolCalls = 0;
      totalEvents = 0;
      toolCountBadge.innerText = '0';
      eventCountBadge.innerText = '0';
      telemToolCalls.innerText = '0';
      telemEventCount.innerText = '0';
      toolsList.innerHTML = '<div class="empty-state" id="tools-empty"><div class="icon">🛠️</div><div class="title">No Tool Calls Yet</div><div class="sub">Triage an incident to watch the agent execute tools live.</div></div>';
      eventsList.innerHTML = '<div class="empty-state" id="events-empty"><div class="icon">⚡</div><div class="title">No Event Frames Yet</div><div class="sub">Raw SSE frames from <code>POST /run_sse</code> will stream here.</div></div>';
    }

    function escapeHtml(str) {
      if (typeof str !== 'string') return '' + str;
      return str.replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/"/g, '&quot;')
                .replace(/'/g, '&#039;');
    }

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

    function recordEvent(type, rawObj) {
      totalEvents++;
      eventCountBadge.innerText = totalEvents;
      telemEventCount.innerText = totalEvents;

      const empty = document.getElementById('events-empty');
      if (empty) empty.remove();

      const timeStr = new Date().toLocaleTimeString();
      const card = document.createElement('div');
      card.className = 'event-card';
      card.innerHTML = '<div class="event-top">' +
        '<span class="event-tag">' + escapeHtml(type) + '</span>' +
        '<span class="event-time">' + timeStr + '</span>' +
        '</div>' +
        '<pre class="tool-json"><code>' + escapeHtml(JSON.stringify(rawObj, null, 2)) + '</code></pre>';
      eventsList.appendChild(card);
      eventsList.scrollTop = eventsList.scrollHeight;
    }

    function recordToolCall(name, args) {
      totalToolCalls++;
      toolCountBadge.innerText = totalToolCalls;
      telemToolCalls.innerText = totalToolCalls;

      const empty = document.getElementById('tools-empty');
      if (empty) empty.remove();

      const callId = 'call-' + Date.now() + '-' + Math.floor(Math.random()*1000);
      const card = document.createElement('div');
      card.className = 'tool-card running';
      card.id = callId;
      card.setAttribute('data-tool-name', name);

      card.innerHTML = '<div class="tool-header">' +
        '<div class="tool-name-wrap"><span>🔧</span> <span>' + escapeHtml(name) + '</span></div>' +
        '<span class="tool-status-badge running">RUNNING</span>' +
        '</div>' +
        '<div class="section-label">Input Arguments</div>' +
        '<pre class="tool-json"><code>' + escapeHtml(JSON.stringify(args, null, 2)) + '</code></pre>' +
        '<div class="response-container"></div>';

      toolsList.appendChild(card);
      toolsList.scrollTop = toolsList.scrollHeight;
      return callId;
    }

    function recordToolResponse(name, response) {
      // Find the most recent running tool card for this name
      const cards = toolsList.querySelectorAll('.tool-card[data-tool-name="' + name + '"]');
      let targetCard = null;
      for (let i = cards.length - 1; i >= 0; i--) {
        if (cards[i].classList.contains('running')) {
          targetCard = cards[i];
          break;
        }
      }
      if (!targetCard && cards.length > 0) {
        targetCard = cards[cards.length - 1];
      }

      if (targetCard) {
        targetCard.classList.remove('running');
        targetCard.classList.add('success');
        const badge = targetCard.querySelector('.tool-status-badge');
        if (badge) {
          badge.className = 'tool-status-badge success';
          badge.innerText = 'SUCCESS';
        }
        const respContainer = targetCard.querySelector('.response-container');
        if (respContainer) {
          respContainer.innerHTML = '<div class="section-label">Tool Output</div>' +
            '<pre class="tool-json response"><code>' + escapeHtml(JSON.stringify(response, null, 2)) + '</code></pre>';
        }
      }
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
      telemStatus.innerText = 'TRIAGING';
      telemStatus.style.color = 'var(--running)';

      const agentBubble = appendMessage('agent', '');
      const liveStatus = document.createElement('div');
      liveStatus.className = 'live-status-indicator';
      liveStatus.innerHTML = '<span class="spinner blue"></span> <span id="live-status-text">Reasoning with Gemini 3.8 Flash...</span>';
      agentBubble.appendChild(liveStatus);

      const contentDiv = document.createElement('div');
      contentDiv.className = 'agent-content';
      agentBubble.appendChild(contentDiv);

      let accumulatedMarkdown = '';

      try {
        const response = await fetch('/run_sse', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            appName: "sre-triage-agent",
            userId: "user-playground",
            sessionId: sessionId,
            newMessage: {
              role: "user",
              parts: [{ text: text }]
            }
          })
        });

        if (!response.ok || !response.body) {
          throw new Error('SSE stream error: HTTP ' + response.status);
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
          const res = await reader.read();
          if (res.done) break;

          buffer += decoder.decode(res.value, { stream: true });
          const lines = buffer.split('\n');
          buffer = lines.pop(); // keep partial trailing line

          for (const line of lines) {
            const trimmed = line.trim();
            if (!trimmed.startsWith('data:')) continue;
            const dataStr = trimmed.slice(5).trim();
            if (!dataStr) continue;

            try {
              const frame = JSON.parse(dataStr);
              recordEvent(frame.content && frame.content.role ? frame.content.role : 'event', frame);

              if (frame.content && Array.isArray(frame.content.parts)) {
                for (const part of frame.content.parts) {
                  if (part.functionCall) {
                    const fc = part.functionCall;
                    const statusText = document.getElementById('live-status-text');
                    if (statusText) statusText.innerHTML = 'Executing <code>' + escapeHtml(fc.name) + '</code>...';
                    recordToolCall(fc.name, fc.args || {});
                  } else if (part.functionResponse) {
                    const fr = part.functionResponse;
                    const statusText = document.getElementById('live-status-text');
                    if (statusText) statusText.innerHTML = 'Completed <code>' + escapeHtml(fr.name) + '</code>';
                    recordToolResponse(fr.name, fr.response || {});
                  } else if (part.text) {
                    accumulatedMarkdown += part.text;
                    contentDiv.innerHTML = renderMarkdown(accumulatedMarkdown);
                    if (liveStatus && liveStatus.parentNode) {
                      liveStatus.remove(); // Remove spinner once markdown text arrives
                    }
                  }
                }
              }
            } catch (err) {
              console.warn('SSE frame parse error:', err, dataStr);
            }
          }
        }

        if (accumulatedMarkdown) {
          contentDiv.innerHTML = renderMarkdown(accumulatedMarkdown);
        }
        if (liveStatus && liveStatus.parentNode) {
          liveStatus.remove();
        }

      } catch (sseErr) {
        console.warn('Streaming failed, falling back to A2A JSON-RPC:', sseErr);
        const statusText = document.getElementById('live-status-text');
        if (statusText) statusText.innerHTML = 'Streaming unavailable, invoking JSON-RPC fallback...';

        try {
          const fallbackResp = await fetch('/a2a/invoke', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
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

          const data = await fallbackResp.json();
          recordEvent('jsonrpc_fallback', data);

          const msg = (data && data.result && data.result.message) ? data.result.message : (data ? data.result : null);
          let replyText = null;
          if (msg && Array.isArray(msg.parts) && msg.parts.length > 0) {
            replyText = msg.parts.map(p => p.text || '').join('\n');
          } else if (data && typeof data.result === 'string') {
            replyText = data.result;
          }

          if (liveStatus && liveStatus.parentNode) liveStatus.remove();

          if (replyText) {
            contentDiv.innerHTML = renderMarkdown(replyText);
          } else if (data && data.error) {
            contentDiv.innerHTML = renderMarkdown('⚠️ **Triage Error (' + data.error.code + '):** ' + data.error.message);
          } else {
            contentDiv.innerHTML = renderMarkdown('⚠️ **Unexpected response format:**\n\n<pre>' + JSON.stringify(data, null, 2) + '</pre>');
          }
        } catch (fbErr) {
          if (liveStatus && liveStatus.parentNode) liveStatus.remove();
          contentDiv.innerHTML = renderMarkdown('⚠️ **Transport Error:** ' + fbErr.message);
        }
      } finally {
        isWaiting = false;
        sendBtn.disabled = false;
        sendBtn.innerText = 'Send';
        telemStatus.innerText = 'READY';
        telemStatus.style.color = 'var(--info)';
        chatStream.scrollTop = chatStream.scrollHeight;
        promptInput.focus();
      }
    }
  </script>
</body>
</html>
`
