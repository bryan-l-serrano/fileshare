let currentPath = '';

async function loadFiles() {
  const res = await fetch('/api/files?path=' + encodeURIComponent(currentPath));
  const files = await res.json();
  document.getElementById('loading').style.display = 'none';
  renderBreadcrumb();
  renderFiles(files);
}

const zone = document.getElementById('drop-zone');
const input = document.getElementById('upload-btn');
const status = document.getElementById('status');

if (zone && input && status) {
  zone.addEventListener('click', () => input.click());
  zone.addEventListener('dragover', e => { e.preventDefault(); zone.classList.add('hover'); });
  zone.addEventListener('dragleave', () => zone.classList.remove('hover'));
  zone.addEventListener('drop', e => {
    e.preventDefault();
    zone.classList.remove('hover');
    if (e.dataTransfer.files.length) uploadMultiple(e.dataTransfer.files);
  });
  input.addEventListener('change', () => { if (input.files.length) uploadMultiple(input.files); });

  async function uploadMultiple(files) {
    for (const file of files) {
      status.textContent = 'Uploading: ' + file.name + '...';
      const form = new FormData();
      form.append('file', file);
      form.append('dir', currentPath);
      const res = await fetch('/api/upload', { method: 'POST', body: form });
      const data = await res.json();
      if (!res.ok) { status.textContent = 'Error: ' + (data.detail || res.statusText); return; }
      status.textContent = 'Uploaded: ' + data.filename + ' (' + data.size + ' bytes)';
    }
    loadFiles();
  }
}

function renderBreadcrumb() {
  if (!currentPath) {
    document.getElementById('breadcrumb').innerHTML = '<a class="drop-target" href="#" onclick="navigateTo(\'\')">/</a>';
    return;
  }
  let parts = currentPath.split('/').filter(Boolean);
  let html = '<a class="drop-target" href="#" onclick="navigateTo(\'\')">/</a>';
  let accum = '';
  for (const part of parts) {
    accum = accum ? accum + '/' + part : part;
    html += ' / <a class="drop-target" href="#" onclick="navigateTo(\'' + accum + '\')">' + escapeHtml(part) + '</a>';
  }
  document.getElementById('breadcrumb').innerHTML = html;
}

function navigateTo(path) {
  currentPath = path;
  loadFiles();
}

function renderFiles(files) {
  const content = document.getElementById('content');
  if (files.length === 0) {
    content.innerHTML = '<p class="empty">Empty directory.</p>';
    return;
  }

  let html = '<table><thead><tr><th>Name</th><th>Size</th><th>Actions</th></tr></thead><tbody>';
  for (const f of files) {
    const icon = f.is_dir ? '<span class="folder-icon">&#128193;</span> ' : '<span class="file-icon">&#128196;</span> ';
    const size = f.is_dir ? '-' : formatSize(f.size);
    let actions = '';
    if (f.is_dir) {
      actions = '<a href="#" onclick="navigateTo(\'' + escapeHtml(f.saved_as) + '\')">Open</a>';
      actions += ' <button class="delete-btn" onclick="deleteFolder(\'' + escapeJs(f.saved_as) + '\')">Delete</button>';
    } else {
      actions = '<a href="/download/' + escapeHtml(f.saved_as) + '">Download</a>';
      actions += ' <button class="delete-btn" onclick="deleteFile(\'' + escapeJs(f.saved_as) + '\')">Delete</button>';
    }
    const dragClass = f.is_dir ? 'drop-target' : 'drag-source';
    const draggableAttr = f.is_dir ? '' : 'draggable="true"';
    html += '<tr class="' + dragClass + '" data-name=\'' + escapeJs(f.name) + '\' data-saved=\'' + escapeJs(f.saved_as) + '\' ' + draggableAttr + '><td>' + icon + escapeHtml(f.name) + '</td><td>' + size + '</td><td>' + actions + '</td></tr>';
  }
  html += '</tbody></table>';
  content.innerHTML = html;
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

function escapeJs(str) {
  return str.replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
}

async function createFolder() {
  const name = document.getElementById('new-folder-name').value.trim();
  if (!name) return;
  const res = await fetch('/api/mkdir', {
    method: 'POST',
    body: new URLSearchParams({ name, path: currentPath })
  });
  if (!res.ok) { alert('Could not create folder'); return; }
  document.getElementById('new-folder-name').value = '';
  loadFiles();
}

async function deleteFile(savedAs) {
  if (!confirm('Delete ' + savedAs + '?')) return;
  const res = await fetch('/api/delete', {
    method: 'POST',
    body: new URLSearchParams({ saved_as: savedAs, path: currentPath })
  });
  if (!res.ok) { alert('Could not delete file'); return; }
  loadFiles();
}

async function deleteFolder(folderName) {
  if (!confirm('Delete folder ' + folderName + '? This will delete all contents.')) return;
  const res = await fetch('/api/rmdir', {
    method: 'POST',
    body: new URLSearchParams({ name: folderName, path: currentPath })
  });
  if (!res.ok) { alert('Could not delete folder'); return; }
  loadFiles();
}

document.addEventListener('DOMContentLoaded', () => {
  const content = document.getElementById('content');
  const breadcrumb = document.getElementById('breadcrumb');

  content.addEventListener('dragstart', e => {
    const row = e.target.closest('.drag-source');
    if (!row) return;
    e.dataTransfer.setData('text/plain', JSON.stringify({
      name: row.dataset.name,
      saved_as: row.dataset.saved,
      src_path: currentPath
    }));
    e.dataTransfer.effectAllowed = 'move';
  });

  content.addEventListener('dragover', e => {
    e.preventDefault();
    const row = e.target.closest('.drop-target');
    if (!row) {
      content.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drag-over'));
      return;
    }
    content.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drag-over'));
    row.classList.add('drag-over');
  });

  content.addEventListener('dragleave', e => {
    content.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drag-over'));
  });

  content.addEventListener('drop', e => {
    e.preventDefault();
    content.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drag-over'));
    const row = e.target.closest('.drop-target');
    if (!row) return;
    const data = JSON.parse(e.dataTransfer.getData('text/plain'));
    const targetSaved = row.dataset.saved;
    fetch('/api/move', {
      method: 'POST',
      body: new URLSearchParams({
        saved_as: data.saved_as,
        src_path: currentPath,
        dst_path: targetSaved
      })
    }).then(() => { loadFiles(); });
  });

  breadcrumb.addEventListener('dragover', e => {
    e.preventDefault();
    breadcrumb.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drag-over'));
    const link = e.target.closest('.drop-target');
    if (link) link.classList.add('drag-over');
  });

  breadcrumb.addEventListener('dragleave', e => {
    breadcrumb.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drag-over'));
  });

  breadcrumb.addEventListener('drop', e => {
    e.preventDefault();
    breadcrumb.querySelectorAll('.drop-target').forEach(el => el.classList.remove('drag-over'));
    const link = e.target.closest('.drop-target');
    if (!link) return;
    const data = JSON.parse(e.dataTransfer.getData('text/plain'));
    fetch('/api/move', {
      method: 'POST',
      body: new URLSearchParams({
        saved_as: data.saved_as,
        src_path: currentPath,
        dst_path: link.dataset.path
      })
    }).then(() => { loadFiles(); });
  });
});

loadFiles();
