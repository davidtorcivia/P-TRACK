// Service Worker for Injection Tracker PWA
// Version: 1.1.0 - Update this when deploying changes
//
// PRIVACY: This app serves per-user medical data over authenticated, cookie-based
// requests. The service worker therefore MUST NOT cache authenticated API
// responses or rendered HTML pages — doing so would leak one user's data to the
// next user of a shared device (and serve stale CSRF tokens). Only truly static,
// non-user-specific assets (CSS/JS/icons/fonts) are cached.

const CACHE_VERSION = '1.1.0';
const CACHE_NAME = `injection-tracker-v${CACHE_VERSION}`;
const RUNTIME_CACHE = `injection-tracker-runtime-v${CACHE_VERSION}`;

// Static, non-sensitive assets to cache on install. Note: '/' and other HTML
// pages are intentionally excluded — they are authenticated and per-user.
const STATIC_ASSETS = [
    '/offline.html',
    '/static/css/app.css',
    '/static/css/custom.css',
    '/static/js/app.js',
    '/manifest.json',
    '/static/icons/icon-192.png',
    '/static/icons/icon-512.png',
    'https://unpkg.com/htmx.org@1.9.10',
    'https://cdn.jsdelivr.net/npm/alpinejs@3.13.5/dist/cdn.min.js',
    'https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js'
];

// Returns true for same-origin paths that are safe to cache (static assets only).
function isCacheableStatic(url) {
    return url.origin === location.origin && url.pathname.startsWith('/static/');
}

// Install event - cache static assets
self.addEventListener('install', (event) => {
    console.log('[SW] Installing service worker...');
    event.waitUntil(
        caches.open(CACHE_NAME)
            .then((cache) => {
                console.log('[SW] Caching static assets');
                return cache.addAll(STATIC_ASSETS).catch((err) => {
                    console.error('[SW] Failed to cache some assets:', err);
                    // Continue anyway - app will work with partial cache
                });
            })
            .then(() => {
                console.log('[SW] Service worker installed');
                return self.skipWaiting();
            })
    );
});

// Activate event - clean up old caches
self.addEventListener('activate', (event) => {
    console.log('[SW] Activating service worker...');
    event.waitUntil(
        caches.keys()
            .then((cacheNames) => {
                return Promise.all(
                    cacheNames
                        .filter((name) =>
                            name.startsWith('injection-tracker-') &&
                            name !== CACHE_NAME &&
                            name !== RUNTIME_CACHE
                        )
                        .map((name) => {
                            console.log('[SW] Deleting old cache:', name);
                            return caches.delete(name);
                        })
                );
            })
            .then(() => {
                console.log('[SW] Service worker activated');
                return self.clients.claim();
            })
    );
});

// Fetch event - privacy-preserving caching strategies
self.addEventListener('fetch', (event) => {
    const { request } = event;
    const url = new URL(request.url);

    // Only handle GET; never intercept state-changing requests.
    if (request.method !== 'GET') {
        return;
    }

    // Skip Chrome extensions and other protocols
    if (!url.protocol.startsWith('http')) {
        return;
    }

    // Authenticated API responses carry per-user medical data. Never cache them:
    // network-only, with a small offline JSON error so callers can degrade.
    if (url.pathname.startsWith('/api/')) {
        event.respondWith(
            fetch(request).catch(() =>
                new Response(
                    JSON.stringify({ error: 'Offline', cached: false }),
                    { status: 503, headers: { 'Content-Type': 'application/json' } }
                )
            )
        );
        return;
    }

    // HTML navigations are authenticated and per-user (and embed a per-request
    // CSRF token). Use network-only with an offline fallback page; never cache
    // the rendered HTML.
    if (request.mode === 'navigate') {
        event.respondWith(
            fetch(request).catch(() => caches.match('/offline.html'))
        );
        return;
    }

    // Static, non-sensitive assets - cache first, fall back to network.
    if (isCacheableStatic(url) || STATIC_ASSETS.includes(request.url)) {
        event.respondWith(
            caches.match(request).then((cachedResponse) => {
                if (cachedResponse) {
                    return cachedResponse;
                }
                return fetch(request).then((response) => {
                    if (response.ok) {
                        const responseClone = response.clone();
                        caches.open(RUNTIME_CACHE).then((cache) => {
                            cache.put(request, responseClone);
                        });
                    }
                    return response;
                });
            })
        );
        return;
    }

    // Everything else: pass through to the network (no caching).
});

