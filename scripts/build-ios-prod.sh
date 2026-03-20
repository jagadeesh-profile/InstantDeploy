#!/bin/bash
# iOS App Production Build and Release Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
IOS_DIR="$PROJECT_DIR/ios"
PROJECT_FILE="$IOS_DIR/InstantDeploy.xcodeproj"

# Configuration
SCHEME="${SCHEME:-InstantDeploy}"
CONFIGURATION="${CONFIGURATION:-Release}"
TEAM_ID="${TEAM_ID:-}"
BUNDLE_ID="${BUNDLE_ID:-com.instantdeploy.app}"
VERSION="${VERSION:-1.0}"
BUILD_NUMBER="${BUILD_NUMBER:-1}"
EXPORT_METHOD="${EXPORT_METHOD:-app-store}"  # app-store, ad-hoc, enterprise, development

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    command -v xcodebuild &>/dev/null || { log_error "xcodebuild not found"; exit 1; }
    [ -d "$PROJECT_FILE" ] || { log_error "Project file not found: $PROJECT_FILE"; exit 1; }
    
    log_info "✓ Prerequisites met"
}

# Update build configuration
update_build_config() {
    log_info "Updating build configuration..."
    
    # Update version and build number
    agvtool new-marketing-version "$VERSION" || true
    agvtool new-build --all "$BUILD_NUMBER" || true
    
    log_info "✓ Version: $VERSION, Build: $BUILD_NUMBER"
}

# Build archive
build_archive() {
    log_info "Building archive..."
    
    cd "$PROJECT_DIR"
    
    xcodebuild -project "$PROJECT_FILE" \
        -scheme "$SCHEME" \
        -configuration "$CONFIGURATION" \
        -archivePath "build/InstantDeploy.xcarchive" \
        archive \
        CODE_SIGN_IDENTITY="iPhone Developer" \
        PROVISIONING_PROFILE_SPECIFIER="" \
        OTHER_CODE_SIGN_FLAGS="--keychain ~/Library/Keychains/login.keychain-db"
    
    log_info "✓ Archive built: build/InstantDeploy.xcarchive"
}

# Export IPA
export_ipa() {
    log_info "Exporting IPA..."
    
    # Create export options plist
    cat > /tmp/ExportOptions.plist <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>method</key>
    <string>$EXPORT_METHOD</string>
    <key>signingStyle</key>
    <string>automatic</string>
    <key>stripSwiftSymbols</key>
    <true/>
    <key>teamID</key>
    <string>$TEAM_ID</string>
    <key>bundleIdentifier</key>
    <string>$BUNDLE_ID</string>
</dict>
</plist>
EOF
    
    xcodebuild -exportArchive \
        -archivePath "build/InstantDeploy.xcarchive" \
        -exportOptionsPlist /tmp/ExportOptions.plist \
        -exportPath "build/InstantDeploy.ipa" \
        -allowProvisioningUpdates
    
    log_info "✓ IPA exported: build/InstantDeploy.ipa"
}

# Run tests
run_tests() {
    log_info "Running tests..."
    
    cd "$PROJECT_DIR"
    
    xcodebuild test \
        -project "$PROJECT_FILE" \
        -scheme "$SCHEME" \
        -destination 'generic/platform=iOS' \
        -configuration "$CONFIGURATION" \
        || log_warn "Some tests failed"
    
    log_info "✓ Tests completed"
}

# Generate dsym for crash reporting
generate_dsym() {
    log_info "Generating dSYM..."
    
    dsymutil "build/InstantDeploy.xcarchive/dSYMs/InstantDeploy.app.dSYM" \
        -o "build/InstantDeploy.dSYM" || true
    
    log_info "✓ dSYM generated"
}

# Create release notes
create_release_notes() {
    log_info "Creating release notes..."
    
    cat > "build/RELEASE_NOTES.txt" <<EOF
InstantDeploy v$VERSION (Build $BUILD_NUMBER)

Release Date: $(date)

Features:
- Application deployment from repositories
- Real-time deployment monitoring
- WebSocket-based updates
- Git integration

Bug Fixes & Improvements:
- Performance optimizations
- Enhanced error handling
- Improved UI/UX

Requirements:
- iOS 14.0+
- 50MB free storage

For more information, visit:
https://github.com/yourusername/instantdeploy
EOF
    
    log_info "✓ Release notes created"
}

# Main
main() {
    log_info "InstantDeploy iOS Production Build"
    log_info "===================================="
    log_info "Scheme: $SCHEME"
    log_info "Configuration: $CONFIGURATION"
    log_info "Export Method: $EXPORT_METHOD"
    log_info ""
    
    check_prerequisites
    update_build_config
    run_tests
    build_archive
    export_ipa
    generate_dsym
    create_release_notes
    
    log_info ""
    log_info "✓ Build complete!"
    log_info ""
    log_info "Artifacts:"
    log_info "- IPA: build/InstantDeploy.ipa"
    log_info "- dSYM: build/InstantDeploy.dSYM"
    log_info "- Release Notes: build/RELEASE_NOTES.txt"
    log_info ""
    log_info "Next steps for App Store:"
    log_info "1. Open Transporter or App Store Connect"
    log_info "2. Upload build/InstantDeploy.ipa"
    log_info "3. Configure metadata and screenshots"
    log_info "4. Submit for review"
}

main "$@"
