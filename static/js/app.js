// Main application JavaScript

// Theme management
function initTheme() {
    const theme = localStorage.getItem('theme') || 'auto';
    applyTheme(theme);
}

function applyTheme(theme) {
    if (theme === 'auto') {
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    } else {
        document.documentElement.setAttribute('data-theme', theme);
    }
    localStorage.setItem('theme', theme);
}

// Listen for theme changes
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
    const theme = localStorage.getItem('theme');
    if (theme === 'auto') {
        document.documentElement.setAttribute('data-theme', e.matches ? 'dark' : 'light');
    }
});

// PWA install prompt
let deferredPrompt;

window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault();
    deferredPrompt = e;
    showInstallPrompt();
});

function showInstallPrompt() {
    const promptDiv = document.createElement('div');
    promptDiv.className = 'install-prompt';
    promptDiv.innerHTML = `
        <div style="display: flex; justify-content: space-between; align-items: center;">
            <div>
                <strong>Install Injection Tracker</strong>
                <p style="margin: 0.5rem 0 0 0;">Add to home screen for quick access</p>
            </div>
            <div>
                <button onclick="installPWA()" style="margin-right: 0.5rem;">Install</button>
                <button onclick="dismissInstallPrompt()" class="outline">Later</button>
            </div>
        </div>
    `;
    document.body.appendChild(promptDiv);
}

function installPWA() {
    if (deferredPrompt) {
        deferredPrompt.prompt();
        deferredPrompt.userChoice.then((choiceResult) => {
            if (choiceResult.outcome === 'accepted') {
                console.log('User accepted the install prompt');
            }
            deferredPrompt = null;
            dismissInstallPrompt();
        });
    }
}

function dismissInstallPrompt() {
    const prompt = document.querySelector('.install-prompt');
    if (prompt) {
        prompt.remove();
    }
}

// Offline detection
window.addEventListener('online', () => {
    const indicator = document.querySelector('.offline-indicator');
    if (indicator) {
        indicator.remove();
    }
    // Try to sync any pending offline data
    syncOfflineData();
});

window.addEventListener('offline', () => {
    if (!document.querySelector('.offline-indicator')) {
        const indicator = document.createElement('div');
        indicator.className = 'offline-indicator';
        indicator.textContent = '⚠️ You are currently offline. Changes will be saved when connection is restored.';
        document.body.appendChild(indicator);
    }
});

// Offline data sync
async function syncOfflineData() {
    if ('indexedDB' in window) {
        // TODO: Implement IndexedDB sync logic
        console.log('Syncing offline data...');
    }
}

// Keyboard shortcuts
document.addEventListener('keydown', (e) => {
    // Ctrl/Cmd + K: Quick injection log
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        const injectButton = document.querySelector('[data-action="log-injection"]');
        if (injectButton) {
            injectButton.click();
        }
    }

    // Escape: Close modals
    if (e.key === 'Escape') {
        // Alpine.js handles this via @keydown.escape.window
    }
});

// Auto-save form data (for longer forms)
function autoSaveForm(formId, storageKey) {
    const form = document.getElementById(formId);
    if (!form) return;

    // Load saved data
    const savedData = localStorage.getItem(storageKey);
    if (savedData) {
        const data = JSON.parse(savedData);
        Object.keys(data).forEach(key => {
            const input = form.querySelector(`[name="${key}"]`);
            if (input) {
                if (input.type === 'checkbox') {
                    input.checked = data[key];
                } else {
                    input.value = data[key];
                }
            }
        });
    }

    // Save on input
    form.addEventListener('input', debounce(() => {
        const formData = new FormData(form);
        const data = {};
        for (let [key, value] of formData.entries()) {
            data[key] = value;
        }
        localStorage.setItem(storageKey, JSON.stringify(data));
    }, 500));

    // Clear on submit
    form.addEventListener('submit', () => {
        localStorage.removeItem(storageKey);
    });
}

