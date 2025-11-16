/**
 * Theme Management System
 * Handles dark/light mode switching with persistence
 */

class ThemeManager {
    constructor() {
        this.storageKey = 'injection-tracker-theme';
        this.mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
        this.init();
    }

    async init() {
        // Try to load theme from backend first (for authenticated users)
        let theme = null;
        const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;

        if (csrfToken) {
            try {
                const response = await fetch('/api/settings');
                if (response.ok) {
                    const settings = await response.json();
                    if (settings.theme) {
                        theme = settings.theme;
                        // If auto mode, determine actual theme based on time or system
                        if (theme === 'auto') {
                            const hour = new Date().getHours();
                            theme = (hour >= 6 && hour < 18) ? 'light' : 'dark';
                        }
                    }
                }
            } catch (err) {
                console.warn('Failed to load theme from backend:', err);
            }
        }

        // Fallback to localStorage or system preference
        if (!theme) {
            const savedTheme = localStorage.getItem(this.storageKey);
            const systemTheme = this.mediaQuery.matches ? 'dark' : 'light';
            theme = savedTheme || systemTheme;
        }

        this.setTheme(theme, false); // Don't save back to backend on init

        // Listen for system theme changes
        this.mediaQuery.addEventListener('change', (e) => {
            if (!localStorage.getItem(this.storageKey)) {
                this.setTheme(e.matches ? 'dark' : 'light', false);
            }
        });
    }

    setTheme(theme, saveToBackend = true) {
        const root = document.documentElement;
        root.setAttribute('data-theme', theme);

        // Update meta theme-color for mobile browsers
        const metaThemeColor = document.querySelector('meta[name="theme-color"]');
        if (metaThemeColor) {
            metaThemeColor.content = theme === 'dark' ? '#1C1917' : '#FAFAF9';
        }

        // Save preference locally
        localStorage.setItem(this.storageKey, theme);

        // Save to backend if user is authenticated and saveToBackend is true
        if (saveToBackend) {
            const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
            if (csrfToken) {
                fetch('/api/settings/app', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': csrfToken
                    },
                    body: JSON.stringify({
                        theme: theme,
                        advanced_mode: false // Will be overwritten by actual value from settings page
                    })
                }).catch(err => console.error('Failed to save theme:', err));
            }
        }

        // Dispatch custom event
        window.dispatchEvent(new CustomEvent('themechange', {
            detail: { theme }
        }));
    }

    toggleTheme() {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
        this.setTheme(newTheme);
        return newTheme;
    }

    getCurrentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'light';
    }

    // Auto theme based on time of day (6am - 6pm = light, else dark)
    setAutoTheme() {
        const hour = new Date().getHours();
        const isDaytime = hour >= 6 && hour < 18;
        this.setTheme(isDaytime ? 'light' : 'dark');
        localStorage.removeItem(this.storageKey); // Remove saved preference for auto mode
    }
}

// Initialize theme manager
window.themeManager = new ThemeManager();

// Alpine.js integration
document.addEventListener('alpine:init', () => {
    Alpine.data('theme', () => ({
        currentTheme: window.themeManager.getCurrentTheme(),

        init() {
            // Listen for theme changes
            window.addEventListener('themechange', (e) => {
                this.currentTheme = e.detail.theme;
            });
        },

        toggle() {
            return window.themeManager.toggleTheme();
        },

        set(theme) {
            window.themeManager.setTheme(theme);
        },

        setAuto() {
            window.themeManager.setAutoTheme();
            this.currentTheme = window.themeManager.getCurrentTheme();
        },

        get isDark() {
            return this.currentTheme === 'dark';
        },

        get isLight() {
            return this.currentTheme === 'light';
        },

        get icon() {
            return this.isDark ? '☼' : '☾';
        },

        get label() {
            return this.isDark ? 'Switch to Light Mode' : 'Switch to Dark Mode';
        }
    }));
});

// Export for use in other scripts
window.ThemeManager = ThemeManager;