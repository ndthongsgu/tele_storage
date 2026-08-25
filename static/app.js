document.addEventListener('DOMContentLoaded', () => {
  const serverStatusText = document.getElementById('serverStatusText');
  const serverStatusBadge = document.getElementById('serverStatusBadge');

  // Check API Health
  checkHealth();

  // Sidebar navigation scrolling & active class
  const navItems = document.querySelectorAll('.nav-item');
  navItems.forEach(item => {
    item.addEventListener('click', (e) => {
      e.preventDefault();
      navItems.forEach(n => n.classList.remove('active'));
      item.classList.add('active');
      const targetId = item.getAttribute('data-target');
      const targetEl = document.getElementById(targetId);
      if (targetEl) {
        targetEl.scrollIntoView({ behavior: 'smooth' });
      }
    });
  });

  async function checkHealth() {
    try {
      const res = await fetch('/health');
      if (res.ok) {
        serverStatusText.textContent = 'API Status: Online';
        serverStatusBadge.querySelector('.status-dot').style.background = '#34d399';
      } else {
        serverStatusText.textContent = 'API Status: Degraded';
        serverStatusBadge.querySelector('.status-dot').style.background = '#f59e0b';
      }
    } catch (e) {
      serverStatusText.textContent = 'API Status: Offline';
      serverStatusBadge.querySelector('.status-dot').style.background = '#f87171';
    }
  }

  async function parseResponseData(res) {
    const text = await res.text();
    try {
      return JSON.parse(text);
    } catch {
      return { status: 'error', message: text || `HTTP ${res.status} ${res.statusText}` };
    }
  }

  // 1. Upload Form Tester
  const formUpload = document.getElementById('formUpload');
  const uploadFileInput = document.getElementById('uploadFileInput');
  const responseUpload = document.getElementById('responseUpload');
  const resStatusUpload = document.getElementById('resStatusUpload');
  const resBodyUpload = document.getElementById('resBodyUpload');

  formUpload.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!uploadFileInput.files.length) return;

    const formData = new FormData();
    formData.append('file', uploadFileInput.files[0]);

    resStatusUpload.textContent = 'Sending POST /api/upload...';
    resBodyUpload.textContent = 'Uploading file to Telegram...';
    responseUpload.classList.remove('hidden');

    try {
      const res = await fetch('/api/upload', {
        method: 'POST',
        body: formData
      });
      const data = await parseResponseData(res);

      resStatusUpload.className = `res-status ${res.ok ? '' : 'error'}`;
      resStatusUpload.textContent = `HTTP ${res.status} ${res.statusText}`;
      resBodyUpload.textContent = JSON.stringify(data, null, 2);
    } catch (err) {
      resStatusUpload.className = 'res-status error';
      resStatusUpload.textContent = 'Error sending request';
      resBodyUpload.textContent = err.toString();
    }
  });

  // 2. Download Form Tester
  const formDownload = document.getElementById('formDownload');

  const dlFileIdInput = document.getElementById('dlFileIdInput');
  const dlFileNameInput = document.getElementById('dlFileNameInput');
  const responseDownload = document.getElementById('responseDownload');
  const resStatusDownload = document.getElementById('resStatusDownload');
  const resBodyDownload = document.getElementById('resBodyDownload');

  formDownload.addEventListener('submit', (e) => {
    e.preventDefault();
    const fileId = dlFileIdInput.value.trim();
    const fileName = dlFileNameInput.value.trim();

    if (!fileId) return;

    let url = `/api/download?file_id=${encodeURIComponent(fileId)}`;
    if (fileName) {
      url += `&file_name=${encodeURIComponent(fileName)}`;
    }

    const fullUrl = window.location.origin + url;

    resStatusDownload.className = 'res-status';
    resStatusDownload.textContent = 'Generated API Link';
    resBodyDownload.innerHTML = `
      <div style="font-family: var(--font-code); font-size: 13px; margin-bottom: 10px; word-break: break-all;">
        <strong>URL:</strong> <a href="${url}" target="_blank" style="color: var(--primary);">${fullUrl}</a>
      </div>
      <div style="display: flex; gap: 10px;">
        <a href="${url}" target="_blank" class="btn-primary"><i class="fa-solid fa-arrow-up-right-from-square"></i> Open / Stream in Browser</a>
        <a href="${url}&dl=1" target="_blank" class="btn-primary danger"><i class="fa-solid fa-download"></i> Direct Download</a>
      </div>
    `;
    responseDownload.classList.remove('hidden');
  });

  // 3. Info Form Tester
  const formInfo = document.getElementById('formInfo');
  const infoFileIdInput = document.getElementById('infoFileIdInput');
  const responseInfo = document.getElementById('responseInfo');
  const resStatusInfo = document.getElementById('resStatusInfo');
  const resBodyInfo = document.getElementById('resBodyInfo');

  formInfo.addEventListener('submit', async (e) => {
    e.preventDefault();
    const fileId = infoFileIdInput.value.trim();
    if (!fileId) return;

    resStatusInfo.textContent = 'Sending GET /api/info...';
    responseInfo.classList.remove('hidden');

    try {
      const res = await fetch(`/api/info?file_id=${encodeURIComponent(fileId)}`);
      const data = await parseResponseData(res);

      resStatusInfo.className = `res-status ${res.ok ? '' : 'error'}`;
      resStatusInfo.textContent = `HTTP ${res.status} ${res.statusText}`;
      resBodyInfo.textContent = JSON.stringify(data, null, 2);
    } catch (err) {
      resStatusInfo.className = 'res-status error';
      resStatusInfo.textContent = 'Error sending request';
      resBodyInfo.textContent = err.toString();
    }
  });

  // 4. Delete Form Tester
  const formDelete = document.getElementById('formDelete');
  const delMsgIdInput = document.getElementById('delMsgIdInput');
  const responseDelete = document.getElementById('responseDelete');
  const resStatusDelete = document.getElementById('resStatusDelete');
  const resBodyDelete = document.getElementById('resBodyDelete');

  formDelete.addEventListener('submit', async (e) => {
    e.preventDefault();
    const msgId = delMsgIdInput.value.trim();
    if (!msgId) return;

    resStatusDelete.textContent = `Sending DELETE /api/messages/${msgId}...`;
    responseDelete.classList.remove('hidden');

    try {
      const res = await fetch(`/api/messages/${msgId}`, { method: 'DELETE' });
      const data = await parseResponseData(res);

      resStatusDelete.className = `res-status ${res.ok ? '' : 'error'}`;
      resStatusDelete.textContent = `HTTP ${res.status} ${res.statusText}`;
      resBodyDelete.textContent = JSON.stringify(data, null, 2);
    } catch (err) {
      resStatusDelete.className = 'res-status error';
      resStatusDelete.textContent = 'Error sending request';
      resBodyDelete.textContent = err.toString();
    }
  });
});
