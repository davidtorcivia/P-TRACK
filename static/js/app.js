/**
 * Injection Tracker - Enhanced JavaScript
 * Modern interactions and utilities
 */

// Initialize Alpine.js store for global state
document.addEventListener('alpine:init', () => {
    // Notification Store
    Alpine.store('notificationCount', {
        count: 0,

        init() {
            this.fetchCount();
            // Refresh every 30 seconds
            setInterval(() => this.fetchCount(), 30000);
        },

        fetchCount() {
            fetch('/api/notifications/count')
                .then(response => response.json())
                .then(data => {
                    this.count = data.count || 0;
                })
                .catch(error => {
                    console.error('Error fetching notification count:', error);
                });
        },

        increment() {
            this.count++;
        },

        decrement() {
            if (this.count > 0) this.count--;
        }
    });

    // App Store
    Alpine.store('app', {
        isLoading: false,
        sidebarOpen: false,

        startLoading() {
            this.isLoading = true;
        },

        stopLoading() {
            this.isLoading = false;
        },

        toggleSidebar() {
            this.sidebarOpen = !this.sidebarOpen;
        }
    });
});

// CSRF token handling for HTMX requests
document.addEventListener('htmx:configRequest', (event) => {
    const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
    if (csrfToken) {
        event.detail.headers['X-CSRF-Token'] = csrfToken;
    }
});

// Global loading states for HTMX
document.addEventListener('htmx:beforeRequest', () => {
    Alpine.store('app').startLoading();
});

document.addEventListener('htmx:afterRequest', () => {
    Alpine.store('app').stopLoading();
});

// Enhanced flash message handling
document.addEventListener('DOMContentLoaded', () => {
    // Auto-hide flash messages
    const flashMessages = document.querySelectorAll('.alert');
    flashMessages.forEach((message, index) => {
        setTimeout(() => {
            message.style.opacity = '0';
            message.style.transform = 'translateY(-10px)';
            setTimeout(() => message.remove(), 300);
        }, 5000 + (index * 200)); // Stagger multiple messages
    });

    // Initialize tooltips
    initializeTooltips();

    // Smooth scroll for anchor links
    initializeSmoothScroll();

    // Form enhancements
    enhanceForms();

    // Initialize existing app functionality
    initTheme();
    checkOfflineStatus();
    initPWA();
});

// Tooltip system
function initializeTooltips() {
    const tooltipElements = document.querySelectorAll('[x-tooltip]');

    tooltipElements.forEach(element => {
        const text = element.getAttribute('x-tooltip');
        let tooltip = null;

        element.addEventListener('mouseenter', (e) => {
            tooltip = document.createElement('div');
            tooltip.className = 'tooltip';
            tooltip.textContent = text;
            tooltip.style.cssText = `
                position: absolute;
                background: var(--color-bg-tertiary);
                color: var(--color-text-primary);
                padding: var(--space-2) var(--space-3);
                border-radius: var(--radius-md);
                font-size: var(--text-xs);
                font-weight: var(--font-medium);
                box-shadow: var(--shadow-lg);
                z-index: var(--z-tooltip);
                pointer-events: none;
                white-space: nowrap;
                opacity: 0;
                transform: translateY(-5px);
                transition: all 150ms ease;
            `;

            document.body.appendChild(tooltip);

            const rect = element.getBoundingClientRect();
            const tooltipRect = tooltip.getBoundingClientRect();

            // Position tooltip above element
            tooltip.style.left = rect.left + (rect.width / 2) - (tooltipRect.width / 2) + 'px';
            tooltip.style.top = rect.top - tooltipRect.height - 8 + 'px';

            // Animate in
            requestAnimationFrame(() => {
                tooltip.style.opacity = '1';
                tooltip.style.transform = 'translateY(0)';
            });
        });

        element.addEventListener('mouseleave', () => {
            if (tooltip) {
                tooltip.style.opacity = '0';
                tooltip.style.transform = 'translateY(-5px)';
                setTimeout(() => {
                    if (tooltip) {
                        tooltip.remove();
                        tooltip = null;
                    }
                }, 150);
            }
        });
    });
}

