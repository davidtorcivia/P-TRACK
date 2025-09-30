// Service Worker for Injection Tracker PWA
// Version: 1.0.0 - Update this when deploying changes

const CACHE_VERSION = '1.0.0';
const CACHE_NAME = `injection-tracker-v${CACHE_VERSION}`;
const RUNTIME_CACHE = `injection-tracker-runtime-v${CACHE_VERSION}`;
const API_CACHE = `injection-tracker-api-v${CACHE_VERSION}`;

// Cache duration for API responses (15 minutes)
const API_CACHE_DURATION = 15 * 60 * 1000;

// Assets to cache on install
const STATIC_ASSETS = [
    '/',
    '/offline.html',
    '/static/css/custom.css',
    '/static/js/app.js',
    '/manifest.json',
    '/static/icons/icon-192.png',
    '/static/icons/icon-512.png',
    'https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css',
    'https://unpkg.com/htmx.org@1.9.10',
    'https://cdn.jsdelivr.net/npm/alpinejs@3.13.5/dist/cdn.min.js',
    'https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js'
];

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
                            name !== RUNTIME_CACHE &&
                            name !== API_CACHE
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

// Helper function to check if cache is fresh
function isCacheFresh(response) {
    if (!response) return false;
    const cachedDate = response.headers.get('sw-cache-date');
    if (!cachedDate) return false;
    const age = Date.now() - parseInt(cachedDate);
    return age < API_CACHE_DURATION;
}

// Helper function to add cache timestamp
function addCacheTimestamp(response) {
    const headers = new Headers(response.headers);
    headers.set('sw-cache-date', Date.now().toString());
    return new Response(response.body, {
        status: response.status,
        statusText: response.statusText,
        headers: headers
    });
}

// Fetch event - intelligent caching strategies
self.addEventListener('fetch', (event) => {
    const { request } = event;
    const url = new URL(request.url);

    // Skip non-GET requests
    if (request.method !== 'GET') {
        return;
    }

    // Skip Chrome extensions and other protocols
    if (!url.protocol.startsWith('http')) {
        return;
    }

    // API GET requests - network first with timed cache fallback
    if (url.pathname.startsWith('/api/')) {
        event.respondWith(
            fetch(request)
                .then((response) => {
                    // Clone and cache successful responses with timestamp
                    if (response.ok) {
                        const responseClone = addCacheTimestamp(response.clone());
                        caches.open(API_CACHE).then((cache) => {
                            cache.put(request, responseClone);
                        });
                    }
                    return response;
                })
                .catch(() => {
                    // Try to serve fresh cache if offline
                    return caches.open(API_CACHE)
                        .then((cache) => cache.match(request))
                        .then((cachedResponse) => {
                            if (cachedResponse && isCacheFresh(cachedResponse)) {
                                console.log('[SW] Serving fresh cached API response');
                                return cachedResponse;
                            }
                            // Return offline page for navigation requests
                            if (request.mode === 'navigate') {
                                return caches.match('/offline.html');
                            }
                            return new Response(
                                JSON.stringify({ error: 'Offline', cached: false }),
                                { status: 503, headers: { 'Content-Type': 'application/json' } }
                            );
                        });
                })
        );
        return;
    }

    // Static assets - cache first, fallback to network
    if (url.origin === location.origin || STATIC_ASSETS.includes(request.url)) {
        event.respondWith(
            caches.match(request)
                .then((cachedResponse) => {
                    if (cachedResponse) {
                        return cachedResponse;
                    }
                    return fetch(request).then((response) => {
                        if (response.ok) {
                            const responseClone = response.clone();
                            caches.open(CACHE_NAME).then((cache) => {
                                cache.put(request, responseClone);
                            });
                        }
                        return response;
                    });
                })
        );
        return;
    }

    // For everything else, try network first
    event.respondWith(
        fetch(request)
            .then((response) => {
                // Cache successful responses
                if (response.ok && request.url.startsWith(location.origin)) {
                    const responseClone = response.clone();
                    caches.open(RUNTIME_CACHE).then((cache) => {
                        cache.put(request, responseClone);
                    });
                }
                return response;
            })
            .catch(() => {
                return caches.match(request);
            })
    );
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