import { useEffect, useState } from "react";

function ThreadList() {
  const [threads, setThreads] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchThreads = async () => {
    try {
      const response = await fetch("/web/api/threads");
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      setThreads(data.threads || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: fetchThreads should only run on mount
  useEffect(() => {
    fetchThreads();
  }, []);

  const buildSlackUrl = (thread) => {
    const threadTsFormatted = thread.thread_ts.replace(".", "");
    return `https://${thread.workspace_subdomain}.slack.com/archives/${thread.channel_id}/p${threadTsFormatted}`;
  };

  if (loading) {
    return (
      <div className="bg-white shadow rounded-lg p-6">
        <p className="text-gray-500">Loading threads...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-white shadow rounded-lg p-6">
        <p className="text-red-500">Error: {error}</p>
      </div>
    );
  }

  return (
    <div>
      <h2 className="text-xl font-semibold text-gray-900 mb-4">Threads</h2>
      <div className="space-y-4">
        {threads.length === 0 ? (
          <div className="bg-white shadow rounded-lg p-6">
            <p className="text-gray-500">No threads found</p>
          </div>
        ) : (
          threads.map((thread) => (
            <div
              key={thread.thread_ts}
              className="bg-white shadow rounded-lg p-6"
            >
              <div className="flex justify-between items-start">
                <div>
                  <p className="text-sm font-medium text-gray-900">
                    Thread: {thread.thread_ts}
                  </p>
                  <p className="text-sm text-gray-500">
                    Channel: {thread.channel_id}
                  </p>
                  <p className="text-sm text-gray-500">
                    Sessions: {thread.session_count} | Latest:{" "}
                    {thread.latest_session_status}
                  </p>
                </div>
                <a
                  href={buildSlackUrl(thread)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center px-3 py-1.5 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                >
                  Open in Slack
                  <svg
                    className="ml-2 -mr-0.5 h-4 w-4"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    role="img"
                    aria-label="External link arrow"
                  >
                    <path
                      fillRule="evenodd"
                      d="M10.293 3.293a1 1 0 011.414 0l6 6a1 1 0 010 1.414l-6 6a1 1 0 01-1.414-1.414L14.586 11H3a1 1 0 110-2h11.586l-4.293-4.293a1 1 0 010-1.414z"
                      clipRule="evenodd"
                    />
                  </svg>
                </a>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

export default ThreadList;
