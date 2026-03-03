// www/index.js - SSE-enabled, smooth file list updates
let selectedFile = null;
let maxUploadMB = 200;
const POLL_MS = 3000;

const fileInput = document.getElementById('fileInput');
const fileNameDisplay = document.getElementById('fileName');
const maxSizeEl = document.getElementById('maxSizeDisplay');
const progressContainer = document.getElementById('progressContainer');
const progressBar = document.getElementById('progressBar');
const progressText = document.getElementById('progressText');
const dropArea = document.getElementById('uploadBox');
const form = document.getElementById('uploadForm');
const fileListEl = document.getElementById('fileList');

let refreshLock = false;

function makeDisplayName(name, maxLen = 36) {
    if (!name) return '';
    if (name.length <= maxLen) return name;
    const start = name.slice(0, Math.max(8, Math.floor((maxLen - 1) / 2)));
    const end = name.slice(-Math.max(8, Math.floor((maxLen - 1) / 2)));
    return `${start}…${end}`;
}

async function getMaxUploadSize() {
    try {
        const res = await fetch('/maxsize');
        if (res.ok) {
            const data = await res.json();
            maxUploadMB = data.maxUploadMB || maxUploadMB;
        }
    } catch (err) {
        // fallback ok
    } finally {
        maxSizeEl.textContent = `Max upload size: ${maxUploadMB} MB`;
    }
}

function createFileLi(filename) {
    const li = document.createElement('li');
    li.dataset.filename = filename;
    li.style.opacity = 0;
    li.style.transition = 'opacity 0.25s ease, transform 0.25s ease';

    const a = document.createElement('a');
    a.className = 'filename';
    a.href = '/download/' + encodeURIComponent(filename);
    a.textContent = makeDisplayName(filename, 40);
    a.title = filename;

    const btn = document.createElement('a');
    btn.className = 'download-btn';
    btn.href = '/download/' + encodeURIComponent(filename);
    btn.textContent = 'Download';

    li.appendChild(a);
    li.appendChild(btn);
    return li;
}

async function refreshFiles() {
    if (refreshLock) return;
    refreshLock = true;

    if (!fileListEl.children.length) {
        fileListEl.innerHTML = '<li class="placehnewer"><em>Loading…</em></li>';
    }

    try {
        const res = await fetch('/files', { cache: 'no-store' });
        if (!res.ok) throw new Error('fetch failed');
        const files = await res.json();
        if (!Array.isArray(files) || files.length === 0) {
            const existing = Array.from(fileListEl.querySelectorAll('li[data-filename]'));
            existing.forEach(li => {
                li.classList.add('fade-out');
                li.style.opacity = 0;
                setTimeout(() => li.remove(), 300);
            });
            setTimeout(() => fileListEl.innerHTML = '<li><em>No files available</em></li>', 320);
            return;
        }

        // map existing nodes
        const existingMap = {};
        Array.from(fileListEl.children).forEach(li => {
            const fn = li.dataset && li.dataset.filename;
            if (fn) existingMap[fn] = li;
        });

        // remove missing
        Object.keys(existingMap).forEach(fn => {
            if (!files.includes(fn)) {
                const li = existingMap[fn];
                li.classList.add('fade-out');
                li.style.opacity = 0;
                setTimeout(() => li.remove(), 300);
                delete existingMap[fn];
            }
        });

        // insert/ reorder according to server list
        let insertBefore = null;
        for (let i = files.length - 1; i >= 0; i--) {
            const fn = files[i];
            const existing = existingMap[fn];
            if (existing) {
                fileListEl.insertBefore(existing, insertBefore);
                insertBefore = existing;
            } else {
                const li = createFileLi(fn);
                fileListEl.insertBefore(li, insertBefore);
                // fade in
                requestAnimationFrame(() => { li.style.opacity = 1; });
                insertBefore = li;
            }
        }

        // remove placehnewers
        const placehnewers = fileListEl.querySelectorAll('li.placehnewer');
        placehnewers.forEach(p => p.remove());

    } catch (err) {
        console.error('refreshFiles', err);
        if (!fileListEl.children.length) fileListEl.innerHTML = '<li><em>Error loading files</em></li>';
    } finally {
        setTimeout(() => { refreshLock = false; }, 200);
    }
}