// Smooth scroll for anchor links
function initializeSmoothScroll() {
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function (e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                target.scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        });
    });
}

// Form enhancements
function enhanceForms() {
    // Auto-resize textareas
    const textareas = document.querySelectorAll('textarea');
    textareas.forEach(textarea => {
        textarea.addEventListener('input', () => {
            textarea.style.height = 'auto';
            textarea.style.height = textarea.scrollHeight + 'px';
        });
    });

    // Better focus handling
    const inputs = document.querySelectorAll('input, select, textarea');
    inputs.forEach(input => {
        input.addEventListener('focus', () => {
            input.parentElement.classList.add('focused');
        });

        input.addEventListener('blur', () => {
            input.parentElement.classList.remove('focused');
        });
    });
}

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

// Offline detection
function checkOfflineStatus() {
    window.addEventListener('online', () => {
        const indicator = document.querySelector('.offline-indicator');
        if (indicator) {
            indicator.remove();
        }
        showToast('Connection restored!', 'success');
        syncOfflineData();
    });

    window.addEventListener('offline', () => {
        if (!document.querySelector('.offline-indicator')) {
            const indicator = document.createElement('div');
            indicator.className = 'offline-indicator alert alert-warning';
            indicator.innerHTML = `
                <div class="flex items-center gap-2">
                    <span>⚠️</span>
                    <span>You are currently offline. Changes will be saved when connection is restored.</span>
                </div>
            `;
            document.body.prepend(indicator);
        }
        showToast('You are offline', 'warning');
    });

    // Check initial status
    if (!navigator.onLine) {
        window.dispatchEvent(new Event('offline'));
    }
}

// PWA functionality
function initPWA() {
    let deferredPrompt;

    window.addEventListener('beforeinstallprompt', (e) => {
        e.preventDefault();
        deferredPrompt = e;
        setTimeout(() => showInstallPrompt(), 3000);
    });

    function showInstallPrompt() {
        if (deferredPrompt) {
            showToast('Add Injection Tracker to your home screen for quick access!', 'info', 5000);
            // You could show a more prominent install UI here
        }
    }

    window.app = {
        ...window.app,
        installPWA: () => {
            if (deferredPrompt) {
                deferredPrompt.prompt();
                deferredPrompt.userChoice.then((choiceResult) => {
                    if (choiceResult.outcome === 'accepted') {
                        showToast('App installed successfully!', 'success');
                    }
                    deferredPrompt = null;
                });
            }
        }
    };
}

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

    // Ctrl/Cmd + /: Show keyboard shortcuts
    if ((e.ctrlKey || e.metaKey) && e.key === '/') {
        e.preventDefault();
        showKeyboardShortcuts();
    }
});

// Show keyboard shortcuts modal
function showKeyboardShortcuts() {
    const shortcuts = [
        { key: 'Ctrl + K', description: 'Quick injection log' },
        { key: 'Ctrl + /', description: 'Show keyboard shortcuts' },
        { key: 'Escape', description: 'Close modal' }
    ];

    let html = '<div class="space-y-2">';
    shortcuts.forEach(shortcut => {
        html += `
            <div class="flex justify-between">
                <span class="font-medium">${shortcut.description}</span>
                <kbd class="px-2 py-1 bg-surface border rounded text-xs">${shortcut.key}</kbd>
            </div>
        `;
    });
    html += '</div>';

    showModal('Keyboard Shortcuts', html);
}

