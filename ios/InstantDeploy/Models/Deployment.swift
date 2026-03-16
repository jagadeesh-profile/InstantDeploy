import Foundation

struct Deployment: Codable, Identifiable {
    let id: String
    let repository: String
    let branch: String
    let status: String
    let url: String
    let createdAt: String
}