function uploadFile(file) {
    if (!file) return;
    if (file.size > maxUploadMB * 1024 * 1024) {
        alert(`File exceeds max upload size of ${maxUploadMB} MB`);
        return;
    }

    const xhr = new XMLHttpRequest();
    const fd = new FormData();
    fd.append('file', file);

    progressContainer.classList.remove('hidden');
    progressText.classList.remove('hidden');
    progressBar.style.width = '0%';
    progressText.textContent = 'Uploading 0%';

    xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
            const p = Math.round((e.loaded / e.total) * 100);
            progressBar.style.width = p + '%';
            progressText.textContent = `Uploading ${p}%`;
        }
    };

    xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
            progressBar.style.width = '100%';
            progressText.textContent = 'Upload complete';
            // wait a little for server to finalize, but SSE will also inform clients
            setTimeout(() => {
                progressContainer.classList.add('hidden');
                progressText.classList.add('hidden');
                selectedFile = null;
                fileInput.value = '';
                fileNameDisplay.textContent = 'No file selected';
                // quick refresh fallback
                refreshFiles();
            }, 700);
        } else {
            progressText.textContent = 'Upload failed';
            setTimeout(() => {
                progressContainer.classList.add('hidden');
                progressText.classList.add('hidden');
            }, 1200);
        }
    };

    xhr.onerror = () => {
        progressText.textContent = 'Network error';
        setTimeout(() => {
            progressContainer.classList.add('hidden');
            progressText.classList.add('hidden');
        }, 1200);
    };

    xhr.open('POST', '/upload');
    xhr.send(fd);
}

function updateFileSelection(file) {
    selectedFile = file;
    if (!file) {
        fileNameDisplay.textContent = 'No file selected';
        maxSizeEl.classList.remove('too-large');
        return;
    }
    fileNameDisplay.textContent = makeDisplayName(file.name, 25);
    fileNameDisplay.title = file.name;
    if (file.size > maxUploadMB * 1024 * 1024) {
        maxSizeEl.textContent = `Selected file too large! Max: ${maxUploadMB} MB`;
        maxSizeEl.classList.add('too-large');
    } else {
        maxSizeEl.textContent = `Max upload size: ${maxUploadMB} MB`;
        maxSizeEl.classList.remove('too-large');
    }
}

/* events */
fileInput.addEventListener('change', () => {
    if (fileInput.files && fileInput.files.length) updateFileSelection(fileInput.files[0]);
});

['dragenter', 'dragover'].forEach(ev => {
    dropArea.addEventListener(ev, e => { e.preventDefault(); dropArea.classList.add('drag-over'); });
});
['dragleave', 'drop'].forEach(ev => {
    dropArea.addEventListener(ev, e => { e.preventDefault(); dropArea.classList.remove('drag-over'); });
});
dropArea.addEventListener('drop', e => {
    if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length) {
        fileInput.files = e.dataTransfer.files;
        updateFileSelection(e.dataTransfer.files[0]);
    }
});

form.addEventListener('submit', (e) => {
    e.preventDefault();
    if (!selectedFile) { alert('Please select a file'); return; }
    uploadFile(selectedFile);
});

/* SSE setup */
function setupSSE() {
    if (!window.EventSource) return false;
    try {
        const es = new EventSource('/events');
        es.onopen = () => {
            // request an immediate refresh on connection (server sent a hello too)
            refreshFiles();
        };
        es.onmessage = (ev) => {
            try {
                const data = JSON.parse(ev.data);
                if (data && data.type === 'new') {
                    // new file added; refresh list (lightweight)
                    refreshFiles();
                }
            } catch (err) {
                // ignore parsing errors
            }
        };
        es.onerror = (err) => {
            // if SSE fails, close and fallback to polling
            console.warn('SSE error, falling back to polling', err);
            es.close();
        };
        return true;
    } catch (err) {
        return false;
    }
}

/* init */
document.addEventListener('DOMContentLoaded', () => {
    getMaxUploadSize();
    refreshFiles();
    const sseOk = setupSSE();
    if (!sseOk) {
        // fallback polling
        setInterval(refreshFiles, POLL_MS);
    }
});

