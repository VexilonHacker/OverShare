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
let searchTerm = '';

const themeToggle = document.createElement('button');
themeToggle.className = 'theme-toggle';
themeToggle.setAttribute('aria-label', 'Toggle theme');
themeToggle.innerHTML = `
    <svg class="sun-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"></path>
    </svg>
    <svg class="moon-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path>
    </svg>
`;
document.body.appendChild(themeToggle);

const savedTheme = localStorage.getItem('theme') || 'light';
document.documentElement.setAttribute('data-theme', savedTheme);
updateThemeIcon(savedTheme);

themeToggle.addEventListener('click', () => {
	const currentTheme = document.documentElement.getAttribute('data-theme');
	const newTheme = currentTheme === 'light' ? 'dark' : 'light';
	document.body.classList.add('theme-transition');
	document.documentElement.setAttribute('data-theme', newTheme);
	localStorage.setItem('theme', newTheme);
	updateThemeIcon(newTheme);
	setTimeout(() => {
		document.body.classList.remove('theme-transition');
	}, 500);
});

function updateThemeIcon(theme) {
	const sunIcon = themeToggle.querySelector('.sun-icon');
	const moonIcon = themeToggle.querySelector('.moon-icon');
	if (theme === 'dark') {
		sunIcon.style.opacity = '0';
		sunIcon.style.transform = 'rotate(-90deg) scale(0.5)';
		moonIcon.style.opacity = '1';
		moonIcon.style.transform = 'rotate(0) scale(1)';
	} else {
		sunIcon.style.opacity = '1';
		sunIcon.style.transform = 'rotate(0) scale(1)';
		moonIcon.style.opacity = '0';
		moonIcon.style.transform = 'rotate(90deg) scale(0.5)';
	}
}

const selectFilesBtn = document.getElementById('selectFilesBtn');
const selectionBar = document.getElementById('selectionBar');
const selectAllCheckbox = document.getElementById('selectAllCheckbox');
const downloadSelectedBtn = document.getElementById('downloadSelectedBtn');
const selectedCountSpan = document.getElementById('selectedCount');
const cancelSelectionBtn = document.getElementById('cancelSelectionBtn');
let selectedFiles = new Set();
let selectionModeActive = false;

function enterSelectionMode() {
	selectionModeActive = true;
	selectionBar.classList.remove('hidden');
	document.body.classList.add('selection-mode-active');
	setupFileSelection();
	clearSelection();
}

function exitSelectionMode() {
	selectionModeActive = false;
	selectionBar.classList.add('hidden');
	document.body.classList.remove('selection-mode-active');
	clearSelection();
}

function clearSelection() {
	selectedFiles.clear();
	const checkboxes = document.querySelectorAll('#fileList li[data-filename] .file-checkbox input');
	checkboxes.forEach(cb => {
		cb.checked = false;
		cb.closest('li').classList.remove('selected');
	});
	updateSelectionUI();
}

selectFilesBtn.addEventListener('click', enterSelectionMode);
cancelSelectionBtn.addEventListener('click', exitSelectionMode);

function setupFileSelection() {
	const fileItems = document.querySelectorAll('#fileList li[data-filename]');
	fileItems.forEach(li => {
		if (!li.querySelector('.file-checkbox')) {
			const checkboxDiv = document.createElement('div');
			checkboxDiv.className = 'file-checkbox';
			checkboxDiv.innerHTML = '<input type="checkbox">';
			const checkbox = checkboxDiv.querySelector('input');
			checkbox.addEventListener('change', (e) => {
				e.stopPropagation();
				const filename = li.dataset.filename;
				if (checkbox.checked) {
					selectedFiles.add(filename);
					li.classList.add('selected');
				} else {
					selectedFiles.delete(filename);
					li.classList.remove('selected');
				}
				updateSelectionUI();
			});
			li.insertBefore(checkboxDiv, li.firstChild);
		}
	});
}

function updateSelectionUI() {
	const count = selectedFiles.size;
	selectedCountSpan.textContent = `${count} selected`;
	if (selectAllCheckbox) {
		const totalItems = document.querySelectorAll('#fileList li[data-filename]').length;
		if (count === 0) {
			selectAllCheckbox.checked = false;
			selectAllCheckbox.indeterminate = false;
		} else if (count === totalItems) {
			selectAllCheckbox.checked = true;
			selectAllCheckbox.indeterminate = false;
		} else {
			selectAllCheckbox.indeterminate = true;
		}
	}
	if (downloadSelectedBtn) {
		downloadSelectedBtn.disabled = count === 0;
	}
}