// Background sync for offline submissions
self.addEventListener('sync', (event) => {
    console.log('[SW] Background sync triggered:', event.tag);
    if (event.tag === 'sync-injections') {
        event.waitUntil(syncInjections());
    } else if (event.tag === 'sync-symptoms') {
        event.waitUntil(syncSymptoms());
    } else if (event.tag === 'sync-medications') {
        event.waitUntil(syncMedications());
    }
});

async function syncInjections() {
    console.log('[SW] Syncing offline injections...');
    try {
        // Open IndexedDB and get pending injections
        const db = await openDB();
        const tx = db.transaction('pending_injections', 'readonly');
        const store = tx.objectStore('pending_injections');
        const pending = await store.getAll();

        // Sync each pending injection
        for (const injection of pending) {
            try {
                const response = await fetch('/api/injections', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(injection.data)
                });

                if (response.ok) {
                    // Remove from pending queue
                    const deleteTx = db.transaction('pending_injections', 'readwrite');
                    await deleteTx.objectStore('pending_injections').delete(injection.id);
                    console.log('[SW] Synced injection:', injection.id);
                }
            } catch (err) {
                console.error('[SW] Failed to sync injection:', err);
            }
        }
    } catch (err) {
        console.error('[SW] Sync failed:', err);
    }
}

async function syncSymptoms() {
    console.log('[SW] Syncing offline symptoms...');
    // Similar implementation to syncInjections
}

async function syncMedications() {
    console.log('[SW] Syncing offline medications...');
    // Similar implementation to syncInjections
}

// IndexedDB helper
function openDB() {
    return new Promise((resolve, reject) => {
        const request = indexedDB.open('InjectionTrackerDB', 1);

        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve(request.result);

        request.onupgradeneeded = (event) => {
            const db = event.target.result;
            if (!db.objectStoreNames.contains('pending_injections')) {
                db.createObjectStore('pending_injections', { keyPath: 'id', autoIncrement: true });
            }
            if (!db.objectStoreNames.contains('pending_symptoms')) {
                db.createObjectStore('pending_symptoms', { keyPath: 'id', autoIncrement: true });
            }
            if (!db.objectStoreNames.contains('pending_medications')) {
                db.createObjectStore('pending_medications', { keyPath: 'id', autoIncrement: true });
            }
        };
    });
}

// Push notifications
self.addEventListener('push', (event) => {
    const data = event.data ? event.data.json() : {};
    const title = data.title || 'Injection Reminder';
    const options = {
        body: data.body || 'Time for your injection',
        icon: '/static/icons/icon-192.png',
        badge: '/static/icons/badge-72.png',
        vibrate: [200, 100, 200],
        data: {
            url: data.url || '/'
        },
        actions: [
            {
                action: 'log',
                title: 'Log Injection'
            },
            {
                action: 'dismiss',
                title: 'Dismiss'
            }
        ]
    };

    event.waitUntil(
        self.registration.showNotification(title, options)
    );
});

// Notification click handler
self.addEventListener('notificationclick', (event) => {
    event.notification.close();

    if (event.action === 'log') {
        event.waitUntil(
            clients.openWindow('/?action=log-injection')
        );
    } else {
        event.waitUntil(
            clients.openWindow(event.notification.data.url || '/')
        );
    }
});

// Message handler for communication with app
self.addEventListener('message', (event) => {
    if (event.data && event.data.type === 'SKIP_WAITING') {
        self.skipWaiting();
    }

    if (event.data && event.data.type === 'CLEAR_CACHE') {
        event.waitUntil(
            caches.keys().then((cacheNames) => {
                return Promise.all(
                    cacheNames.map((name) => caches.delete(name))
                );
            })
        );
    }
});

// Update notification - inform user when new version is available
self.addEventListener('controllerchange', () => {
    // Send message to all clients
    self.clients.matchAll().then(clients => {
        clients.forEach(client => {
            client.postMessage({
                type: 'SW_UPDATED',
                message: 'A new version is available. Refresh to update.'
            });
        });
    });
});

console.log('[SW] Service Worker loaded - Version:', CACHE_VERSION);