// mdview — client-side: file navigation, content loading, WebSocket live reload

var currentFile = '';

document.addEventListener('DOMContentLoaded', function() {
    // Restore file from URL if present.
    var params = new URLSearchParams(location.search);
    var fileParam = params.get('file');
    if (fileParam) {
        currentFile = fileParam;
    }

    loadFiles().then(function() {
        loadInfo();
        loadContent();
    });
    connectWebSocket();
});

// ── File navigation ─────────────────────────────────────────────────

function loadFiles() {
    return fetch('/api/files')
        .then(function(res) { return res.json(); })
        .then(function(files) {
            if (!files || files.length === 0) return;
            if (!currentFile) {
                currentFile = files[0];
            }
            renderFileNav(files);
        })
        .catch(function(err) {
            console.error('mdview: failed to load file list:', err);
        });
}

function renderFileNav(files) {
    var nav = document.getElementById('filenav');
    if (!nav) return;

    // Hide nav when there's only one file.
    if (files.length <= 1) {
        nav.innerHTML = '';
        return;
    }

    nav.innerHTML = '';
    for (var i = 0; i < files.length; i++) {
        var a = document.createElement('a');
        a.className = 'filenav-item' + (files[i] === currentFile ? ' active' : '');
        a.textContent = files[i];
        a.href = '?file=' + encodeURIComponent(files[i]);
        a.setAttribute('data-file', files[i]);
        a.addEventListener('click', handleFileNavClick);
        nav.appendChild(a);
    }
}

function handleFileNavClick(e) {
    e.preventDefault();
    var file = this.getAttribute('data-file');
    if (file === currentFile) return;
    currentFile = file;
    history.pushState(null, '', '?file=' + encodeURIComponent(file));
    updateActiveNav();
    loadInfo();
    loadContent();
}

function updateActiveNav() {
    var items = document.querySelectorAll('.filenav-item');
    for (var i = 0; i < items.length; i++) {
        items[i].classList.toggle('active', items[i].getAttribute('data-file') === currentFile);
    }
}

// Handle browser back/forward.
window.addEventListener('popstate', function() {
    var params = new URLSearchParams(location.search);
    var file = params.get('file');
    if (file && file !== currentFile) {
        currentFile = file;
        updateActiveNav();
        loadInfo();
        loadContent();
    }
});

// ── File info (top bar) ─────────────────────────────────────────────

function fileQueryString() {
    return currentFile ? '?file=' + encodeURIComponent(currentFile) : '';
}

function loadInfo() {
    fetch('/api/info' + fileQueryString())
        .then(function(res) { return res.json(); })
        .then(function(info) {
            document.title = info.fileName + ' — mdview';
            document.getElementById('filename').textContent = info.fileName;
            document.getElementById('dirpath').textContent = shortenPath(info.filePath, info.fileName);
        })
        .catch(function(err) {
            console.error('mdview: failed to load file info:', err);
        });
}

// Show last 3 directory segments before the filename.
function shortenPath(fullPath, fileName) {
    var dir = fullPath.replace(fileName, '');
    var parts = dir.split('/').filter(Boolean);
    if (parts.length <= 3) return dir;
    return '…/' + parts.slice(-3).join('/') + '/';
}

// ── Content loading ─────────────────────────────────────────────────

function loadContent() {
    var content = document.getElementById('content');
    if (content) content.style.opacity = '0.5';

    fetch('/api/render' + fileQueryString())
        .then(function(res) {
            if (res.status === 404) {
                showNotFoundOverlay();
                return null;
            }
            return res.text();
        })
        .then(function(html) {
            if (html === null) return;
            content.innerHTML = html;
            postProcess(content);
            removeNotFoundOverlay();
        })
        .catch(function(err) {
            console.error('mdview: failed to load content:', err);
            content.innerHTML = '<p>Error loading content.</p>';
        })
        .finally(function() {
            if (content) content.style.opacity = '1';
        });
}

// ── Post-processing ─────────────────────────────────────────────────

function postProcess(container) {
    rewriteExternalLinks(container);
    rewriteRelativeImages(container);
    rewriteRelativeLinks(container);
    rewriteMdLinks(container);
    wrapTables(container);
    highlightCode();
}

// External links open in a new tab.
function rewriteExternalLinks(container) {
    var links = container.querySelectorAll('a[href]');
    for (var i = 0; i < links.length; i++) {
        var href = links[i].getAttribute('href');
        if (href && (href.indexOf('http://') === 0 || href.indexOf('https://') === 0)) {
            links[i].setAttribute('target', '_blank');
            links[i].setAttribute('rel', 'noopener noreferrer');
        }
    }
}

// Relative image sources go through /file/ so the server can serve them.
function rewriteRelativeImages(container) {
    var imgs = container.querySelectorAll('img[src]');
    for (var i = 0; i < imgs.length; i++) {
        var src = imgs[i].getAttribute('src');
        if (src && !src.match(/^(https?:\/\/|\/)/)) {
            imgs[i].setAttribute('src', '/file/' + src);
        }
    }
}

