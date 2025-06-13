#!/bin/bash

# Create a build directory
mkdir -p build

# Copy necessary files to build directory
cp manifest.json build/
cp content.js build/
cp popup.html build/
cp popup.js build/
cp background.js build/
cp settings.html build/
cp settings.js build/
cp README.md build/
cp icon.svg build/

# Create a ZIP file
cd build
zip -r ../ai-capitalizer.zip *

# Clean up
cd ..
rm -rf build

echo "Extension packaged as ai-capitalizer.zip"
echo "Next steps:"
echo "1. Go to https://chrome.google.com/webstore/devconsole"
echo "2. Sign in with your Google account"
echo "3. Click 'New Item'"
echo "4. Upload ai-capitalizer.zip"
echo "5. Fill in the following required information:"
echo "   - Detailed description"
echo "   - At least 2 screenshots"
echo "   - A 128x128 icon (use icon.svg)"
echo "   - A 440x280 small tile image"
echo "6. Pay the one-time $5 developer registration fee"
echo "7. Submit for review" 