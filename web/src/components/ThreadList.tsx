import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { truncatePrompt } from "../utils/sessionUtils";
import { buildSlackThreadUrl } from "../utils/slackUtils";

interface Thread {
  thread_ts: string;
  thread_time?: string;
  channel_id: string;
  channel_name?: string;
  workspace_subdomain?: string;
  session_count: number;
  latest_session_status: string;
  initial_prompt?: string;
}

interface ThreadsResponse {
  threads: Thread[];
  has_more: boolean;
  page: number;
}

function ThreadList() {
  const [threads, setThreads] = useState<Thread[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const currentPage = parseInt(searchParams.get("page") || "1", 10);

  const fetchThreads = async (page: number) => {
    try {
      setLoading(true);
      const response = await fetch(`/api/threads?page=${page}`);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: ThreadsResponse = await response.json();
      setThreads(data.threads || []);
      setHasMore(data.has_more || false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: fetchThreads depends on currentPage
  useEffect(() => {
    fetchThreads(currentPage);
  }, [currentPage]);

  const goToPage = (page: number) => {
    if (page === 1) {
      searchParams.delete("page");
    } else {
      searchParams.set("page", page.toString());
    }
    setSearchParams(searchParams);
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

  const PaginationControls = () => (
    <div className="flex justify-between items-center py-3">
      <button
        type="button"
        onClick={() => goToPage(currentPage - 1)}
        disabled={currentPage === 1}
        className={`px-4 py-2 text-sm font-medium rounded-md ${
          currentPage === 1
            ? "bg-gray-100 text-gray-400 cursor-not-allowed"
            : "bg-white text-gray-700 hover:bg-gray-50 border border-gray-300"
        }`}
      >
        ← Previous
      </button>
      <span className="text-sm text-gray-700">Page {currentPage}</span>
      <button
        type="button"
        onClick={() => goToPage(currentPage + 1)}
        disabled={!hasMore}
        className={`px-4 py-2 text-sm font-medium rounded-md ${
          !hasMore
            ? "bg-gray-100 text-gray-400 cursor-not-allowed"
            : "bg-white text-gray-700 hover:bg-gray-50 border border-gray-300"
        }`}
      >
        Next →
      </button>
    </div>
  );

  return (
    <div>
      <h2 className="text-xl font-semibold text-gray-900 mb-4">Threads</h2>
      {threads.length > 0 && <PaginationControls />}
      <div className="space-y-2">
        {threads.length === 0 ? (
          <div className="bg-white shadow rounded-lg p-6">
            <p className="text-gray-500">No threads found</p>
          </div>
        ) : (
          threads.map((thread) => (
            <div
              key={thread.thread_ts}
              className="bg-white shadow rounded-lg p-4"
            >
              <div className="flex justify-between items-start mb-2">
                <div className="flex-1 min-w-0">
                  <div className="flex items-baseline gap-2 flex-wrap">
                    <span className="text-sm font-semibold text-gray-900">
                      {thread.thread_time || thread.thread_ts}
                    </span>
                    <span className="text-xs text-gray-500">
                      #{thread.channel_name || thread.channel_id} •{" "}
                      {thread.session_count} session
                      {thread.session_count !== 1 ? "s" : ""} •{" "}
                      {thread.latest_session_status}
                    </span>
                  </div>
                </div>
                <div className="flex gap-2 ml-4 flex-shrink-0">
                  <Link
                    to={`/threads/${thread.thread_ts}/sessions`}
                    className="inline-flex items-center px-2.5 py-1 border border-blue-300 shadow-sm text-xs font-medium rounded-md text-blue-700 bg-white hover:bg-blue-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                  >
                    View Sessions
                  </Link>
                  <a
                    href={buildSlackThreadUrl(thread) || "#"}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center px-2.5 py-1 border border-gray-300 shadow-sm text-xs font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
                  >
                    Open in Slack
                    <svg
                      className="ml-1.5 -mr-0.5 h-3 w-3"
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
              {thread.initial_prompt && (
                <div className="text-xs text-gray-600 leading-relaxed">
                  {truncatePrompt(thread.initial_prompt, 200)}
                </div>
              )}
            </div>
          ))
        )}
      </div>
      {threads.length > 0 && <PaginationControls />}
    </div>
  );
}

export default ThreadList;
