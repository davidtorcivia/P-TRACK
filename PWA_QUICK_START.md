# PWA Quick Start Guide
## Get Your App Running as a PWA in 5 Minutes

---

## Step 1: Generate Icons (Required)

### Option A: Browser-Based (Easiest)
1. Open `static/icons/generate-icons.html` in your browser
2. Click "Download All Icons"
3. Files automatically saved as `icon-192.png` and `icon-512.png`
4. Move files to `static/icons/` directory

### Option B: Online Converter
1. Go to https://svgtopng.com/
2. Paste SVG code from `static/icons/ICON_GENERATION_INSTRUCTIONS.md`
3. Generate 192x192 and 512x512 versions
4. Save as `icon-192.png` and `icon-512.png` in `static/icons/`

---

## Step 2: Verify File Structure

Your project should have these files:

```
static/
├── css/
│   └── custom.css ✅
├── js/
│   └── app.js ✅
├── icons/
│   ├── icon-192.png ⚠️ (you need to generate this)
│   ├── icon-512.png ⚠️ (you need to generate this)
│   ├── generate-icons.html ✅
│   └── ICON_GENERATION_INSTRUCTIONS.md ✅
├── manifest.json ✅
├── sw.js ✅
└── offline.html ✅
```

---

## Step 3: Test Locally

### Start the Server
```bash
cd "C:\Users\David\Resilio Sync\CODE\P-TRACK"
go run cmd/server/main.go
```

### Open in Browser
```
http://localhost:8080
```

### Open DevTools
1. Press F12
2. Go to "Application" tab (Chrome) or "Storage" tab (Firefox)
3. Check "Service Workers" section

---

## Step 4: Verify PWA Installation

### Chrome DevTools Checklist
1. **Application → Manifest**
   - ✅ Name, short_name, icons visible
   - ✅ No manifest errors

2. **Application → Service Workers**
   - ✅ Service worker registered
   - ✅ Status: "activated and is running"

3. **Application → Cache Storage**
   - ✅ Three caches created:
     - `injection-tracker-v1.0.0`
     - `injection-tracker-runtime-v1.0.0`
     - `injection-tracker-api-v1.0.0`

4. **Network Tab**
   - ✅ Reload page - resources served from Service Worker
   - ✅ Look for "(from ServiceWorker)" label

---

## Step 5: Test Offline Mode

### Method 1: DevTools
1. Open DevTools → Network tab
2. Check "Offline" checkbox
3. Reload page
4. ✅ App should still work with cached data
5. ✅ Offline page should show for uncached routes

### Method 2: Airplane Mode
1. Enable airplane mode on your device
2. Open the app
3. ✅ Should work with cached data

---

## Step 6: Test Installation

### Desktop (Chrome/Edge)
1. Look for install icon in address bar (⊕ or 🖥️)
2. Click install button
3. App opens in standalone window
4. ✅ No browser UI visible

### Mobile (Android Chrome)
1. Tap menu (⋮)
2. Select "Install app" or "Add to Home screen"
3. Confirm installation
4. ✅ App icon appears on home screen
5. ✅ Opens in fullscreen mode

### iOS Safari
1. Tap Share button (↑)
2. Scroll down, tap "Add to Home Screen"
3. Confirm
4. ✅ App icon appears on home screen

---

## Troubleshooting

### Service Worker Not Registering

**Problem:** No service worker in DevTools

**Solutions:**
- ✅ Check browser console for errors
- ✅ Verify `/service-worker.js` returns 200 status
- ✅ Clear browser cache and hard reload (Ctrl+Shift+R)
- ✅ Check Content-Type header: `application/javascript`

### Install Prompt Not Appearing

**Problem:** No install button visible

**Solutions:**
- ✅ Verify icons exist (192px and 512px)
- ✅ Check manifest.json is valid (no JSON errors)
- ✅ Ensure HTTPS in production (or localhost for dev)
- ✅ Service worker must be registered and active
- ✅ User must visit site at least once

### Caching Issues

**Problem:** Changes not appearing after deploy

**Solutions:**
- ✅ Increment `CACHE_VERSION` in `sw.js`
- ✅ Clear cache: DevTools → Application → Clear Storage
- ✅ Check "Update on reload" in Service Workers section
- ✅ Hard reload: Ctrl+Shift+R (Windows) or Cmd+Shift+R (Mac)

### Offline Mode Not Working

**Problem:** App doesn't work offline

**Solutions:**
- ✅ Check Cache Storage contains files
- ✅ Verify fetch events in DevTools → Network
- ✅ Look for "from ServiceWorker" label on requests
- ✅ Check console for service worker errors

### Icons Not Showing

**Problem:** Default browser icon appears

**Solutions:**
- ✅ Generate icons using provided tools
- ✅ Verify paths in manifest.json: `/static/icons/icon-XXX.png`
- ✅ Check icon files are PNG format
- ✅ Ensure correct sizes: 192x192 and 512x512
- ✅ Clear browser cache

