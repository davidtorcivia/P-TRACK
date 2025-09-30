# Icon Generation Instructions

This document provides SVG code and instructions for generating PWA icons for the Injection Tracker app.

## Base SVG Icon

Save this SVG as `icon-base.svg` and convert to PNG at required sizes:

```svg
<svg width="512" height="512" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
  <!-- Background -->
  <rect width="512" height="512" rx="115" fill="#6366f1"/>

  <!-- Syringe Icon -->
  <g transform="translate(128, 128) scale(4)">
    <!-- Plunger -->
    <rect x="2" y="8" width="8" height="32" rx="2" fill="#ffffff"/>

    <!-- Barrel -->
    <rect x="0" y="40" width="12" height="40" rx="2" fill="#e0e7ff" stroke="#ffffff" stroke-width="1"/>

    <!-- Needle base -->
    <rect x="4" y="80" width="4" height="4" fill="#d1d5db"/>

    <!-- Needle -->
    <path d="M 5 84 L 6 96 L 7 84 Z" fill="#9ca3af"/>

    <!-- Measurement marks -->
    <line x1="2" y1="50" x2="4" y2="50" stroke="#6366f1" stroke-width="0.5"/>
    <line x1="2" y1="60" x2="4" y2="60" stroke="#6366f1" stroke-width="0.5"/>
    <line x1="2" y1="70" x2="4" y2="70" stroke="#6366f1" stroke-width="0.5"/>
  </g>

  <!-- Medical Cross Accent -->
  <g transform="translate(320, 80)">
    <rect x="10" y="0" width="12" height="48" rx="3" fill="#ffffff" opacity="0.3"/>
    <rect x="0" y="18" width="32" height="12" rx="3" fill="#ffffff" opacity="0.3"/>
  </g>
</svg>
```

## Generation Methods

### Method 1: Online Converter (Easiest)
1. Copy the SVG code above
2. Go to https://svgtopng.com/ or https://cloudconvert.com/svg-to-png
3. Paste the SVG code
4. Generate two versions:
   - `icon-192.png` at 192x192 pixels
   - `icon-512.png` at 512x512 pixels
5. Save both files to `static/icons/` directory

### Method 2: Using ImageMagick (Command Line)
```bash
# Install ImageMagick if not already installed
# On Windows with Chocolatey:
choco install imagemagick

# Convert SVG to PNG
magick convert icon-base.svg -resize 192x192 icon-192.png
magick convert icon-base.svg -resize 512x512 icon-512.png
```

### Method 3: Using Inkscape (GUI)
1. Install Inkscape: https://inkscape.org/
2. Open the SVG file in Inkscape
3. File → Export PNG Image
4. Set width/height to 192 or 512
5. Export to `static/icons/` directory

### Method 4: Using Node.js (Automated)
```bash
npm install sharp

# Create convert.js:
```

```javascript
const sharp = require('sharp');
const fs = require('fs');

const svgBuffer = fs.readFileSync('icon-base.svg');

// Generate 192x192
sharp(svgBuffer)
  .resize(192, 192)
  .png()
  .toFile('static/icons/icon-192.png');

// Generate 512x512
sharp(svgBuffer)
  .resize(512, 512)
  .png()
  .toFile('static/icons/icon-512.png');
```

## Additional Icons (Optional)

### Badge Icon (72x72)
For notification badges, create a simplified version:

```svg
<svg width="72" height="72" viewBox="0 0 72 72" xmlns="http://www.w3.org/2000/svg">
  <rect width="72" height="72" rx="16" fill="#6366f1"/>
  <text x="36" y="50" font-family="Arial, sans-serif" font-size="48" font-weight="bold" fill="#ffffff" text-anchor="middle">💉</text>
</svg>
```

Save as `badge-72.png`

### Maskable Icon (Adaptive Icon)
For better display on Android devices, create a version with safe zone:

```svg
<svg width="512" height="512" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
  <!-- Add 10% padding for safe zone -->
  <rect width="512" height="512" fill="#6366f1"/>
  <g transform="translate(51.2, 51.2) scale(0.8)">
    <!-- Same content as base icon -->
  </g>
</svg>
```

## Favicon Generation

For browser favicon support, generate these additional sizes:

```bash
# 16x16 for browser tab
magick convert icon-base.svg -resize 16x16 favicon-16.png

# 32x32 for browser tab
magick convert icon-base.svg -resize 32x32 favicon-32.png

# Create multi-size ICO
magick convert icon-base.svg -resize 16x16 -resize 32x32 -resize 48x48 favicon.ico
```

## Apple Touch Icons

For iOS home screen:

```bash
# 180x180 for modern iOS devices
magick convert icon-base.svg -resize 180x180 apple-touch-icon.png
```

Add to HTML:
```html
<link rel="apple-touch-icon" href="/static/icons/apple-touch-icon.png">
```

## File Checklist

Once generated, you should have these files in `static/icons/`:

- [x] `icon-192.png` - Required for PWA manifest
- [x] `icon-512.png` - Required for PWA manifest
- [ ] `badge-72.png` - Optional for notifications
- [ ] `apple-touch-icon.png` - Optional for iOS
- [ ] `favicon.ico` - Optional for browser tab

## Testing Icons

After generating icons:

1. Clear browser cache
2. Refresh the app
3. Check DevTools → Application → Manifest
4. Verify all icons load correctly
5. Try installing the PWA to home screen

## Alternative: Use Emoji (Quick Start)

If you need icons immediately for testing, use emoji as placeholder:

```javascript
// In manifest.json, you can reference a simple colored square
// Generate programmatically:
const canvas = document.createElement('canvas');
canvas.width = 512;
canvas.height = 512;
const ctx = canvas.getContext('2d');
ctx.fillStyle = '#6366f1';
ctx.fillRect(0, 0, 512, 512);
ctx.font = 'bold 256px Arial';
ctx.fillStyle = '#ffffff';
ctx.textAlign = 'center';
ctx.textBaseline = 'middle';
ctx.fillText('💉', 256, 256);

canvas.toBlob(blob => {
  // Save as icon-512.png
});
```

## Updating Icons

When you update icons:

1. Update the version in `manifest.json`
2. Update `CACHE_VERSION` in `sw.js`
3. Clear service worker cache
4. Test on multiple devices

## Resources

- PWA Icon Guidelines: https://web.dev/add-manifest/
- Maskable Icon Editor: https://maskable.app/
- Icon Generator Tool: https://realfavicongenerator.net/
- SVG to PNG Converter: https://svgtopng.com/