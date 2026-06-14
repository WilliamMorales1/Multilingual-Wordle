const CACHE = 'wordle-v1';
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
  e.respondWith(
    caches.match(e.request).then(cached => cached ?? fetch(e.request))
  );
});
