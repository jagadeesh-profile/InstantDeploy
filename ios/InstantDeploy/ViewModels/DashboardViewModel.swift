import Foundation

@MainActor
final class DashboardViewModel: ObservableObject {
    @Published var deployments: [Deployment] = []
    @Published var isLoading = false
    @Published var errorMessage: String?

    func loginAndLoad() async {
        do {
            try await APIService.shared.login(username: "demo", password: "demo123")
            try await refresh()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func refresh() async throws {
        isLoading = true
        defer { isLoading = false }
        deployments = try await APIService.shared.fetchDeployments()
    }

    func create(repository: String, branch: String) async {
        do {
            let created = try await APIService.shared.createDeployment(repository: repository, branch: branch)
            deployments.insert(created, at: 0)
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
