#!/bin/bash
set -euo pipefail

# Bundle sushiro-overdose into a macOS .app
# Usage: ./scripts/bundle-macos.sh <binary-path> <version> <output-dir>

BINARY="${1:?Usage: $0 <binary> <version> <output-dir>}"
VERSION="${2:?}"
OUTPUT_DIR="${3:?}"

APP_NAME="Sushiro Overdose"
BUNDLE_ID="com.sushiro-overdose.app"
APP_DIR="${OUTPUT_DIR}/${APP_NAME}.app"

rm -rf "${APP_DIR}"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

cp "${BINARY}" "${APP_DIR}/Contents/MacOS/sushiro-overdose"
chmod +x "${APP_DIR}/Contents/MacOS/sushiro-overdose"

cat > "${APP_DIR}/Contents/Info.plist" << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>zh_CN</string>
    <key>CFBundleExecutable</key>
    <string>sushiro-overdose</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>LSApplicationCategoryType</key>
    <string>public.app-category.utilities</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <false/>
</dict>
</plist>
PLIST

cd "${OUTPUT_DIR}"
zip -r "${APP_NAME}-${VERSION}-macOS.zip" "${APP_NAME}.app"

echo "Created: ${OUTPUT_DIR}/${APP_NAME}-${VERSION}-macOS.zip"
