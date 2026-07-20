(() => {
  const state = {
    ws: null, sessionId: "", retry: 0, retryTimer: 0, statusTimer: 0, generation: 0,
    assistantChunk: null, pendingTokens: "", tokenFrame: 0,
    sessions: [], projects: [], providers: [], templates: [],
    engineReady: false, connected: false, sessionReady: false,
    draft: null, selectedSession: null, voiceBase: ""
  };
  const messages = document.querySelector("#messages");
  const connection = document.querySelector("#connection-state");
  const banner = document.querySelector("#startup-banner");
  const chat = document.querySelector("#chat-view");
  const admin = document.querySelector("#admin-view");
  const composer = document.querySelector("#composer");
  const prompt = document.querySelector("#prompt");
  const projectSelect = document.querySelector("#draft-project");
  const providerSelect = document.querySelector("#draft-provider");
  const providerLabel = document.querySelector("#provider-label");
  const permissionSelect = document.querySelector("#draft-permission");
  const sessionContext = document.querySelector("#session-context");
  const draftStatus = document.querySelector("#draft-status");
  const sendButton = composer.querySelector("button.primary");
  const params = new URLSearchParams(location.search);
  const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
  const platform = params.get("platform") || "desktop";
  const configuredWS = params.get("ws") || "";
  const deviceToken = params.get("deviceToken") || `${platform}-${token || "local"}`;
  const deviceName = params.get("deviceName") || (platform === "mobile" ? "Android" : "Mac");
  document.body.classList.add(platform);

  function newRequestID() {
    if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
    return `req-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }

  function basename(path) {
    const parts = String(path || "").replace(/\/+$/, "").split("/");
    return parts[parts.length - 1] || "项目";
  }

  function sessionNameFor(value, cwd) {
    const normalized = String(value || "").trim().replace(/\s+/g, " ");
    const title = Array.from(normalized).slice(0, 32).join("");
    if (title) return title;
    const time = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    return `${basename(cwd)} · ${time}`;
  }

  function showBanner(message) {
    banner.textContent = message || "";
    banner.hidden = !message;
  }

  function currentProvider() {
    const id = providerSelect.value || state.providers.find(item => item.available)?.id || "claude";
    return state.providers.find(item => item.id === id);
  }

  function isDraft() {
    return Boolean(state.draft);
  }

  function updateControls() {
    const engineAvailable = state.engineReady && state.connected;
    const draftReady = isDraft() && state.draft.status !== "creating" && Boolean(
      projectSelect.value && currentProvider()?.available && permissionSelect.value && prompt.value.trim()
    );
    const sessionReady = !isDraft() && state.sessionReady && Boolean(prompt.value.trim());
    prompt.disabled = !engineAvailable || (isDraft() && state.draft.status === "creating");
    sendButton.disabled = !engineAvailable || !(draftReady || sessionReady);
    document.querySelector("#new-session").disabled = !engineAvailable;
    document.querySelector("#new-session-mobile").disabled = !engineAvailable;
    projectSelect.disabled = !isDraft() || state.draft?.status === "creating";
    providerSelect.disabled = !isDraft() || state.draft?.status === "creating";
    permissionSelect.disabled = !engineAvailable || state.draft?.status === "creating";
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
    if (isDraft() && prompt.value.trim() && !window.confirm("当前新会话草稿尚未发送，确认离开？")) return;
    chat.hidden = true;
    admin.hidden = false;
    document.body.classList.toggle("admin-mode", true);
    document.querySelector("#show-admin").classList.add("active");
    document.querySelector("#view-title").textContent = "管理与诊断";
    if (window.codeAfar.refreshAdmin) {
      try { await window.codeAfar.refreshAdmin(); }
      catch (error) { showBanner(`管理数据加载失败：${error.message}`); }
    }
  }

  const bridge = {
    adminToken: token,
    state,
    showChat,
    showAdmin,
    setPrompt(value) { prompt.value = value || ""; prompt.dispatchEvent(new Event("input")); prompt.focus(); },
    setVoiceText(value) {
      const separator = state.voiceBase && value && !/\s$/.test(state.voiceBase) ? " " : "";
      prompt.value = `${state.voiceBase}${separator}${value || ""}`;
      prompt.dispatchEvent(new Event("input"));
      prompt.focus();
    },
    setVoiceState(next, message) {
      const button = document.querySelector("#voice-mobile");
      button.dataset.state = next || "idle";
      button.setAttribute("aria-label", message || (next === "listening" ? "停止语音输入" : "开始语音输入"));
      if (message) draftStatus.textContent = message;
    }
  };
  window.codeAfar = bridge;
  window.claudePhone = window.codeAfar;

  function legacyCopy(text) {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.append(area);
    area.select();
    area.setSelectionRange(0, area.value.length);
    const copied = document.execCommand("copy");
    area.remove();
    if (!copied) throw new Error("copy command was rejected");
  }

  async function writeClipboard(text) {
    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        return;
      } catch (_) {
        // WKWebView may expose Clipboard API while denying it; use the user-gesture fallback.
      }
    }
    legacyCopy(text);
  }

  function copyButton(content) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "message-copy";
    button.setAttribute("aria-label", "复制消息");
    button.title = "复制消息";
    button.addEventListener("click", async event => {
      event.stopPropagation();
      try {
        await writeClipboard(content.textContent);
        button.dataset.copied = "true";
        window.setTimeout(() => { delete button.dataset.copied; }, 1500);
      } catch (_) {
        showBanner("复制失败，请选择文本后按 ⌘C。");
      }
    });
    return button;
  }

  function append(role, text) {
    messages.querySelector(".empty")?.remove();
    const node = document.createElement("div");
    node.className = `message ${role}`;
    const content = document.createElement("span");
    content.className = "message-content";
    content.textContent = text;
    node.append(content, copyButton(content));
    messages.append(node);
    while (messages.children.length > 500) messages.firstElementChild?.remove();
    messages.scrollTop = messages.scrollHeight;
    return content;
  }

  function renderDraftEmpty() {
    messages.replaceChildren();
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.innerHTML = "<strong class=\"empty-title\">开始新的开发任务</strong><p>选择工作目录，然后告诉 CodeAfar 要做什么。</p><span>Return 发送 · Shift-Return 换行</span>";
    messages.append(empty);
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

  function renderProjectOptions(preferred = "") {
    const selected = preferred || projectSelect.value;
    projectSelect.replaceChildren(new Option(state.projects.length ? "选择项目" : "选择项目", ""));
    state.projects.forEach(project => {
      projectSelect.add(new Option(`${project.name} · ${project.path}`, project.path));
    });
    if (platform === "desktop") projectSelect.add(new Option("选择文件夹…", "__choose__"));
    if (state.projects.some(project => project.path === selected)) projectSelect.value = selected;
    else if (state.projects.length === 1) projectSelect.value = state.projects[0].path;
    updateControls();
  }

  function renderPermissions(preferred = "") {
    const descriptor = currentProvider();
    const selected = preferred || permissionSelect.value;
    permissionSelect.replaceChildren();
    (descriptor?.permissions || []).forEach(option => {
      const label = option.dangerous ? `${option.label} ⚠` : option.label;
      permissionSelect.add(new Option(label, option.id));
    });
    if ([...permissionSelect.options].some(option => option.value === selected)) permissionSelect.value = selected;
    else if ([...permissionSelect.options].some(option => option.value === "default")) permissionSelect.value = "default";
    updateControls();
  }

  function renderProviders() {
    const selected = providerSelect.value || "claude";
    providerSelect.replaceChildren();
    state.providers.forEach(descriptor => providerSelect.add(new Option(descriptor.name, descriptor.id)));
    if ([...providerSelect.options].some(option => option.value === selected)) providerSelect.value = selected;
    const only = state.providers.length === 1 ? state.providers[0] : null;
    providerSelect.hidden = Boolean(only);
    providerLabel.hidden = !only;
    providerLabel.textContent = only?.name || "";
    renderPermissions();
  }

  async function chooseProjectDirectory() {
    if (platform !== "desktop" || !window.codeAfarNative?.chooseDirectory) {
      showBanner("请先在 Mac 上添加工作目录。");
      return;
    }
    try {
      const path = await window.codeAfarNative.chooseDirectory();
      if (!path) return;
      const response = await fetch("/desktop/projects", {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path })
      });
      if (!response.ok) throw new Error((await response.text()).trim() || `状态 ${response.status}`);
      const project = await response.json();
      state.projects = state.projects.filter(item => item.path !== project.path).concat(project);
      renderProjectOptions(project.path);
      showBanner("");
    } catch (error) {
      showBanner(`无法添加工作目录：${error.message}`);
      renderProjectOptions();
    }
  }

  function beginDraft() {
    if (isDraft() && prompt.value.trim() && !window.confirm("当前新会话草稿尚未发送，确认丢弃？")) return;
    state.sessionId = "";
    state.sessionReady = false;
    state.selectedSession = null;
    state.draft = { status: "draft", requestId: newRequestID(), firstPrompt: "" };
    prompt.value = "";
    prompt.placeholder = "告诉 CodeAfar 要做什么…";
    projectSelect.hidden = false;
    providerLabel.hidden = state.providers.length !== 1;
    providerSelect.hidden = state.providers.length === 1;
    sessionContext.hidden = true;
    permissionSelect.disabled = false;
    draftStatus.textContent = "";
    document.querySelector("#view-title").textContent = "新会话";
    document.querySelector("#stop-session").disabled = true;
    resetStream();
    renderDraftEmpty();
    renderSessions();
    showChat();
    updateControls();
  }

  function selectSession(sessionId, name) {
    if (isDraft() && prompt.value.trim() && !window.confirm("当前新会话草稿尚未发送，确认离开？")) return;
    state.draft = null;
    state.sessionId = sessionId;
    state.selectedSession = state.sessions.find(item => item.sessionId === sessionId) || null;
    state.sessionReady = false;
    prompt.value = "";
    prompt.placeholder = "输入消息…";
    projectSelect.hidden = true;
    providerSelect.hidden = true;
    providerLabel.hidden = true;
    sessionContext.hidden = false;
    sessionContext.textContent = state.selectedSession ? `${state.selectedSession.provider === "claude" ? "Claude Code" : state.selectedSession.provider} · ${state.selectedSession.cwd}` : "";
    renderPermissions(state.selectedSession?.permissionMode || "default");
    resetStream();
    showChat();
    document.querySelector("#view-title").textContent = name || "会话";
    document.querySelector("#stop-session").disabled = false;
    messages.replaceChildren();
    send({ type: "control", action: "select_session", sessionId });
    send({ type: "control", action: "load_history", sessionId, limit: 500 });
    renderSessions();
    updateControls();
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

  function sendCreateRequest() {
    if (!state.draft?.firstPrompt) return false;
    return send({
      type: "control", action: "create_session", name: sessionNameFor(state.draft.firstPrompt, projectSelect.value),
      workingDir: projectSelect.value, provider: providerSelect.value || "claude",
      permissionMode: permissionSelect.value, requestId: state.draft.requestId
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
    if (state.draft?.status === "creating") {
      state.draft.status = "failed";
      draftStatus.textContent = "创建失败，可再次发送重试";
      showBanner(msg.message || "会话创建失败");
      updateControls();
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
        send({ type: "control", action: "list_providers" });
        send({ type: "control", action: "list_templates" });
        if (state.sessionId) {
          send({ type: "control", action: "select_session", sessionId: state.sessionId });
          send({ type: "control", action: "load_history", sessionId: state.sessionId, limit: 500 });
        } else if (state.draft?.status === "creating") {
          sendCreateRequest();
        }
        break;
      case "session_list":
        state.sessions = msg.sessions || [];
        renderSessions();
        break;
      case "session_created": {
        if (state.draft && msg.requestId && msg.requestId !== state.draft.requestId) break;
        const firstPrompt = state.draft?.firstPrompt || "";
        state.draft = null;
        state.sessionId = msg.sessionId;
        state.sessionReady = true;
        state.selectedSession = {
          sessionId: msg.sessionId, name: msg.name, cwd: msg.cwd, provider: msg.provider,
          permissionMode: msg.permissionMode, model: msg.model
        };
        prompt.disabled = false;
        prompt.value = "";
        prompt.placeholder = "输入消息…";
        projectSelect.hidden = true;
        providerSelect.hidden = true;
        providerLabel.hidden = true;
        sessionContext.hidden = false;
        sessionContext.textContent = `${msg.provider === "claude" ? "Claude Code" : msg.provider} · ${msg.cwd}`;
        renderPermissions(msg.permissionMode);
        resetStream();
        showChat();
        messages.replaceChildren();
        document.querySelector("#view-title").textContent = msg.name || "新会话";
        document.querySelector("#stop-session").disabled = false;
        draftStatus.textContent = "";
        showBanner("");
        if (firstPrompt) {
          append("user", firstPrompt);
          send({ type: "text", content: firstPrompt });
        }
        send({ type: "control", action: "list_sessions", limit: 100 });
        updateControls();
        break;
      }
      case "project_list":
        state.projects = msg.projects || [];
        renderProjectOptions();
        break;
      case "provider_list":
        state.providers = msg.providers || [];
        renderProviders();
        break;
      case "template_list": {
        state.templates = msg.templates || [];
        const container = document.querySelector("#prompt-templates");
        container.replaceChildren();
        state.templates.forEach(template => {
          const button = document.createElement("button");
          button.type = "button";
          button.className = "quiet";
          button.textContent = template.label;
          button.addEventListener("click", () => window.codeAfar.setPrompt(template.prompt));
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
      case "permission_changed":
        if (msg.sessionId === state.sessionId) {
          if (!msg.pending) {
            permissionSelect.value = msg.permissionMode;
            if (state.selectedSession) state.selectedSession.permissionMode = msg.permissionMode;
          }
          draftStatus.textContent = msg.pending ? "将在本轮结束后应用" : "权限已更新";
        }
        break;
      case "session_stopped":
        if (state.sessionId === msg.sessionId) {
          state.sessionId = "";
          state.sessionReady = false;
          connection.textContent = state.connected ? "已连接" : "重新连接中";
          beginDraft();
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

  document.querySelector("#new-session").addEventListener("click", beginDraft);
  document.querySelector("#new-session-mobile").addEventListener("click", beginDraft);
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
    const button = document.querySelector("#voice-mobile");
    if (!["listening", "processing"].includes(button.dataset.state)) state.voiceBase = prompt.value;
    if (window.AndroidBridge?.startVoice) AndroidBridge.startVoice();
  });
  projectSelect.addEventListener("change", () => {
    if (projectSelect.value === "__choose__") chooseProjectDirectory();
    else updateControls();
  });
  providerSelect.addEventListener("change", () => renderPermissions());
  permissionSelect.addEventListener("change", () => {
    const option = currentProvider()?.permissions?.find(item => item.id === permissionSelect.value);
    if (option?.dangerous && !window.confirm("完全访问会跳过常规权限限制。只应在隔离环境或完全信任的目录中使用，确认继续？")) {
      permissionSelect.value = state.selectedSession?.permissionMode || "default";
      return;
    }
    if (!isDraft() && state.sessionId) {
      send({ type: "control", action: "set_permission_mode", sessionId: state.sessionId, permissionMode: permissionSelect.value });
    }
    updateControls();
  });
  prompt.addEventListener("input", updateControls);
  composer.addEventListener("submit", event => {
    event.preventDefault();
    const content = prompt.value.trim();
    if (!content || !state.engineReady || !state.connected) return;
    if (isDraft()) {
      if (!projectSelect.value || !currentProvider()?.available || !permissionSelect.value || state.draft.status === "creating") return;
      state.draft = { ...state.draft, status: "creating", firstPrompt: content };
      draftStatus.textContent = "正在创建会话…";
      showBanner("");
      updateControls();
      if (!sendCreateRequest()) {
        state.draft.status = "failed";
        draftStatus.textContent = "连接中断，可再次发送重试";
        updateControls();
      }
      return;
    }
    if (!state.sessionReady) return;
    append("user", content);
    state.assistantChunk = null;
    send({ type: "text", content });
    prompt.value = "";
    updateControls();
  });
  prompt.addEventListener("keydown", event => {
    if (event.isComposing || event.keyCode === 229) return;
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      composer.requestSubmit();
    }
  });
  document.addEventListener("keydown", event => {
    if (!event.metaKey) return;
    if (event.key.toLowerCase() === "n") { event.preventDefault(); beginDraft(); }
    if (event.key === ",") { event.preventDefault(); showAdmin(); }
  });

  beginDraft();
  setComposerEnabled(false);
  bootstrapDesktop();
})();
