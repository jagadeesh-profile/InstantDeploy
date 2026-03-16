import Foundation

final class APIService {
    static let shared = APIService()

    private let baseURL = URL(string: "http://localhost:8080/api/v1")!
    private var token: String = ""

    private init() {}

    func login(username: String, password: String) async throws {
        let endpoint = baseURL.appendingPathComponent("auth/login")
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(["username": username, "password": password])

        let (data, _) = try await URLSession.shared.data(for: request)
        let result = try JSONDecoder().decode(LoginResponse.self, from: data)
        token = result.token
    }

    func fetchDeployments() async throws -> [Deployment] {
        let endpoint = baseURL.appendingPathComponent("deployments")
        var request = URLRequest(url: endpoint)
        request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let (data, _) = try await URLSession.shared.data(for: request)
        let result = try JSONDecoder().decode(DeploymentListResponse.self, from: data)
        return result.items
    }

    func createDeployment(repository: String, branch: String) async throws -> Deployment {
        let endpoint = baseURL.appendingPathComponent("deployments")
        var request = URLRequest(url: endpoint)
        request.httpMethod = "POST"
        request.addValue("application/json", forHTTPHeaderField: "Content-Type")
        request.addValue("Bearer \(token)", forHTTPHeaderField: "Authorization")

        let payload = ["repository": repository, "branch": branch]
        request.httpBody = try JSONEncoder().encode(payload)

        let (data, _) = try await URLSession.shared.data(for: request)
        return try JSONDecoder().decode(Deployment.self, from: data)
    }
}

private struct LoginResponse: Codable {
    let token: String
}

private struct DeploymentListResponse: Codable {
    let items: [Deployment]
}
