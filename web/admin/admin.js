(() => {
  const chat = document.querySelector("#chat-view");
  const admin = document.querySelector("#admin-view");
  const title = document.querySelector("#view-title");
  async function refresh() {
    const token = window.claudePhone.adminToken;
    const response = await fetch("/admin/status", { headers: { Authorization: `Bearer ${token}` } });
    if (!response.ok) throw new Error(`admin status ${response.status}`);
    const { agent, devices, projects, diagnostics } = await response.json();
    document.querySelector("#metrics").innerHTML = [
      ["在线设备", agent.connectedDevices?.length || 0], ["活跃会话", agent.sessions?.length || 0],
      ["Agent", agent.agentVersion], ["Claude", agent.claudeVersion],
      ["运行时间", `${diagnostics.uptimeSeconds}s`], ["内存", `${Math.round(diagnostics.allocBytes / 1048576)} MB`],
      ["Goroutine", diagnostics.goroutines], ["平台", `${diagnostics.goos}/${diagnostics.goarch}`]
    ].map(([label, value]) => `<article><span>${label}</span><strong>${value}</strong></article>`).join("");
    document.querySelector("#admin-sessions").textContent = agent.sessions?.length ? JSON.stringify(agent.sessions, null, 2) : "暂无活跃会话";
    const deviceList = document.querySelector("#admin-devices");
    deviceList.replaceChildren();
    (devices || []).forEach(device => {
      const row = document.createElement("div");
      row.className = "device-row";
      const label = document.createElement("span");
      label.textContent = `${device.online ? "●" : "○"} ${device.name}`;
      const revoke = document.createElement("button");
      revoke.className = "quiet danger";
      revoke.textContent = "吊销";
      revoke.addEventListener("click", () => revokeDevice(device.deviceId));
      row.append(label, revoke);
      deviceList.append(row);
    });
    if (!devices?.length) deviceList.textContent = "暂无已授权设备";
    const projectList = document.querySelector("#admin-projects");
    projectList.replaceChildren();
    (projects || []).forEach(project => {
      const row = document.createElement("div");
      row.className = "project-row";
      const label = document.createElement("span");
      label.textContent = `${project.name} — ${project.path}`;
      const remove = document.createElement("button");
      remove.className = "quiet danger";
      remove.textContent = "删除";
      remove.addEventListener("click", () => deleteProject(project.projectId));
      row.append(label, remove);
      projectList.append(row);
    });
    if (!projects?.length) projectList.textContent = "尚未配置工作目录";
  }
  async function revokeDevice(deviceId) {
    const response = await fetch(`/admin/devices/${encodeURIComponent(deviceId)}`, {
      method: "DELETE",
      headers: { Authorization: `Bearer ${window.claudePhone.adminToken}` }
    });
    if (!response.ok) throw new Error(`revoke device ${response.status}`);
    await refresh();
  }
  async function deleteProject(projectId) {
    const response = await fetch(`/admin/projects/${encodeURIComponent(projectId)}`, {
      method: "DELETE", headers: { Authorization: `Bearer ${window.claudePhone.adminToken}` }
    });
    if (!response.ok) throw new Error(`delete project ${response.status}`);
    await refresh();
  }
  document.querySelector("#project-form").addEventListener("submit", async event => {
    event.preventDefault();
    const response = await fetch("/admin/projects", {
      method: "POST",
      headers: { Authorization: `Bearer ${window.claudePhone.adminToken}`, "Content-Type": "application/json" },
      body: JSON.stringify({ name: document.querySelector("#project-name").value.trim(), path: document.querySelector("#project-path").value.trim(), permission: "default" })
    });
    if (!response.ok) throw new Error(await response.text());
    event.target.reset();
    await refresh();
  });
  document.querySelector("#device-form").addEventListener("submit", async event => {
    event.preventDefault();
    const response = await fetch("/admin/devices", {
      method: "POST",
      headers: { Authorization: `Bearer ${window.claudePhone.adminToken}`, "Content-Type": "application/json" },
      body: JSON.stringify({ name: document.querySelector("#device-name").value.trim() || "Android" })
    });
    if (!response.ok) throw new Error(await response.text());
    const credential = await response.json();
    const output = document.querySelector("#new-device-token");
    output.hidden = false;
    output.textContent = `请复制到手机（只显示一次）：\n${credential.deviceToken}`;
    event.target.reset();
    await refresh();
  });
  document.querySelector("#show-admin").addEventListener("click", async () => {
    chat.hidden = true; admin.hidden = false; title.textContent = "管理与诊断";
    try { await refresh(); } catch (error) { document.querySelector("#admin-sessions").textContent = error.message; }
  });
})();