// Non-external, non-md links also go through /file/.
function rewriteRelativeLinks(container) {
    var links = container.querySelectorAll('a[href]');
    for (var i = 0; i < links.length; i++) {
        var href = links[i].getAttribute('href');
        if (!href) continue;
        if (href.match(/^(https?:\/\/|\/|#)/)) continue;
        if (href.match(/\.md$/i)) continue; // handled by rewriteMdLinks
        links[i].setAttribute('href', '/file/' + href);
    }
}

// Links to .md files navigate within mdview instead of downloading.
function rewriteMdLinks(container) {
    var links = container.querySelectorAll('a[href]');
    for (var i = 0; i < links.length; i++) {
        var href = links[i].getAttribute('href');
        if (!href) continue;
        if (href.match(/^(https?:\/\/|\/|#)/)) continue;
        if (!href.match(/\.md$/i)) continue;

        var fileName = href.split('/').pop();
        links[i].setAttribute('href', '?file=' + encodeURIComponent(fileName));
        links[i].addEventListener('click', (function(name) {
            return function(e) {
                e.preventDefault();
                if (name === currentFile) return;
                currentFile = name;
                history.pushState(null, '', '?file=' + encodeURIComponent(name));
                updateActiveNav();
                loadInfo();
                loadContent();
                window.scrollTo(0, 0);
            };
        })(fileName));
    }
}

// Wrap tables in a scrollable container for wide tables.
function wrapTables(container) {
    var tables = container.querySelectorAll('table');
    for (var i = 0; i < tables.length; i++) {
        if (tables[i].parentNode.classList.contains('table-wrapper')) continue;
        var wrapper = document.createElement('div');
        wrapper.className = 'table-wrapper';
        tables[i].parentNode.insertBefore(wrapper, tables[i]);
        wrapper.appendChild(tables[i]);
    }
}

// Run highlight.js on code blocks.
function highlightCode() {
    if (typeof hljs !== 'undefined') {
        hljs.highlightAll();
    }
}

// ── WebSocket live reload ───────────────────────────────────────────

var reconnectDelay = 1000;
var maxReconnectDelay = 5000;
var ws = null;
var wasConnected = false;

function connectWebSocket() {
    var protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    var url = protocol + '//' + location.host + '/ws';

    ws = new WebSocket(url);

    // Warn if connection not established within 5 seconds.
    var connectTimeout = setTimeout(function() {
        if (!wasConnected) {
            showConnectionBanner('warn', '\u26A0 Could not connect to server. Is mdview running?');
        }
    }, 5000);

    ws.onopen = function() {
        clearTimeout(connectTimeout);
        reconnectDelay = 1000;
        if (wasConnected) {
            showConnectionBanner('ok', '\u2713 Reconnected');
            setTimeout(removeConnectionBanner, 2000);
        }
        wasConnected = true;
    };

    ws.onmessage = function(event) {
        if (event.data === 'reload') {
            reloadContent();
        }
    };

    ws.onclose = function() {
        if (wasConnected) {
            showConnectionBanner('warn', '\u26A0 Connection lost \u2014 reconnecting\u2026');
        }
        setTimeout(function() {
            connectWebSocket();
            reconnectDelay = Math.min(reconnectDelay * 1.5, maxReconnectDelay);
        }, reconnectDelay);
    };

    ws.onerror = function() {
        ws.close();
    };
}

function reloadContent() {
    var scrollY = window.scrollY;

    // Refresh file list in case files were added/removed.
    loadFiles();

    fetch('/api/render' + fileQueryString())
        .then(function(res) {
            if (res.status === 404) {
                showNotFoundOverlay();
                return null;
            }
            return res.text();
        })
        .then(function(html) {
            if (html === null) return;
            var content = document.getElementById('content');
            content.innerHTML = html;
            postProcess(content);
            removeNotFoundOverlay();
            window.scrollTo(0, scrollY);
        })
        .catch(function(err) {
            console.error('mdview: reload error:', err);
        });
}

// ── Connection banners ──────────────────────────────────────────────

function showConnectionBanner(type, text) {
    removeConnectionBanner();
    var banner = document.createElement('div');
    banner.className = 'connection-banner connection-banner-' + type;
    banner.id = 'connection-banner';
    banner.textContent = text;
    var content = document.getElementById('content');
    if (content) content.parentNode.insertBefore(banner, content);
}

function removeConnectionBanner() {
    var b = document.getElementById('connection-banner');
    if (b) b.remove();
}

// ── Not-found overlay ───────────────────────────────────────────────

function showNotFoundOverlay() {
    if (document.getElementById('not-found-overlay')) return;
    var overlay = document.createElement('div');
    overlay.id = 'not-found-overlay';
    overlay.innerHTML = '<div class="not-found-box"><h2>File not found</h2><p>Waiting for the file to reappear\u2026</p></div>';
    document.body.appendChild(overlay);
}

function removeNotFoundOverlay() {
    var overlay = document.getElementById('not-found-overlay');
    if (overlay) overlay.remove();
}