---

## Testing with Lighthouse

### Run PWA Audit
1. Open DevTools → Lighthouse tab
2. Select "Progressive Web App" category
3. Click "Generate report"
4. ✅ Target score: 90+

### Expected Results
- ✅ Registers a service worker
- ✅ Responds with 200 when offline
- ✅ Has a web app manifest
- ✅ Uses HTTPS (production only)
- ✅ Redirects HTTP to HTTPS
- ✅ Configured for custom splash screen
- ✅ Sets theme color
- ✅ Content sized correctly for viewport
- ✅ Page load fast on mobile

---

## Production Deployment

### Prerequisites
1. **HTTPS Certificate**
   ```bash
   # Using Certbot (Let's Encrypt)
   sudo certbot --nginx -d yourdomain.com
   ```

2. **Nginx Configuration**
   ```nginx
   # Add to nginx.conf
   location /service-worker.js {
       add_header Cache-Control "no-cache, no-store, must-revalidate";
       add_header Service-Worker-Allowed "/";
   }

   location /manifest.json {
       add_header Cache-Control "public, max-age=3600";
       add_header Content-Type "application/manifest+json";
   }
   ```

3. **Environment Variables**
   ```bash
   # .env file
   HTTPS_ENABLED=true
   DOMAIN=yourdomain.com
   ```

### Deploy Checklist
- [ ] Generate production icons (high quality)
- [ ] Update manifest.json with production URLs
- [ ] Increment service worker version
- [ ] Enable HTTPS redirect
- [ ] Test on multiple devices
- [ ] Run Lighthouse audit
- [ ] Monitor service worker errors

---

## Common Commands

### Clear Service Worker
```javascript
// In browser console
navigator.serviceWorker.getRegistrations()
  .then(registrations => {
    registrations.forEach(reg => reg.unregister())
  })
```

### Clear All Caches
```javascript
// In browser console
caches.keys()
  .then(keys => Promise.all(keys.map(key => caches.delete(key))))
```

### Force Update Service Worker
```javascript
// In browser console
navigator.serviceWorker.ready
  .then(registration => registration.update())
```

### Check Cache Contents
```javascript
// In browser console
caches.open('injection-tracker-v1.0.0')
  .then(cache => cache.keys())
  .then(keys => console.log(keys.map(k => k.url)))
```

---

## Performance Tips

### Optimize First Load
1. Defer non-critical JavaScript
2. Preload critical resources
3. Compress assets (gzip/brotli)
4. Minify CSS and JavaScript

### Reduce Cache Size
1. Cache only essential assets
2. Set appropriate cache expiry
3. Use network-first for frequently changing data
4. Implement cache size limits

### Improve Offline Experience
1. Show offline indicator
2. Queue failed requests in IndexedDB
3. Provide meaningful offline fallback
4. Auto-retry when connection restored

---

## Next Steps

### After PWA is Working
1. [ ] Configure push notification VAPID keys
2. [ ] Set up background sync for reminders
3. [ ] Implement app update notifications
4. [ ] Add analytics to track install rate
5. [ ] Monitor service worker errors in production
6. [ ] Optimize cache strategy based on usage
7. [ ] Add more offline functionality
8. [ ] Create app screenshots for manifest

### Optional Enhancements
- [ ] Add Web Share API for sharing reports
- [ ] Implement Badging API for notification count
- [ ] Add File System Access for data export
- [ ] Configure shortcuts in manifest
- [ ] Add dark mode splash screen

---

## Support & Resources

### Documentation
- Full implementation details: `PWA_IMPLEMENTATION_SUMMARY.md`
- Icon generation: `static/icons/ICON_GENERATION_INSTRUCTIONS.md`
- Project overview: `CLAUDE.md`

### External Resources
- [MDN PWA Guide](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps)
- [Google PWA Checklist](https://web.dev/pwa-checklist/)
- [Can I Use - PWA Features](https://caniuse.com/?search=service%20worker)

### Testing Tools
- [Lighthouse](https://developers.google.com/web/tools/lighthouse)
- [PWA Builder](https://www.pwabuilder.com/)
- [Chrome DevTools](chrome://inspect/#service-workers)

---

## Quick Reference

| Feature | File | Status |
|---------|------|--------|
| Service Worker | `static/sw.js` | ✅ Complete |
| Manifest | `static/manifest.json` | ✅ Complete |
| App JavaScript | `static/js/app.js` | ✅ Complete |
| Custom CSS | `static/css/custom.css` | ✅ Complete |
| Offline Page | `static/offline.html` | ✅ Complete |
| Icons 192px | `static/icons/icon-192.png` | ⚠️ Generate |
| Icons 512px | `static/icons/icon-512.png` | ⚠️ Generate |
| Server Handlers | `cmd/server/main.go` | ✅ Complete |

---

**Ready to go?** Just generate the icons and start testing!

**Need help?** Check `PWA_IMPLEMENTATION_SUMMARY.md` for detailed documentation.