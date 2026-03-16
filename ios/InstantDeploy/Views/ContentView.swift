import SwiftUI

struct ContentView: View {
    @StateObject private var viewModel = DashboardViewModel()
    @State private var repository = "octocat/Hello-World"
    @State private var branch = "main"

    var body: some View {
        NavigationView {
            VStack(spacing: 12) {
                HStack {
                    TextField("Repository", text: $repository)
                        .textFieldStyle(.roundedBorder)
                    TextField("Branch", text: $branch)
                        .textFieldStyle(.roundedBorder)
                }

                Button("Deploy") {
                    Task { await viewModel.create(repository: repository, branch: branch) }
                }
                .buttonStyle(.borderedProminent)

                if viewModel.isLoading {
                    ProgressView("Loading deployments...")
                }

                List(viewModel.deployments) { deployment in
                    VStack(alignment: .leading) {
                        Text(deployment.repository).font(.headline)
                        Text("\(deployment.branch) • \(deployment.status)")
                            .font(.subheadline)
                            .foregroundColor(.secondary)
                        Text(deployment.url).font(.footnote).foregroundColor(.blue)
                    }
                }
            }
            .padding()
            .navigationTitle("InstantDeploy")
            .task { await viewModel.loginAndLoad() }
            .alert("Error", isPresented: .constant(viewModel.errorMessage != nil)) {
                Button("OK") { viewModel.errorMessage = nil }
            } message: {
                Text(viewModel.errorMessage ?? "Unknown error")
            }
        }
    }
}

#Preview {
    ContentView()
}