if (selectAllCheckbox) {
	selectAllCheckbox.addEventListener('change', () => {
		const checkboxes = document.querySelectorAll('#fileList li[data-filename] .file-checkbox input');
		checkboxes.forEach(cb => {
			cb.checked = selectAllCheckbox.checked;
			const event = new Event('change', { bubbles: true });
			cb.dispatchEvent(event);
		});
	});
}

if (downloadSelectedBtn) {
	downloadSelectedBtn.addEventListener('click', () => {
		if (selectedFiles.size === 0) return;
		const filesArray = Array.from(selectedFiles);
		const filesParam = filesArray.map(f => encodeURIComponent(f)).join(',');
		downloadSelectedBtn.disabled = true;
		downloadSelectedBtn.innerHTML = `
            <svg class="zip-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" />
                <path d="M12 6v6l4 2" />
            </svg>
            <span>Creating ZIP...</span>
        `;
		window.location.href = `/zip?files=${filesParam}`;
		setTimeout(() => {
			downloadSelectedBtn.innerHTML = `
                <svg class="zip-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="7 10 12 15 17 10" />
                    <line x1="12" y1="15" x2="12" y2="3" />
                    <rect x="3" y="18" width="18" height="2" rx="1" />
                </svg>
                <span>Download selected as ZIP</span>
            `;
			downloadSelectedBtn.disabled = selectedFiles.size === 0;
			exitSelectionMode();
		}, 2000);
	});
}

document.addEventListener('keydown', (e) => {
	if ((e.ctrlKey || e.metaKey) && e.key === 'a') {
		e.preventDefault();
		if (selectAllCheckbox && selectionModeActive) {
			selectAllCheckbox.checked = true;
			const event = new Event('change', { bubbles: true });
			selectAllCheckbox.dispatchEvent(event);
		}
	}
});

const searchInput = document.getElementById('fileSearch');
const searchClear = document.getElementById('searchClear');

if (searchInput) {
	searchInput.addEventListener('input', (e) => {
		searchTerm = e.target.value.toLowerCase().trim();
		filterFiles();
	});
}

if (searchClear) {
	searchClear.addEventListener('click', () => {
		searchInput.value = '';
		searchTerm = '';
		filterFiles();
		searchInput.focus();
	});
}

function filterFiles() {
	const items = document.querySelectorAll('#fileList li[data-filename]');
	let hasVisible = false;
	items.forEach(li => {
		const filename = li.dataset.filename.toLowerCase();
		const displayName = li.querySelector('.filename span')?.textContent.toLowerCase() || '';
		if (searchTerm === '' || filename.includes(searchTerm) || displayName.includes(searchTerm)) {
			li.style.display = 'flex';
			hasVisible = true;
		} else {
			li.style.display = 'none';
		}
	});
	const fileList = document.getElementById('fileList');
	const noResults = document.querySelector('.no-results');
	if (!hasVisible && items.length > 0) {
		if (!noResults) {
			const msg = document.createElement('li');
			msg.className = 'no-results';
			msg.innerHTML = '<em>No matching files found</em>';
			msg.style.justifyContent = 'center';
			fileList.appendChild(msg);
		}
	} else if (noResults) {
		noResults.remove();
	}
}

const qrModal = document.getElementById('qrModal');
const qrModalTitle = document.getElementById('qrModalTitle');
const qrModalImage = document.getElementById('qrModalImage');
const qrModalClose = document.querySelector('.qr-modal-close');

const qrSvgIcon = `
<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
    <rect x="3" y="3" width="7" height="7" rx="1" />
    <rect x="14" y="3" width="7" height="7" rx="1" />
    <rect x="3" y="14" width="7" height="7" rx="1" />
    <rect x="14" y="14" width="4" height="4" rx="0.5" />
    <path d="M18 14h3v3M14 18v3M21 18v3h-3" />
</svg>
`;



