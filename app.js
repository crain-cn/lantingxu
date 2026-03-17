(function () {
  const storyCanvas = document.getElementById("storyCanvas");
  const suspenseText = document.getElementById("suspenseText");
  const btnContinue = document.getElementById("btnContinue");
  const btnReset = document.getElementById("btnReset");
  const statusEl = document.getElementById("status");
  const newRoleInput = document.getElementById("newRoleInput");
  const authStatus = document.getElementById("authStatus");
  const btnLogin = document.getElementById("btnLogin");
  const btnLogout = document.getElementById("btnLogout");

  const TOKEN_KEY = "secondme_access_token";
  const REFRESH_KEY = "secondme_refresh_token";
  const TOKEN_TIME_KEY = "secondme_token_time";
  const EXPIRES_KEY = "secondme_expires_in";

  let segments = [];
  let currentSuspense = "";
  let apiConfig = { clientId: "", redirectUri: "" };

  function getToken() {
    return sessionStorage.getItem(TOKEN_KEY);
  }

  function getRefreshToken() {
    return sessionStorage.getItem(REFRESH_KEY);
  }

  async function refreshAccessToken() {
    const refresh = getRefreshToken();
    if (!refresh) return null;
    try {
      const r = await fetch("/api/oauth/refresh", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
      });
      const data = await r.json();
      if (data.accessToken) {
        sessionStorage.setItem(TOKEN_KEY, data.accessToken);
        sessionStorage.setItem(TOKEN_TIME_KEY, String(Date.now()));
        sessionStorage.setItem(EXPIRES_KEY, String(data.expiresIn || 7200));
        if (data.refreshToken) sessionStorage.setItem(REFRESH_KEY, data.refreshToken);
        return data.accessToken;
      }
    } catch (_) {}
    return null;
  }

  async function ensureToken() {
    let token = getToken();
    if (token) {
      const t0 = Number(sessionStorage.getItem(TOKEN_TIME_KEY)) || 0;
      const exp = Number(sessionStorage.getItem(EXPIRES_KEY)) || 7200;
      if ((Date.now() - t0) / 1000 > exp - 300) token = await refreshAccessToken();
    }
    return token || (await refreshAccessToken());
  }

  function updateAuthUI() {
    const token = getToken();
    if (token) {
      authStatus.textContent = "已登录";
      authStatus.classList.add("ready");
      if (btnLogin) btnLogin.classList.add("hidden");
      if (btnLogout) btnLogout.classList.remove("hidden");
    } else {
      authStatus.textContent = "未登录";
      authStatus.classList.remove("ready");
      if (btnLogin) btnLogin.classList.remove("hidden");
      if (btnLogout) btnLogout.classList.add("hidden");
    }
  }

  async function loadConfig() {
    try {
      const r = await fetch("/api/config");
      apiConfig = await r.json();
    } catch (_) {}
  }

  function doLogin() {
    const { clientId, redirectUri } = apiConfig;
    if (!clientId || !redirectUri) {
      setStatus("服务未配置 OAuth，请先配置后端环境变量", "error");
      return;
    }
    const state = Math.random().toString(36).slice(2);
    sessionStorage.setItem("oauth_state", state);
    const url =
      "https://go.second.me/oauth/?" +
      new URLSearchParams({
        client_id: clientId,
        redirect_uri: redirectUri,
        response_type: "code",
        state,
      });
    location.href = url;
  }

  function doLogout() {
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(REFRESH_KEY);
    sessionStorage.removeItem(TOKEN_TIME_KEY);
    sessionStorage.removeItem(EXPIRES_KEY);
    updateAuthUI();
    setStatus("已退出");
    setTimeout(() => setStatus(""), 2000);
  }

  function getControls() {
    const time = document.querySelector('input[name="time"]:checked')?.value || "continue";
    const view = document.querySelector('input[name="view"]:checked')?.value || "keep";
    const style = document.querySelector('input[name="style"]:checked')?.value || "any";
    const newRole = (newRoleInput?.value || "").trim();
    return { time, view, style, newRole };
  }

  function syncChipActive() {
    document.querySelectorAll(".chip-group .chip").forEach((label) => {
      const input = label.querySelector('input[type="radio"]');
      label.classList.toggle("active", input?.checked ?? false);
    });
  }
  document.querySelectorAll(".chip-group .chip").forEach((label) => {
    label.addEventListener("click", () => setTimeout(syncChipActive, 0));
  });
  syncChipActive();

  function renderSegment(seg) {
    const div = document.createElement("div");
    div.className = "segment";
    const meta = [];
    if (seg.timeJump) meta.push(`时间：${seg.timeJump}`);
    if (seg.viewpoint) meta.push(`视角：${seg.viewpoint}`);
    if (seg.newRole) meta.push(`新角色：${seg.newRole}`);
    if (seg.style) meta.push(`风格：${seg.style}`);
    div.innerHTML =
      (meta.length ? `<div class="segment-meta">${meta.map((m) => `<span class="tag">${m}</span>`).join("")}</div>` : "") +
      `<div class="segment-text">${escapeHtml(seg.text)}</div>`;
    return div;
  }

  function escapeHtml(s) {
    const div = document.createElement("div");
    div.textContent = s;
    return div.innerHTML;
  }

  function render() {
    storyCanvas.innerHTML = "";
    segments.forEach((seg) => storyCanvas.appendChild(renderSegment(seg)));
    suspenseText.textContent = currentSuspense || "（暂无）";
  }

  function setStatus(msg, type = "") {
    statusEl.textContent = msg;
    statusEl.className = "status" + (type ? " " + type : "");
  }

  function buildPrompt(controls) {
    const styleLabels = { "sci-fi": "科幻", warm: "温情", mystery: "悬疑", philosophy: "哲思", any: "任意" };
    const storyText =
      segments.length > 0
        ? segments.map((s) => s.text).join("\n\n")
        : "（故事尚未开始，请从第一段写起。）";
    const instructions = [
      "时间：" + (controls.time === "jump" ? "跳跃" : "顺承"),
      "视角：" + (controls.view === "switch" ? "切换" : "保持"),
      "新角色：" + (controls.newRole || "无"),
      "风格：" + (styleLabels[controls.style] || "任意"),
    ].join("；");
    return `你是一位故事续写助手。根据以下「已有故事」和「上文悬念」，按「本段要求」续写下一段。要求：承接悬念、自然连贯，只输出一段正文，不要解释或前缀。

【已有故事】
${storyText}

【上文悬念】
${currentSuspense || "（无）"}

【本段要求】
${instructions}

请只输出续写的一段正文：`;
  }

  function extractSuspense(newText) {
    const t = (newText || "").trim();
    if (t.length < 15) return "";
    const sentences = t.match(/[^。！？]+[。！？]/g) || [];
    const last = sentences[sentences.length - 1] || t.slice(-50);
    return last.trim().slice(0, 80) + (last.length > 80 ? "…" : "");
  }

  async function streamSecondMeChat(message) {
    const token = await ensureToken();
    if (!token) return { ok: false, error: "请先登录 SecondMe" };
    const r = await fetch("/api/chat/stream", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({ message }),
    });
    if (r.status === 401) return { ok: false, error: "登录已过期，请重新登录" };
    if (!r.ok) {
      const j = await r.json().catch(() => ({}));
      return { ok: false, error: j.message || `请求失败 ${r.status}` };
    }
    return { ok: true, stream: r.body };
  }

  function parseSSE(stream) {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    return {
      async *[Symbol.asyncIterator]() {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() || "";
          for (const line of lines) {
            if (line.startsWith("data: ")) {
              const data = line.slice(6);
              if (data === "[DONE]") return;
              try {
                const j = JSON.parse(data);
                const content = j?.choices?.[0]?.delta?.content;
                if (content) yield content;
              } catch (_) {}
            }
          }
        }
      },
    };
  }

  async function onContinue() {
    const controls = getControls();
    const token = await ensureToken();
    if (!token) {
      setStatus("请先登录 SecondMe", "error");
      return;
    }
    btnContinue.disabled = true;
    setStatus("正在续写…", "generating");

    const prompt = buildPrompt(controls);
    const result = await streamSecondMeChat(prompt);
    if (!result.ok) {
      setStatus(result.error || "续写失败", "error");
      btnContinue.disabled = false;
      return;
    }

    const styleLabels = { "sci-fi": "科幻", warm: "温情", mystery: "悬疑", philosophy: "哲思", any: "" };
    let streamedText = "";
    const segmentDiv = document.createElement("div");
    segmentDiv.className = "segment segment-streaming";
    segmentDiv.innerHTML =
      (controls.time === "jump" || controls.view === "switch" || controls.newRole || controls.style !== "any"
        ? `<div class="segment-meta">${[
            controls.time === "jump" && "时间：跳跃",
            controls.view === "switch" && "视角：切换",
            controls.newRole && "新角色：" + controls.newRole,
            styleLabels[controls.style] && "风格：" + styleLabels[controls.style],
          ]
            .filter(Boolean)
            .map((m) => `<span class="tag">${m}</span>`)
            .join("")}</div>`
        : "") + '<div class="segment-text"></div>';
    storyCanvas.appendChild(segmentDiv);
    const textEl = segmentDiv.querySelector(".segment-text");

    try {
      for await (const chunk of parseSSE(result.stream)) {
        streamedText += chunk;
        textEl.textContent = streamedText;
        storyCanvas.scrollTop = storyCanvas.scrollHeight;
      }
    } catch (e) {
      setStatus("流式读取异常：" + e.message, "error");
    }

    const finalText = streamedText.trim() || "（未生成内容）";
    segments.push({
      text: finalText,
      timeJump: controls.time === "jump" ? "跳跃" : undefined,
      viewpoint: controls.view === "switch" ? "切换" : undefined,
      newRole: controls.newRole || undefined,
      style: styleLabels[controls.style] || undefined,
    });
    currentSuspense = extractSuspense(finalText);

    newRoleInput.value = "";
    document.querySelector('input[name="view"][value="keep"]')?.click();
    syncChipActive();
    render();
    btnContinue.disabled = false;
    setStatus("已续写一段");
    setTimeout(() => setStatus(""), 2000);
  }

  function onReset() {
    if (segments.length && !confirm("确定要清空当前故事并重新开始吗？")) return;
    segments = [];
    currentSuspense = "";
    render();
    setStatus("已重新开始");
    setTimeout(() => setStatus(""), 2000);
  }

  btnContinue.addEventListener("click", onContinue);
  btnReset.addEventListener("click", onReset);
  if (btnLogin) btnLogin.addEventListener("click", doLogin);
  if (btnLogout) btnLogout.addEventListener("click", doLogout);

  (async function init() {
    await loadConfig();
    updateAuthUI();
    render();
  })();
})();
