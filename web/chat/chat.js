(() => {
  const state = {
    ws: null, sessionId: "", retry: 0, retryTimer: 0, statusTimer: 0, generation: 0,
    assistantChunk: null, pendingTokens: "", tokenFrame: 0,
    sessions: [], projects: [], templates: [], engineReady: false, connected: false, sessionReady: false
  };
  const messages = document.querySelector("#messages");
  const connection = document.querySelector("#connection-state");
  const banner = document.querySelector("#startup-banner");
  const chat = document.querySelector("#chat-view");
  const admin = document.querySelector("#admin-view");
  const composer = document.querySelector("#composer");
  const prompt = document.querySelector("#prompt");
  const params = new URLSearchParams(location.search);
  const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
  const platform = params.get("platform") || "desktop";
  const configuredWS = params.get("ws") || "";
  const deviceToken = params.get("deviceToken") || `${platform}-${token || "local"}`;
  const deviceName = params.get("deviceName") || (platform === "mobile" ? "Android" : "Mac");
  document.body.classList.add(platform);

  function showBanner(message) {
    banner.textContent = message || "";
    banner.hidden = !message;
  }

  function updateControls() {
    const canCreate = state.engineReady && state.connected;
    const canSend = canCreate && state.sessionReady;
    prompt.disabled = !canSend;
    composer.querySelector("button.primary").disabled = !canSend;
    document.querySelector("#new-session").disabled = !canCreate;
    document.querySelector("#new-session-mobile").disabled = !canCreate;
  }

  function setComposerEnabled(enabled) {
    state.engineReady = enabled;
    updateControls();
  }

  function showChat() {
    chat.hidden = false;
    admin.hidden = true;
    document.body.classList.toggle("admin-mode", false);
    document.querySelector("#show-admin").classList.remove("active");
    if (platform === "desktop") prompt.focus();
  }

  async function showAdmin() {
    if (platform === "mobile") return;
    chat.hidden = true;
    admin.hidden = false;
    document.body.classList.toggle("admin-mode", true);
    document.querySelector("#show-admin").classList.add("active");
    document.querySelector("#view-title").textContent = "管理与诊断";
    if (window.claudePhone.refreshAdmin) {
      try { await window.claudePhone.refreshAdmin(); }
      catch (error) { showBanner(`管理数据加载失败：${error.message}`); }
    }
  }

  window.claudePhone = {
    adminToken: token,
    state,
    showChat,
    showAdmin,
    setPrompt(value) { prompt.value = value || ""; prompt.focus(); }
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
    if (state.ws?.readyState !== WebSocket.OPEN) return false;
    state.ws.send(JSON.stringify(value));
    return true;
  }

  function selectSession(sessionId, name) {
    state.sessionId = sessionId;
    state.sessionReady = false;
    updateControls();
    resetStream();
    showChat();
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
      } else if (item.type === "tool_use") {
        append("tool", `🔧 ${item.tool || "工具"}${item.input ? `\n${item.input}` : ""}`);
        assistant = null;
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

  function showProtocolError(msg) {
    if (msg.code === "DEVICE_NOT_AUTHORIZED") {
      showBanner(`${msg.code}: ${msg.message}`);
      connection.textContent = "设备未授权";
      state.connected = false;
      setComposerEnabled(false);
      return;
    }
    append("error", `${msg.code}: ${msg.message}`);
  }

  function handleMessage(event) {
    let msg;
    try { msg = JSON.parse(event.data); }
    catch (error) {
      append("error", `协议消息无法解析：${error.message}`);
      return;
    }
    switch (msg.type) {
      case "hello":
        send({ type: "control", action: "list_sessions", limit: 100 });
        send({ type: "control", action: "list_projects" });
        send({ type: "control", action: "list_templates" });
        if (state.sessionId) {
          send({ type: "control", action: "select_session", sessionId: state.sessionId });
          send({ type: "control", action: "load_history", sessionId: state.sessionId, limit: 500 });
        }
        break;
      case "session_list":
        state.sessions = msg.sessions || [];
        renderSessions();
        break;
      case "session_created":
        state.sessionId = msg.sessionId;
        state.sessionReady = true;
        updateControls();
        resetStream();
        showChat();
        messages.replaceChildren();
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
          button.type = "button";
          button.className = "quiet";
          button.textContent = template.label;
          button.addEventListener("click", () => window.claudePhone.setPrompt(template.prompt));
          container.append(button);
        });
        break;
      }
      case "history":
        if (msg.sessionId === state.sessionId) {
          renderHistory(msg.messages);
          state.sessionReady = true;
          updateControls();
        }
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
        append("tool", `🔧 ${msg.tool}${msg.input ? `\n${msg.input}` : ""}`);
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
          state.sessionReady = false;
          updateControls();
          document.querySelector("#view-title").textContent = "新会话";
          document.querySelector("#stop-session").disabled = true;
          messages.replaceChildren(Object.assign(document.createElement("p"), { className: "empty", textContent: "会话已停止。" }));
        }
        send({ type: "control", action: "list_sessions", limit: 100 });
        break;
      case "error":
        showProtocolError(msg);
        break;
    }
  }

  function scheduleReconnect(generation) {
    if (generation !== state.generation) return;
    clearTimeout(state.retryTimer);
    const delay = Math.min(1000 * (2 ** state.retry++), 15000);
    state.retryTimer = setTimeout(() => connect(), delay);
  }

  function connect() {
    clearTimeout(state.retryTimer);
    const generation = ++state.generation;
    if (state.ws) {
      state.ws.onopen = state.ws.onclose = state.ws.onmessage = null;
      state.ws.close();
    }
    const scheme = location.protocol === "https:" ? "wss" : "ws";
    const endpoint = configuredWS || `${scheme}://${location.host}/ws`;
    const ws = new WebSocket(endpoint);
    state.ws = ws;
    ws.onopen = () => {
      if (generation !== state.generation) return;
      state.retry = 0;
      state.connected = true;
      connection.textContent = "已连接";
      document.querySelector("#status-dot").classList.add("online");
      showBanner("");
      setComposerEnabled(true);
      send({ type: "auth", deviceToken, deviceName });
    };
    ws.onclose = () => {
      if (generation !== state.generation) return;
      state.ws = null;
      state.connected = false;
      state.sessionReady = false;
      connection.textContent = "重新连接中";
      document.querySelector("#status-dot").classList.remove("online");
      setComposerEnabled(false);
      scheduleReconnect(generation);
    };
    ws.onerror = () => {
      if (generation === state.generation) connection.textContent = "连接失败";
    };
    ws.onmessage = handleMessage;
  }

  async function bootstrapDesktop() {
    clearTimeout(state.statusTimer);
    if (platform !== "desktop") {
      connect();
      return;
    }
    try {
      const response = await fetch("/desktop/status", { cache: "no-store" });
      if (!response.ok) throw new Error(`状态服务 ${response.status}`);
      const status = await response.json();
      if (!status.ready) {
        connection.textContent = status.paused ? "引擎已暂停" : "引擎不可用";
        showBanner(status.error || (status.paused ? "引擎已暂停，可从菜单栏恢复。" : "引擎正在启动…"));
        setComposerEnabled(false);
        state.statusTimer = setTimeout(bootstrapDesktop, 2000);
        return;
      }
      showBanner("");
      connect();
    } catch (error) {
      showBanner(`桌面服务不可用：${error.message}`);
      setComposerEnabled(false);
      state.statusTimer = setTimeout(bootstrapDesktop, 2000);
    }
  }

  const createSession = () => {
    showChat();
    send({ type: "control", action: "create_session", name: platform === "mobile" ? "Android 会话" : "Mac 会话", workingDir: document.querySelector("#create-project").value, permissionMode: document.querySelector("#create-permission").value });
  };
  document.querySelector("#new-session").addEventListener("click", createSession);
  document.querySelector("#new-session-mobile").addEventListener("click", createSession);
  document.querySelector("#show-admin").addEventListener("click", showAdmin);
  document.querySelector("#mobile-session-select").addEventListener("change", event => {
    const session = state.sessions.find(item => item.sessionId === event.target.value);
    if (session) selectSession(session.sessionId, session.name);
  });
  document.querySelector("#stop-session").addEventListener("click", () => {
    if (state.sessionId && window.confirm("确认停止当前会话？未完成的输出会中断。")) {
      send({ type: "control", action: "stop_session", sessionId: state.sessionId });
    }
  });
  document.querySelector("#open-settings").addEventListener("click", () => {
    if (window.AndroidBridge?.openSettings) AndroidBridge.openSettings();
  });
  document.querySelector("#voice-mobile").addEventListener("click", () => {
    if (window.AndroidBridge?.startVoice) AndroidBridge.startVoice();
  });
  composer.addEventListener("submit", event => {
    event.preventDefault();
    const content = prompt.value.trim();
    if (!content || !state.engineReady || !state.connected || !state.sessionReady) return;
    append("user", content);
    state.assistantChunk = null;
    send({ type: "text", content });
    prompt.value = "";
  });
  prompt.addEventListener("keydown", event => {
    if (event.metaKey && event.key === "Enter") composer.requestSubmit();
  });
  document.addEventListener("keydown", event => {
    if (!event.metaKey) return;
    if (event.key.toLowerCase() === "n") { event.preventDefault(); createSession(); }
    if (event.key === ",") { event.preventDefault(); showAdmin(); }
  });

  setComposerEnabled(false);
  showChat();
  bootstrapDesktop();
})();