async function generateQRCode(filename) {
	try {
		// Get the correct server URL
		const serverUrl = await getServerUrl();
		const fileUrl = `${serverUrl}/download/${encodeURIComponent(filename)}`;

		// Clear previous QR code
		const qrContainer = document.getElementById("qrModalImage");
		qrContainer.innerHTML = "";

		// Create QR code with styling
		const qrCode = new QRCodeStyling({
			width: 300,
			height: 300,
			data: fileUrl,
			image: "/cat.png",
			dotsOptions: {
				color: "#000000",
				type: "rounded"
			},
			backgroundOptions: {
				color: "#ffffff"
			},
			imageOptions: {
				crossOrigin: "anonymous",
				margin: 0,
				imageSize: 0.55
			},
			cornersSquareOptions: {
				type: "extra-rounded",
				color: "#000000"
			},
			cornersDotOptions: {
				type: "dot",
				color: "#000000"
			}
		});

		// Append to container
		qrCode.append(qrContainer);

		// Update modal title and show
		document.getElementById('qrModalTitle').textContent = `${makeDisplayName(filename, 30)}`;
		document.getElementById('qrModal').style.display = 'block';

	} catch (err) {
		console.error('QR generation failed:', err);
		alert('Failed to generate QR code: ' + err.message);
	}
}

// Function to get the correct server URL
async function getServerUrl() {
	// Try to get the local IP from the server
	try {
		const response = await fetch('/api/local-ip');
		if (response.ok) {
			const data = await response.json();
			return `http://${data.ip}:${window.location.port}`;
		}
	} catch (err) {
		console.log('Could not fetch local IP, falling back to window.location.host');
	}

	// Fallback: use current hostname but replace localhost/127.0.0.1 with private IP
	const hostname = window.location.hostname;
	if (hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '0.0.0.0') {
		// Try to get local IP via WebRTC (client-side)
		const localIP = await getLocalIP();
		if (localIP) {
			return `http://${localIP}:${window.location.port}`;
		}
	}

	// If all else fails, use current host
	return window.location.origin;
}

// Client-side IP detection using WebRTC
function getLocalIP() {
	return new Promise((resolve) => {
		const pc = new RTCPeerConnection({ iceServers: [] });
		pc.createDataChannel('');
		pc.createOffer()
			.then(offer => pc.setLocalDescription(offer))
			.catch(() => { });

		pc.onicecandidate = (ice) => {
			if (!ice || !ice.candidate) {
				pc.close();
				resolve(null);
				return;
			}

			const ipMatch = ice.candidate.candidate.match(/([0-9]{1,3}\.){3}[0-9]{1,3}/);
			if (ipMatch) {
				const ip = ipMatch[0];
				// Filter out localhost and APIPA addresses
				if (!ip.startsWith('127.') && !ip.startsWith('169.254.')) {
					pc.close();
					resolve(ip);
				}
			}
		};

		// Timeout after 2 seconds
		setTimeout(() => {
			pc.close();
			resolve(null);
		}, 2000);
	});
}

if (qrModalClose) {
	qrModalClose.addEventListener('click', () => {
		qrModal.style.display = 'none';
	});
}

window.addEventListener('click', (e) => {
	if (e.target === qrModal) {
		qrModal.style.display = 'none';
	}
});

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
	} finally {
		maxSizeEl.textContent = `Max upload size: ${maxUploadMB} MB`;
	}
}

function getFileIcon(filename) {
	const ext = filename.split('.').pop().toLowerCase();
	const icons = {
		'pdf': '📕', 'doc': '📘', 'docx': '📘', 'txt': '📄',
		'rtf': '📄', 'md': '📄', 'odt': '📄',
		'jpg': '🖼️', 'jpeg': '🖼️', 'png': '🖼️', 'gif': '🖼️',
		'bmp': '🖼️', 'webp': '🖼️', 'svg': '🖼️', 'ico': '🖼️',
		'mp3': '🎵', 'wav': '🎵', 'ogg': '🎵', 'flac': '🎵',
		'm4a': '🎵', 'aac': '🎵',
		'mp4': '🎬', 'avi': '🎬', 'mkv': '🎬', 'mov': '🎬',
		'wmv': '🎬', 'webm': '🎬', 'flv': '🎬',
		'zip': '📦', 'rar': '📦', '7z': '📦', 'tar': '📦',
		'gz': '📦', 'bz2': '📦',
		'html': '🌐', 'css': '🎨', 'js': '⚙️', 'json': '📋',
		'xml': '📋', 'py': '🐍', 'go': '🔵', 'java': '☕',
		'c': '⚙️', 'cpp': '⚙️', 'php': '🐘', 'rb': '💎',
		'exe': '⚙️', 'msi': '📦', 'sh': '📜', 'bat': '📜',
		'app': '📱', 'dmg': '💿', 'iso': '💿',
		'xls': '📊', 'xlsx': '📊', 'csv': '📊', 'ods': '📊',
		'ppt': '📽️', 'pptx': '📽️', 'odp': '📽️',
		'default': '📄'
	};
	return icons[ext] || icons['default'];
}