// Debounce utility
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Format dates consistently
function formatDate(dateString, format = 'MM/DD/YYYY') {
    const date = new Date(dateString);
    const userFormat = localStorage.getItem('dateFormat') || format;

    const day = String(date.getDate()).padStart(2, '0');
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const year = date.getFullYear();

    switch (userFormat) {
        case 'DD/MM/YYYY':
            return `${day}/${month}/${year}`;
        case 'YYYY-MM-DD':
            return `${year}-${month}-${day}`;
        default: // MM/DD/YYYY
            return `${month}/${day}/${year}`;
    }
}

// Format time consistently
function formatTime(dateString, format = '12h') {
    const date = new Date(dateString);
    const userFormat = localStorage.getItem('timeFormat') || format;

    if (userFormat === '24h') {
        return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
    } else {
        return date.toLocaleTimeString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true });
    }
}

// Haptic feedback for mobile
function hapticFeedback(type = 'light') {
    if ('vibrate' in navigator) {
        switch (type) {
            case 'light':
                navigator.vibrate(10);
                break;
            case 'medium':
                navigator.vibrate(20);
                break;
            case 'heavy':
                navigator.vibrate(30);
                break;
        }
    }
}

// Add haptic feedback to buttons
document.addEventListener('click', (e) => {
    if (e.target.matches('button, [role="button"], input[type="submit"]')) {
        hapticFeedback('light');
    }
});

// Confirmation dialog with better UX
function confirmAction(message, onConfirm, onCancel) {
    if (confirm(message)) {
        if (onConfirm) onConfirm();
    } else {
        if (onCancel) onCancel();
    }
}

// Toast notifications
function showToast(message, type = 'info', duration = 3000) {
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    toast.style.cssText = `
        position: fixed;
        bottom: 2rem;
        right: 2rem;
        padding: 1rem 1.5rem;
        background: var(--pico-card-background-color);
        border-left: 4px solid var(--pico-${type === 'error' ? 'del' : type}-color);
        border-radius: var(--pico-border-radius);
        box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        z-index: 1000;
        animation: slideIn 0.3s ease;
    `;

    document.body.appendChild(toast);

    setTimeout(() => {
        toast.style.animation = 'slideOut 0.3s ease';
        setTimeout(() => toast.remove(), 300);
    }, duration);
}

// Animation keyframes (add to CSS if needed)
const style = document.createElement('style');
style.textContent = `
    @keyframes slideIn {
        from {
            transform: translateX(100%);
            opacity: 0;
        }
        to {
            transform: translateX(0);
            opacity: 1;
        }
    }

    @keyframes slideOut {
        from {
            transform: translateX(0);
            opacity: 1;
        }
        to {
            transform: translateX(100%);
            opacity: 0;
        }
    }
`;
document.head.appendChild(style);

// Service Worker Registration
if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
        navigator.serviceWorker.register('/service-worker.js')
            .then((registration) => {
                console.log('Service Worker registered:', registration.scope);

                // Check for updates periodically
                setInterval(() => {
                    registration.update();
                }, 60000); // Check every minute

                // Handle update found
                registration.addEventListener('updatefound', () => {
                    const newWorker = registration.installing;
                    newWorker.addEventListener('statechange', () => {
                        if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                            // New version available
                            showUpdateNotification();
                        }
                    });
                });
            })
            .catch((err) => {
                console.error('Service Worker registration failed:', err);
            });

        // Listen for messages from service worker
        navigator.serviceWorker.addEventListener('message', (event) => {
            if (event.data && event.data.type === 'SW_UPDATED') {
                showUpdateNotification();
            }
        });
    });
}

// Show update notification
function showUpdateNotification() {
    const notification = document.createElement('div');
    notification.className = 'update-notification';
    notification.style.cssText = `
        position: fixed;
        top: 1rem;
        left: 1rem;
        right: 1rem;
        background: var(--pico-primary);
        color: white;
        padding: 1rem;
        border-radius: var(--pico-border-radius);
        box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        z-index: 1002;
        display: flex;
        justify-content: space-between;
        align-items: center;
    `;
    notification.innerHTML = `
        <span>A new version is available!</span>
        <button onclick="location.reload()" style="background: white; color: var(--pico-primary); border: none;">
            Update Now
        </button>
    `;
    document.body.appendChild(notification);
}

