# iOS Mobile App Deployment Guide

## Overview

This guide covers building, testing, and deploying the InstantDeploy iOS app to the App Store and TestFlight.

## Prerequisites

- **Xcode**: 14.0+
- **Apple Developer Account**: Team membership
- **iOS SDK**: 14.0+
- **Provisioning Profiles**: App Store, Ad Hoc, or development
- **Code Signing Certificate**: iOS Development or Distribution

## Architecture

### App Structure

```
InstantDeploy/
├── Models/
│   ├── Deployment.swift
│   └── User.swift
├── Services/
│   ├── APIService.swift
│   ├── WebSocketService.swift
│   └── StorageService.swift
├── ViewModels/
│   ├── DashboardViewModel.swift
│   ├── DeploymentViewModel.swift
│   └── AuthViewModel.swift
├── Views/
│   ├── ContentView.swift
│   ├── DashboardView.swift
│   ├── DeploymentListView.swift
│   └── LoginView.swift
└── Config.swift
```

### API Integration

 The app communicates with the backend API:

```
App ─────► Backend (REST API)
    ─────► Backend (WebSocket)
```

## Local Development Setup

### 1. Configure for Development

```swift
// Config.swift configured automatically for DEBUG builds
// Uses: http://localhost:8080
```

### 2. Build and Run

```bash
# Open in Xcode
open ios/InstantDeploy.xcodeproj

# Or build from CLI
xcodebuild -scheme InstantDeploy build
```

## Production Build

### 1. Update Version

```bash
# Edit version in project
# Xcode: Project > InstantDeploy > Build Settings > Version
VERSION=1.0 BUILD_NUMBER=1

# Or use script
scripts/build-ios-prod.sh --version 1.0 --build 1
```

### 2. Build for App Store

```bash
export SCHEME=InstantDeploy
export CONFIGURATION=Release
export EXPORT_METHOD=app-store
export TEAM_ID="your-team-id"

chmod +x scripts/build-ios-prod.sh
./scripts/build-ios-prod.sh
```

### 3. Upload to App Store Connect

```bash
# Using Transporter (Apple's official tool)
transporter -t upload -f build/InstantDeploy.ipa \
  -u your-email@example.com \
  -p your-app-specific-password

# Or via Xcode
# Select: Xcode > Organizer > Distribute App
```

## Deployment Methods

### TestFlight (Beta Testing)

```bash
# Build with TestFlight export method
export EXPORT_METHOD=ad-hoc
./scripts/build-ios-prod.sh

# Upload via Transporter or Xcode Organizer
# Distribute App > TestFlight
```

**Add Testers**:
1. App Store Connect > TestFlight > Internal Testing
2. Add developer team members
3. External Testing: Create external group, invite testers

### App Store Release

#### Pre-Release Checklist

- [ ] Version updated in Config.swift
- [ ] Build number incremented
- [ ] All tests passing
- [ ] Screenshots captured (6.5" display)
- [ ] App preview video created
- [ ] Privacy policy URL added
- [ ] Support URL added
- [ ] Keywords optimized
- [ ] Description updated
- [ ] Category selected
- [ ] Age rating completed
- [ ] Encryption declaration completed
- [ ] Pricing tier set

#### Submission Process

1. **Prepare Build**
   ```bash
   ./scripts/build-ios-prod.sh --version 1.0 --build 1
   ```

2. **Upload to App Store Connect**
   - Use Transporter app
   - Wait for processing (5-15 minutes)

3. **Configure Release**
   - App Store Connect > My Apps > InstantDeploy
   - Add screenshots, description, keywords
   - Complete App Rating Questionnaire
   - Review encryption and privacy settings

4. **Submit for Review**
   - Review Section > Submit for Review
   - Select build to review
   - Accept export compliance
   - Click "Submit for Review"

5. **Monitor Review Status**
   - Typical review time: 24-48 hours
   - Check App Store Connect for status
   - App will move through: "Waiting for Review" → "In Review" → "Ready for Sale"

#### Release Stages

```
┌──────────────────┐
│  Submitted Build │
│  (In Review)     │
└────────┬─────────┘
         │
         │ Apple Review (24-48h)
         ▼
┌──────────────────────────┐
│  Approved / Rejected     │
│  - If approved:          │
│    └─ Ready for Sale     │
│  - If rejected:          │
│    └─ Resubmit after fix │
└──────────────────────────┘
```

## App Analytics

### Install Tracking

The app automatically sends analytics events:

```swift
// In APIService
func trackEvent(_ name: String, parameters: [String: Any]? = nil) {
    // Send to backend
    // Backend sends to analytics service
}
```

### Key Metrics

- **Daily Active Users (DAU)**
- **Monthly Active Users (MAU)**
- **Retention Rate** (Day 1, Day 7, Day 30)
- **Feature Usage**
- **Error Rate**
- **Crash Rate**

## Crash Reporting

### Integrated Reporting

The app auto-reports crashes via:

```swift
// Sentry integration
import Sentry

// Automatic crash reporting
try? Sentry.captureException(error)
```

## Security & Privacy

### Data Encryption

- **In Transit**: TLS 1.2+ (all HTTPS/WSS)
- **At Rest**: Keychain for tokens, CoreData for app data

### Permissions

The app requires:

- **Network**: For API communication
- **Camera/Microphone** (optional): For future features

### Privacy Policy

Required for App Store:
- Data collection practices
- How user data is used
- Third-party services
- User rights (access, deletion)

## Troubleshooting

### Build Fails

```bash
# Clean build
xcodebuild clean -scheme InstantDeploy

# Clear cache
rm -rf ~/Library/Developer/Xcode/DerivedData/*

# Rebuild
xcodebuild -scheme InstantDeploy build
```

### Code Signing Issues

```bash
# List available identities
security find-identity -v -p codesigning

# Fix provisioning profiles
xcodebuild -scheme InstantDeploy -allowProvisioningUpdates build
```

### API Connection Fails

1. **Check Config.swift** - Correct API URL
2. **Test manually**: `curl https://api.instantdeploy.example.com/health`
3. **Check network** - Is device on WiFi/cellular?

### TestFlight Upload Fails

```bash
# Verify build
xcodebuild -exportArchive \
  -archivePath "build/InstantDeploy.xcarchive" \
  -exportPath "build/InstantDeploy.ipa" \
  -exportOptionsPlist ExportOptions.plist

# Check file
file build/InstantDeploy.ipa
```

## Distribution Certificate Management

### Renewal (annually)

```bash
# Create new certificate in Apple Developer
# Download in Xcode preferences
# Update in project signing

# Old certificate expires automatically
```

### Provisioning Profile Renewal

```bash
# Regenerate profiles when:
# - Team member added
# - Device added
# - Certificate expires

# Fix in Xcode: Xcode > Preferences > Accounts > Manage Certificates
```

## App Store Optimization (ASO)

### Keywords

Choose high-value keywords:
- "Deploy apps"
- "CI/CD pipeline"
- "Docker deployment"
- "GitHub integration"

### Screenshots

Capture key screenshots:
1. Dashboard overview
2. Deployment creation
3. Real-time logs
4. Success notification

## Post-Release

### Monitor Reviews

- App Store Connect > App Reviews
- Respond to user feedback
- Address low-rating complaints

### Update Plan

Version roadmap:
- v1.1: Additional deployment targets
- v1.2: Enhanced monitoring
- v1.3: Team collaboration features

### Metrics Target

- Target 1,000+ downloads in first month
- Maintain 4.5+ star rating
- < 0.1% crash rate
- < 1% error rate
