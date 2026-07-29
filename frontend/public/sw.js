// service worker: caching for PWA so it can work offline
const CACHE = 'wordgo-v1';
const STATIC = ['/', '/frontend/script.js', '/frontend/style.css'];

self.addEventListener('install', e => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(STATIC)));
  self.skipWaiting();
});

self.addEventListener('activate', e => {
  e.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k)))
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', e => {
  const url = new URL(e.request.url);
  // API calls always go to network
  if (url.pathname.startsWith('/api/')) return;
  // Network-first: always serve the latest deployed asset when online, and
  // only fall back to the cache when offline. A cache-first strategy here
  // would keep serving whatever was cached at install time forever, since
  // CACHE's name never changes across deploys.
  e.respondWith(
    fetch(e.request)
      .then(res => {
        const copy = res.clone();
        caches.open(CACHE).then(c => c.put(e.request, copy));
        return res;
      })
      .catch(() => caches.match(e.request))
  );
});