// Simple modal system
function showModal(title, content) {
    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.innerHTML = `
        <div class="modal max-w-md">
            <div class="modal-header">
                <h3 class="modal-title">${title}</h3>
                <button class="modal-close" onclick="this.closest('.modal-overlay').remove()">✕</button>
            </div>
            <div class="modal-body">
                ${content}
            </div>
            <div class="modal-footer">
                <button class="btn btn-primary" onclick="this.closest('.modal-overlay').remove()">Close</button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);
    modal.addEventListener('click', (e) => {
        if (e.target === modal) {
            modal.remove();
        }
    });
}

// Utility functions
const Utils = {
    // Format dates
    formatDate: (date) => {
        return new Intl.DateTimeFormat('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        }).format(new Date(date));
    },

    // Format relative time
    timeAgo: (date) => {
        const now = new Date();
        const past = new Date(date);
        const diffMs = now - past;
        const diffSecs = Math.floor(diffMs / 1000);
        const diffMins = Math.floor(diffSecs / 60);
        const diffHours = Math.floor(diffMins / 60);
        const diffDays = Math.floor(diffHours / 24);

        if (diffSecs < 60) return 'just now';
        if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
        if (diffHours < 24) return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
        if (diffDays < 7) return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;

        return Utils.formatDate(date);
    },

    // Copy to clipboard
    copyToClipboard: async (text) => {
        try {
            await navigator.clipboard.writeText(text);
            showToast('Copied to clipboard!', 'success');
            return true;
        } catch (err) {
            console.error('Failed to copy: ', err);
            return false;
        }
    },

    // Download data as file
    downloadFile: (data, filename, type = 'text/plain') => {
        const blob = new Blob([data], { type });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
    },

    // Debounce function
    debounce: (func, wait) => {
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
};

// Toast notification system
function showToast(message, type = 'info', duration = 3000) {
    const toast = document.createElement('div');
    toast.className = `alert alert-${type} fixed top-4 right-4 z-50 max-w-sm shadow-lg`;
    toast.style.cssText = `
        animation: slideIn 0.3s ease;
    `;

    const icons = {
        success: '✅',
        error: '❌',
        warning: '⚠️',
        info: 'ℹ️'
    };

    toast.innerHTML = `
        <div class="flex items-center gap-2">
            <span>${icons[type] || icons.info}</span>
            <span>${message}</span>
        </div>
    `;

    document.body.appendChild(toast);

    setTimeout(() => {
        toast.style.animation = 'slideOut 0.3s ease';
        setTimeout(() => toast.remove(), 300);
    }, duration);
}

// Add animation keyframes
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

    .tooltip {
        animation: fadeIn 0.15s ease;
    }

    @keyframes fadeIn {
        from { opacity: 0; }
        to { opacity: 1; }
    }
`;
document.head.appendChild(style);

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
    if (e.target.matches('button, [role="button"], input[type="submit"], .btn')) {
        hapticFeedback('light');
    }
});

// Service Worker Registration
if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
        navigator.serviceWorker.register('/static/sw.js')
            .then((registration) => {
                console.log('Service Worker registered:', registration.scope);

                // Check for updates periodically
                setInterval(() => {
                    registration.update();
                }, 60000);

                // Handle update found
                registration.addEventListener('updatefound', () => {
                    const newWorker = registration.installing;
                    newWorker.addEventListener('statechange', () => {
                        if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                            showUpdateNotification();
                        }
                    });
                });
            })
            .catch((err) => {
                console.error('Service Worker registration failed:', err);
            });
    });
}

// Show update notification
function showUpdateNotification() {
    const notification = document.createElement('div');
    notification.className = 'alert alert-info fixed top-4 left-4 right-4 z-50 shadow-lg';
    notification.innerHTML = `
        <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
                <span>🔄</span>
                <span>A new version is available!</span>
            </div>
            <button onclick="location.reload()" class="btn btn-sm btn-primary">
                Update Now
            </button>
        </div>
    `;
    document.body.appendChild(notification);
}

// Export functions and utilities globally
window.Utils = Utils;
window.showToast = showToast;
window.hapticFeedback = hapticFeedback;
window.showModal = showModal;

// Enhanced HTMX error handling
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

// Performance monitoring
if ('performance' in window) {
    window.addEventListener('load', () => {
        setTimeout(() => {
            const perfData = performance.getEntriesByType('navigation')[0];
            console.log(`Page load time: ${Math.round(perfData.loadEventEnd - perfData.loadEventStart)}ms`);
        }, 0);
    });
}