function createFileLi(filename) {
	const li = document.createElement('li');
	li.dataset.filename = filename;
	li.style.opacity = 0;
	li.style.transition = 'opacity 0.25s ease, transform 0.25s ease';

	const a = document.createElement('a');
	a.className = 'filename';
	a.href = '/download/' + encodeURIComponent(filename);

	const decodedFilename = decodeURIComponent(filename);
	const displayName = makeDisplayName(decodedFilename, 40);
	const icon = getFileIcon(filename);

	// Wrap icon in a span with class for animation
	const iconSpan = document.createElement('span');
	iconSpan.className = 'file-icon';
	iconSpan.textContent = icon + ' ';

	const nameSpan = document.createElement('span');
	nameSpan.textContent = displayName;

	a.appendChild(iconSpan);
	a.appendChild(nameSpan);
	a.title = decodedFilename;

	const actions = document.createElement('div');
	actions.className = 'file-actions';

	const qrBtn = document.createElement('button');
	qrBtn.className = 'qr-file-btn';
	qrBtn.innerHTML = qrSvgIcon;
	qrBtn.title = 'Generate QR Code';
	qrBtn.addEventListener('click', (e) => {
		e.preventDefault();
		e.stopPropagation();
		generateQRCode(filename);
	});

	const btn = document.createElement('a');
	btn.className = 'download-btn';
	btn.href = '/download/' + encodeURIComponent(filename);
	btn.textContent = 'Download';

	actions.appendChild(qrBtn);
	actions.appendChild(btn);
	li.appendChild(a);
	li.appendChild(actions);

	return li;
}

async function refreshFiles() {
	if (refreshLock) return;
	refreshLock = true;
	if (!fileListEl.children.length) {
		fileListEl.innerHTML = '<li class="placeholder"><em>Loading…</em></li>';
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
			refreshLock = false;
			return;
		}
		const existingMap = {};
		Array.from(fileListEl.children).forEach(li => {
			const fn = li.dataset && li.dataset.filename;
			if (fn) existingMap[fn] = li;
		});
		Object.keys(existingMap).forEach(fn => {
			if (!files.includes(fn)) {
				const li = existingMap[fn];
				li.classList.add('fade-out');
				li.style.opacity = 0;
				setTimeout(() => li.remove(), 300);
				delete existingMap[fn];
			}
		});
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
				requestAnimationFrame(() => { li.style.opacity = 1; });
				insertBefore = li;
			}
		}
		const placeholders = fileListEl.querySelectorAll('li.placeholder');
		placeholders.forEach(p => p.remove());
		if (selectionModeActive) {
			setupFileSelection();
		}
		if (searchTerm) {
			filterFiles();
		}
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
			setTimeout(() => {
				progressContainer.classList.add('hidden');
				progressText.classList.add('hidden');
				selectedFile = null;
				fileInput.value = '';
				fileNameDisplay.textContent = 'No file selected';
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

function setupSSE() {
	if (!window.EventSource) return false;
	try {
		const es = new EventSource('/events');
		es.onopen = () => {
			refreshFiles();
		};
		es.onmessage = (ev) => {
			try {
				const data = JSON.parse(ev.data);
				if (data && data.type === 'new') {
					refreshFiles();
				}
			} catch (err) {
			}
		};
		es.onerror = (err) => {
			console.warn('SSE error, falling back to polling', err);
			es.close();
		};
		return true;
	} catch (err) {
		return false;
	}
}



function setupKeyboardShortcuts() {
	document.addEventListener('keydown', (e) => {
		// Don't trigger shortcuts if user is typing in an input
		if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
			return;
		}

		// ? - Show shortcuts help
		if (e.key === '?' && !e.ctrlKey && !e.metaKey) {
			e.preventDefault();
			showShortcutsHelp();
		}

		// Ctrl/Cmd + F - Focus search
		if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
			e.preventDefault();
			document.getElementById('fileSearch').focus();
			showShortcutToast('🔍 Search focused');
		}

		// T - Toggle theme
		if (e.key === 't' && !e.ctrlKey && !e.metaKey && !e.shiftKey) {
			e.preventDefault();
			themeToggle.click();
			showShortcutToast(`🌓 ${document.documentElement.getAttribute('data-theme') === 'dark' ? 'Dark' : 'Light'} mode`);
		}

		// Ctrl/Cmd + A - Select all (only in selection mode)
		if ((e.ctrlKey || e.metaKey) && e.key === 'a') {
			if (selectionModeActive) {
				e.preventDefault();
				selectAllCheckbox.checked = true;
				const event = new Event('change', { bubbles: true });
				selectAllCheckbox.dispatchEvent(event);
			}
		}

		// Ctrl/Cmd + D - Download selected (only in selection mode)
		if ((e.ctrlKey || e.metaKey) && e.key === 'd') {
			if (selectionModeActive && selectedFiles.size > 0) {
				e.preventDefault();
				downloadSelectedBtn.click();
			}
		}
	});
}