// Request notification permission
function requestNotificationPermission() {
    if ('Notification' in window && Notification.permission === 'default') {
        Notification.requestPermission().then((permission) => {
            if (permission === 'granted') {
                console.log('Notification permission granted');
                // Subscribe to push notifications if service worker is ready
                subscribeUserToPush();
            }
        });
    }
}

// Subscribe to push notifications
async function subscribeUserToPush() {
    try {
        const registration = await navigator.serviceWorker.ready;
        const subscription = await registration.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: urlBase64ToUint8Array(VAPID_PUBLIC_KEY || '')
        });

        // Send subscription to server
        await fetch('/api/notifications/subscribe', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(subscription)
        });

        console.log('Push subscription successful');
    } catch (err) {
        console.error('Failed to subscribe to push:', err);
    }
}

// Helper for VAPID key conversion
function urlBase64ToUint8Array(base64String) {
    const padding = '='.repeat((4 - base64String.length % 4) % 4);
    const base64 = (base64String + padding).replace(/\-/g, '+').replace(/_/g, '/');
    const rawData = window.atob(base64);
    const outputArray = new Uint8Array(rawData.length);
    for (let i = 0; i < rawData.length; ++i) {
        outputArray[i] = rawData.charCodeAt(i);
    }
    return outputArray;
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    initTheme();

    // Check if user is online
    if (!navigator.onLine) {
        window.dispatchEvent(new Event('offline'));
    }

    // Request notification permission after a delay (better UX)
    setTimeout(() => {
        if (localStorage.getItem('notificationsPrompted') !== 'true') {
            requestNotificationPermission();
            localStorage.setItem('notificationsPrompted', 'true');
        }
    }, 5000);

    // Log page view (for analytics if implemented)
    console.log('Page loaded:', window.location.pathname);
});

// HTMX Configuration and Event Listeners
document.addEventListener('htmx:configRequest', (event) => {
    // Add CSRF token to all HTMX requests
    const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
    if (csrfToken) {
        event.detail.headers['X-CSRF-Token'] = csrfToken;
    }

    // Add authentication token from cookie
    const authToken = getCookie('auth_token');
    if (authToken) {
        event.detail.headers['Authorization'] = `Bearer ${authToken}`;
    }
});

document.body.addEventListener('htmx:afterSwap', (event) => {
    // Reinitialize any dynamic content
    console.log('Content swapped');

    // Trigger Alpine.js to reinitialize new elements
    if (window.Alpine) {
        window.Alpine.initTree(event.detail.elt);
    }
});

document.body.addEventListener('htmx:beforeRequest', (event) => {
    // Show loading indicator
    const target = event.detail.target;
    if (target) {
        target.setAttribute('aria-busy', 'true');
    }
});

document.body.addEventListener('htmx:afterRequest', (event) => {
    // Hide loading indicator
    const target = event.detail.target;
    if (target) {
        target.removeAttribute('aria-busy');
    }
});

document.body.addEventListener('htmx:responseError', (event) => {
    const status = event.detail.xhr.status;
    if (status === 401) {
        showToast('Session expired. Please log in again.', 'error');
        setTimeout(() => window.location.href = '/login', 2000);
    } else if (status === 403) {
        showToast('Access denied. Invalid CSRF token.', 'error');
    } else if (status === 429) {
        showToast('Too many requests. Please slow down.', 'error');
    } else {
        showToast('An error occurred. Please try again.', 'error');
    }
});

document.body.addEventListener('htmx:timeout', (event) => {
    showToast('Request timed out. Please check your connection.', 'error');
});

document.body.addEventListener('htmx:sendError', (event) => {
    showToast('Network error. Check your connection.', 'error');
});

// Cookie helper
function getCookie(name) {
    const value = `; ${document.cookie}`;
    const parts = value.split(`; ${name}=`);
    if (parts.length === 2) return parts.pop().split(';').shift();
}

// Export functions for use in templates
window.app = {
    formatDate,
    formatTime,
    showToast,
    confirmAction,
    hapticFeedback,
    installPWA,
    dismissInstallPrompt
};