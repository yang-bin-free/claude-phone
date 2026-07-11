(() => {
  const state = { ws: null, sessionId: "", retry: 0, assistantChunk: null, pendingTokens: "", tokenFrame: 0, sessions: [], projects: [], templates: [] };
  const messages = document.querySelector("#messages");
  const connection = document.querySelector("#connection-state");
  const params = new URLSearchParams(location.search);
  const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
  const platform = params.get("platform") || "desktop";
  const configuredWS = params.get("ws") || "";
  const deviceToken = params.get("deviceToken") || `${platform}-${token || "local"}`;
  const deviceName = params.get("deviceName") || (platform === "mobile" ? "Android" : "Mac");
  document.body.classList.add(platform);
  window.claudePhone = {
    adminToken: token,
    state,
    setPrompt(value) { document.querySelector("#prompt").value = value || ""; }
  };

  function append(role, text) {
    messages.querySelector(".empty")?.remove();
    const node = document.createElement("div");
    node.className = `message ${role}`;
    node.textContent = text;
    messages.append(node);
    while (messages.children.length > 500) messages.firstElementChild?.remove();
    messages.scrollTop = messages.scrollHeight;
    return node;
  }

  function flushTokens() {
    state.tokenFrame = 0;
    if (!state.pendingTokens) return;
    if (!state.assistantChunk) state.assistantChunk = append("assistant", "");
    state.assistantChunk.textContent += state.pendingTokens;
    state.pendingTokens = "";
    messages.scrollTop = messages.scrollHeight;
  }

  function queueToken(content) {
    state.pendingTokens += content || "";
    if (!state.tokenFrame) state.tokenFrame = requestAnimationFrame(flushTokens);
  }

  function resetStream() {
    if (state.tokenFrame) cancelAnimationFrame(state.tokenFrame);
    state.tokenFrame = 0;
    state.pendingTokens = "";
    state.assistantChunk = null;
  }

  function send(value) {
    if (state.ws?.readyState === WebSocket.OPEN) state.ws.send(JSON.stringify(value));
  }

  function selectSession(sessionId, name) {
    state.sessionId = sessionId;
    resetStream();
    document.querySelector("#view-title").textContent = name || "会话";
    document.querySelector("#stop-session").disabled = false;
    messages.replaceChildren();
    send({ type: "control", action: "select_session", sessionId });
    send({ type: "control", action: "load_history", sessionId, limit: 500 });
    renderSessions();
  }

  function renderHistory(items) {
    messages.replaceChildren();
    resetStream();
    let assistant = null;
    (items || []).forEach(item => {
      if (item.type === "text") {
        append("user", item.content || "");
        assistant = null;
      } else if (item.type === "token") {
        if (!assistant) assistant = append("assistant", "");
        assistant.textContent += item.content || "";
      } else if (item.type === "done") {
        assistant = null;
      }
    });
  }

  function renderSessions() {
    const list = document.querySelector("#session-list");
    list.replaceChildren();
    const select = document.querySelector("#mobile-session-select");
    select.replaceChildren(new Option("会话", ""));
    state.sessions.forEach(session => {
      const button = document.createElement("button");
      button.className = `session-item${session.sessionId === state.sessionId ? " active" : ""}`;
      button.textContent = session.name || session.sessionId;
      button.addEventListener("click", () => selectSession(session.sessionId, session.name));
      list.append(button);
      select.add(new Option(session.name || session.sessionId, session.sessionId, false, session.sessionId === state.sessionId));
    });
  }

  function connect() {
    const scheme = location.protocol === "https:" ? "wss" : "ws";
    const endpoint = configuredWS || `${scheme}://${location.host}/ws`;
    const ws = new WebSocket(endpoint);
    state.ws = ws;
    ws.onopen = () => {
      state.retry = 0;
      connection.textContent = "已连接";
      document.querySelector("#status-dot").classList.add("online");
      send({ type: "auth", deviceToken, deviceName });
    };
    ws.onclose = () => {
      connection.textContent = "重新连接中";
      document.querySelector("#status-dot").classList.remove("online");
      const delay = Math.min(1000 * (2 ** state.retry++), 15000);
      setTimeout(connect, delay);
    };
    ws.onmessage = event => {
      const msg = JSON.parse(event.data);
      switch (msg.type) {
        case "hello":
          send({ type: "control", action: "list_sessions", limit: 100 });
          send({ type: "control", action: "list_projects" });
          send({ type: "control", action: "list_templates" });
          break;
        case "session_list":
          state.sessions = msg.sessions || [];
          renderSessions();
          break;
        case "session_created":
          state.sessionId = msg.sessionId;
          resetStream();
          document.querySelector("#view-title").textContent = msg.name || "新会话";
          document.querySelector("#stop-session").disabled = false;
          send({ type: "control", action: "list_sessions", limit: 100 });
          break;
        case "project_list": {
          state.projects = msg.projects || [];
          const select = document.querySelector("#create-project");
          select.replaceChildren(new Option("默认目录", ""));
          state.projects.forEach(project => select.add(new Option(project.name, project.path)));
          break;
        }
        case "template_list": {
          state.templates = msg.templates || [];
          const container = document.querySelector("#prompt-templates");
          container.replaceChildren();
          state.templates.forEach(template => {
            const button = document.createElement("button");
            button.type = "button"; button.className = "quiet"; button.textContent = template.label;
            button.addEventListener("click", () => window.claudePhone.setPrompt(template.prompt));
            container.append(button);
          });
          break;
        }
        case "history":
          if (msg.sessionId === state.sessionId) renderHistory(msg.messages);
          break;
        case "health":
          if (msg.sessionId === state.sessionId) {
            connection.textContent = msg.state === "healthy" ? "已连接" : (msg.state === "stalled" ? "会话可能卡住" : "会话无响应");
          }
          break;
        case "queued":
          append("queued", `已排队（第 ${msg.position} 条）`);
          break;
        case "dequeued":
          break;
        case "thinking":
          state.assistantChunk = null;
          break;
        case "tool_use":
          flushTokens();
          state.assistantChunk = null;
          append("assistant", `🔧 ${msg.tool}${msg.input ? `\n${msg.input}` : ""}`);
          break;
        case "token":
          queueToken(msg.content);
          break;
        case "done":
          flushTokens();
          state.assistantChunk = null;
          break;
        case "session_stopped":
          if (state.sessionId === msg.sessionId) {
            state.sessionId = "";
            document.querySelector("#view-title").textContent = "新会话";
            document.querySelector("#stop-session").disabled = true;
          }
          send({ type: "control", action: "list_sessions", limit: 100 });
          break;
        case "error":
          append("error", `${msg.code}: ${msg.message}`);
          break;
      }
    };
  }

  const createSession = () => send({ type: "control", action: "create_session", name: platform === "mobile" ? "Android 会话" : "Mac 会话", workingDir: document.querySelector("#create-project").value, permissionMode: document.querySelector("#create-permission").value });
  document.querySelector("#new-session").addEventListener("click", createSession);
  document.querySelector("#new-session-mobile").addEventListener("click", createSession);
  document.querySelector("#mobile-session-select").addEventListener("change", event => {
    const session = state.sessions.find(item => item.sessionId === event.target.value);
    if (session) selectSession(session.sessionId, session.name);
  });
  document.querySelector("#stop-session").addEventListener("click", () => {
    if (state.sessionId) send({ type: "control", action: "stop_session", sessionId: state.sessionId });
  });
  document.querySelector("#open-settings").addEventListener("click", () => {
    if (window.AndroidBridge?.openSettings) AndroidBridge.openSettings();
  });
  document.querySelector("#voice-mobile").addEventListener("click", () => {
    if (window.AndroidBridge?.startVoice) AndroidBridge.startVoice();
  });
  document.querySelector("#composer").addEventListener("submit", event => {
    event.preventDefault();
    const input = document.querySelector("#prompt");
    const content = input.value.trim();
    if (!content) return;
    append("user", content);
    state.assistantChunk = null;
    send({ type: "text", content });
    input.value = "";
  });
  document.querySelector("#prompt").addEventListener("keydown", event => {
    if (event.metaKey && event.key === "Enter") document.querySelector("#composer").requestSubmit();
  });
  connect();
})();