function showShortcutsHelp() {
	const existingHelp = document.querySelector('.shortcuts-help');
	if (existingHelp) {
		existingHelp.remove();
	}

	const shortcuts = [
		{ key: '?', description: 'Show this help' },
		{ key: 'Ctrl/Cmd + F', description: 'Focus search bar' },
		{ key: 'T', description: 'Toggle dark/light theme' },
		{ key: 'Ctrl/Cmd + A', description: 'Select all files (in selection mode)' },
		{ key: 'Ctrl/Cmd + D', description: 'Download selected as ZIP' }
	];

	const helpDiv = document.createElement('div');
	helpDiv.className = 'shortcuts-help';
	helpDiv.innerHTML = `
        <div class="shortcuts-help-content">
            <table>
                ${shortcuts.map(s => `
                    <tr>
                        <td><kbd>${s.key}</kbd></td>
                        <td>${s.description}</td>
                    </tr>
                `).join('')}
            </table>
            <button class="shortcuts-close">Got it</button>
        </div>
    `;

	document.body.appendChild(helpDiv);
	setTimeout(() => helpDiv.classList.add('show'), 10);

	helpDiv.querySelector('.shortcuts-close').addEventListener('click', () => {
		helpDiv.classList.remove('show');
		setTimeout(() => helpDiv.remove(), 300);
	});

	helpDiv.addEventListener('click', (e) => {
		if (e.target === helpDiv) {
			helpDiv.classList.remove('show');
			setTimeout(() => helpDiv.remove(), 300);
		}
	});
}

function setMobileViewport() {
	const screenWidth = window.innerWidth;
	const desktopWidth = 800;
	const viewport = document.querySelector('meta[name=viewport]');

	if (screenWidth < desktopWidth) {
		// Calculate scale to fit desktop width
		const scale = (screenWidth / desktopWidth).toFixed(2);
		viewport.setAttribute('content',
			`width=${desktopWidth}, initial-scale=${scale}, minimum-scale=${scale}, maximum-scale=${scale}, user-scalable=no`
		);
	} else {
		viewport.setAttribute('content',
			'width=device-width, initial-scale=1.0, maximum-scale=2.0, user-scalable=yes'
		);
	}
}

// Run on load and resize
document.addEventListener('DOMContentLoaded', setMobileViewport);
window.addEventListener('resize', setMobileViewport);
window.addEventListener('orientationchange', () => {
	setTimeout(setMobileViewport, 100);
});


document.addEventListener('DOMContentLoaded', () => {
	getMaxUploadSize();
	refreshFiles();
	const sseOk = setupSSE();
	if (!sseOk) {
		setInterval(refreshFiles, POLL_MS);
	}
	setupKeyboardShortcuts();

});
