import { useEffect, useState } from "react";

interface Repository {
  id: number;
  name: string;
  path: string;
  default_branch: string;
  channel_id?: string;
  username?: string;
  icon_emoji?: string;
  icon_url?: string;
  created_at: string;
  updated_at: string;
}

interface RepositoriesResponse {
  repositories: Repository[];
}

export function RepositoriesPage() {
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchRepositories = async () => {
      try {
        const response = await fetch("/api/repositories");
        if (!response.ok) {
          throw new Error(`Failed to fetch repositories: ${response.statusText}`);
        }
        const data: RepositoriesResponse = await response.json();
        setRepositories(data.repositories);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unknown error");
      } finally {
        setLoading(false);
      }
    };

    fetchRepositories();
  }, []);

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="text-gray-500">Loading repositories...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
        Error: {error}
      </div>
    );
  }

  return (
    <div>
      <h2 className="text-2xl font-semibold text-gray-800 mb-4">Repositories</h2>
      
      {repositories.length === 0 ? (
        <div className="bg-gray-50 border border-gray-200 rounded-lg p-8 text-center">
          <p className="text-gray-500">No repositories configured yet.</p>
        </div>
      ) : (
        <div className="bg-white shadow overflow-hidden sm:rounded-md">
          <ul className="divide-y divide-gray-200">
            {repositories.map((repo) => (
              <li key={repo.id} className="px-6 py-4 hover:bg-gray-50">
                <div className="flex items-center justify-between">
                  <div className="flex-1">
                    <div className="flex items-center">
                      <h3 className="text-lg font-medium text-gray-900">
                        {repo.name}
                      </h3>
                      {repo.default_branch && (
                        <span className="ml-2 px-2 py-1 text-xs bg-gray-100 text-gray-600 rounded">
                          {repo.default_branch}
                        </span>
                      )}
                    </div>
                    <p className="mt-1 text-sm text-gray-600 font-mono">
                      {repo.path}
                    </p>
                    {repo.channel_id && (
                      <p className="mt-1 text-sm text-gray-500">
                        Channel: #{repo.channel_id}
                        {repo.username && ` • @${repo.username}`}
                        {repo.icon_emoji && ` ${repo.icon_emoji}`}
                      </p>
                    )}
                  </div>
                  <div className="ml-4 text-sm text-gray-500">
                    <p>ID: {repo.id}</p>
                    <p>Updated: {new Date(repo.updated_at).toLocaleDateString()}</p>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}