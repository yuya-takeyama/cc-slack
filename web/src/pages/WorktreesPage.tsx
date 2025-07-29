import { useEffect, useState } from "react";

interface Worktree {
  id: number;
  repository_id: number;
  thread_id: number;
  path: string;
  base_branch: string;
  current_branch?: string;
  status: string;
  created_at: string;
  deleted_at?: string;
}

interface WorktreesResponse {
  worktrees: Worktree[];
}

export function WorktreesPage() {
  const [worktrees, setWorktrees] = useState<Worktree[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showAll, setShowAll] = useState(false);

  useEffect(() => {
    const fetchWorktrees = async () => {
      try {
        const url = showAll ? "/api/worktrees" : "/api/worktrees/active";
        const response = await fetch(url);
        if (!response.ok) {
          throw new Error(`Failed to fetch worktrees: ${response.statusText}`);
        }
        const data: WorktreesResponse = await response.json();
        setWorktrees(data.worktrees);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unknown error");
      } finally {
        setLoading(false);
      }
    };

    fetchWorktrees();
  }, [showAll]);

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="text-gray-500">Loading worktrees...</div>
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

  const getStatusColor = (status: string) => {
    switch (status) {
      case "active":
        return "bg-green-100 text-green-800";
      case "archived":
        return "bg-yellow-100 text-yellow-800";
      case "deleted":
        return "bg-gray-100 text-gray-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-2xl font-semibold text-gray-800">Worktrees</h2>
        <label className="inline-flex items-center">
          <input
            type="checkbox"
            className="form-checkbox"
            checked={showAll}
            onChange={(e) => setShowAll(e.target.checked)}
          />
          <span className="ml-2">Show all (including deleted)</span>
        </label>
      </div>
      
      {worktrees.length === 0 ? (
        <div className="bg-gray-50 border border-gray-200 rounded-lg p-8 text-center">
          <p className="text-gray-500">No worktrees found.</p>
        </div>
      ) : (
        <div className="bg-white shadow overflow-hidden sm:rounded-md">
          <ul className="divide-y divide-gray-200">
            {worktrees.map((worktree) => (
              <li key={worktree.id} className="px-6 py-4 hover:bg-gray-50">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center">
                      <h3 className="text-lg font-medium text-gray-900 font-mono">
                        {worktree.path}
                      </h3>
                      <span className={`ml-2 px-2 py-1 text-xs rounded ${getStatusColor(worktree.status)}`}>
                        {worktree.status}
                      </span>
                    </div>
                    <div className="mt-1 text-sm text-gray-600">
                      <p>
                        Base branch: <span className="font-mono">{worktree.base_branch}</span>
                        {worktree.current_branch && worktree.current_branch !== worktree.base_branch && (
                          <> → <span className="font-mono">{worktree.current_branch}</span></>
                        )}
                      </p>
                      <p>Thread ID: {worktree.thread_id} • Repository ID: {worktree.repository_id}</p>
                    </div>
                  </div>
                  <div className="ml-4 text-sm text-gray-500 text-right">
                    <p>ID: {worktree.id}</p>
                    <p>Created: {new Date(worktree.created_at).toLocaleString()}</p>
                    {worktree.deleted_at && (
                      <p>Deleted: {new Date(worktree.deleted_at).toLocaleString()}</p>
                    )}
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