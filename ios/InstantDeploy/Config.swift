// InstantDeploy iOS Configuration
// Configuration management for production builds

import Foundation

struct AppConfig {
    // API Configuration
    static let apiBaseURL: URL = {
        #if DEBUG
            return URL(string: "http://localhost:8080")!
        #else
            return URL(string: "https://api.instantdeploy.example.com")!
        #endif
    }()
    
    static let wsBaseURL: URL = {
        #if DEBUG
            return URL(string: "ws://localhost:8080")!
        #else
            return URL(string: "wss://api.instantdeploy.example.com")!
        #endif
    }()
    
    // App Information
    static let appName = "InstantDeploy"
    static let bundleIdentifier = "com.instantdeploy.app"
    static let appVersion = "1.0"
    static let buildNumber = "1"
    
    // Feature Flags
    static let enableCrashReporting = true
    static let enableAnalytics = true
    static let enableLogging = !isProduction
    
    // Timeout configuration
    static let networkTimeout: TimeInterval = 30
    static let wsReconnectInterval: TimeInterval = 5
    
    // Cache configuration
    static let cacheExpirationInterval: TimeInterval = 3600 // 1 hour
    
    // Is production build
    static var isProduction: Bool {
        #if DEBUG
            return false
        #else
            return true
        #endif
    }
}
