const zone = document.getElementById('drop-zone');
const input = document.getElementById('upload-btn');
const status = document.getElementById('status');

zone.addEventListener('click', () => input.click());
zone.addEventListener('dragover', e => { e.preventDefault(); zone.classList.add('hover'); });
zone.addEventListener('dragleave', () => zone.classList.remove('hover'));
zone.addEventListener('drop', e => {
  e.preventDefault();
  zone.classList.remove('hover');
  if (e.dataTransfer.files.length) upload(e.dataTransfer.files[0]);
});
input.addEventListener('change', () => { if (input.files.length) upload(input.files[0]); });

async function upload(file) {
  status.textContent = 'Uploading...';
  const form = new FormData();
  form.append('file', file);
  const res = await fetch('/api/upload', { method: 'POST', body: form });
  const data = await res.json();
  if (!res.ok) { status.textContent = 'Error: ' + (data.detail || res.statusText); return; }
  status.textContent = 'Uploaded: ' + data.filename + ' (' + data.size + ' bytes)';
}
