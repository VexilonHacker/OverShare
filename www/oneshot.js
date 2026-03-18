(function() {
    'use strict';

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

    themeToggle.addEventListener('click', toggleTheme);

    document.addEventListener('keydown', (e) => {
        if ((e.key === 't' || e.key === 'T') &&
            !e.ctrlKey && !e.metaKey && !e.altKey &&
            document.activeElement.tagName !== 'INPUT' &&
            document.activeElement.tagName !== 'TEXTAREA') {
            e.preventDefault();
            toggleTheme();
        }
    });

    function toggleTheme() {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'light' ? 'dark' : 'light';
        document.body.classList.add('theme-transition');
        document.documentElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
        updateThemeIcon(newTheme);
        setTimeout(() => {
            document.body.classList.remove('theme-transition');
        }, 500);
    }

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

    let wasExpired = false;
    let progressRing = document.getElementById('progressRing');
    let ringRadius = 55;
    let ringCircumference = 2 * Math.PI * ringRadius;

    if (progressRing) {
        progressRing.style.strokeDasharray = `${ringCircumference}`;
        progressRing.style.strokeDashoffset = '0';
    }

    function updateProgress(remaining, max) {
        if (!progressRing) return;
        let percentage = (remaining / max) * 100;
        let offset = ringCircumference - (percentage / 100) * ringCircumference;
        progressRing.style.strokeDashoffset = offset;
        if (remaining === 0) {
            progressRing.style.stroke = '#ff5c5c';
        } else if (remaining <= 2) {
            progressRing.style.stroke = '#f39c12';
        } else {
            progressRing.style.stroke = '#2ecc71';
        }
    }

    function checkStatus() {
        fetch('?status=1&_=' + Date.now())
            .then(response => {
                if (!response.ok) {
                    throw new Error('Network response was not ok');
                }
                return response.json();
            })
            .then(data => {
                const card = document.getElementById('transferCard');
                const badgeText = document.getElementById('badgeText');
                const statusBadge = document.querySelector('.status-badge');
                const downloadBtn = document.getElementById('downloadBtn');
                const btnText = document.getElementById('btnText');
                const noticeMessage = document.getElementById('noticeMessage');
                const expiredOverlay = document.getElementById('expiredOverlay');
                if (data.expired) {
                    if (!wasExpired) {
                        card.classList.add('expired-animate');
                        setTimeout(() => {
                            card.classList.remove('expired-animate');
                        }, 1000);
                        wasExpired = true;
                    }
                    card.classList.add('expired');
                    badgeText.textContent = 'EXPIRED';
                    statusBadge.style.background = '#999';
                    btnText.textContent = 'Download Unavailable';
                    downloadBtn.classList.add('disabled');
                    downloadBtn.style.pointerEvents = 'none';
                    noticeMessage.innerHTML = 'This file has expired and is no longer available for download';
                    if (expiredOverlay) expiredOverlay.classList.add('visible');
                    updateProgress(0, data.max);
                } else {
                    wasExpired = false;
                    card.classList.remove('expired');
                    if (expiredOverlay) expiredOverlay.classList.remove('visible');
                    if (data.max > 1) {
                        badgeText.textContent = `${data.remaining} LEFT`;
                        statusBadge.style.background = data.remaining <= 2 ? '#f39c12' : '#2ecc71';
                        if (data.remaining === 1) {
                            noticeMessage.innerHTML = '<strong>Last download!</strong> This file will expire after this transfer';
                        } else {
                            noticeMessage.innerHTML = `This file allows <strong>${data.max} downloads</strong>. ${data.remaining} remaining`;
                        }
                    } else {
                        badgeText.textContent = 'ONE-TIME';
                        statusBadge.style.background = '#0366d6';
                        noticeMessage.innerHTML = 'This file will expire immediately after download';
                    }
                    updateProgress(data.remaining, data.max);
                    btnText.textContent = 'Download File';
                    downloadBtn.classList.remove('disabled');
                    downloadBtn.style.pointerEvents = 'auto';
                }
            })
            .catch(err => {
                console.error('Status check failed:', err);
                const badgeText = document.getElementById('badgeText');
                const downloadBtn = document.getElementById('downloadBtn');
                if (badgeText) badgeText.textContent = 'OFFLINE';
                if (downloadBtn) {
                    downloadBtn.style.pointerEvents = 'none';
                    downloadBtn.classList.add('disabled');
                }
                const noticeMessage = document.getElementById('noticeMessage');
                if (noticeMessage) {
                    noticeMessage.innerHTML = 'Unable to connect to server. Please check your connection.';
                }
            });
    }

    const downloadBtn = document.getElementById('downloadBtn');
    if (downloadBtn) {
        downloadBtn.addEventListener('click', function(e) {
            if (!this.classList.contains('disabled')) {
                this.classList.add('clicked');
                setTimeout(() => {
                    this.classList.remove('clicked');
                }, 300);
            }
        });
    }

    checkStatus();
    setInterval(checkStatus, 2000);

    document.addEventListener('DOMContentLoaded', function() {
        const card = document.getElementById('transferCard');
        if (card) {
            card.classList.add('loading-pulse');
            setTimeout(() => {
                card.classList.remove('loading-pulse');
            }, 1000);
        }
    });

    document.addEventListener('visibilitychange', function() {
        if (document.hidden) {
            if (window._statusInterval) {
                clearInterval(window._statusInterval);
            }
        } else {
            if (window._statusInterval) {
                clearInterval(window._statusInterval);
            }
            checkStatus();
            window._statusInterval = setInterval(checkStatus, 2000);
        }
    });

    window._statusInterval = setInterval(checkStatus, 2000);
})();
