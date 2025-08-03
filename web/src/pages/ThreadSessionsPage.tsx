import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { formatDateTime } from "../utils/dateFormatter";
import { getSessionStatusColor, truncatePrompt } from "../utils/sessionUtils";
import { buildSlackThreadUrl } from "../utils/slackUtils";

interface Thread {
  thread_ts: string;
  channel_id: string;
  workspace_subdomain?: string;
}

interface Session {
  session_id: string;
  status: "active" | "completed" | "failed" | "unknown";
  started_at: string;
  ended_at?: string;
  initial_prompt?: string;
}

interface ThreadSessionsResponse {
  thread: Thread;
  sessions: Session[];
}

function ThreadSessionsPage() {
  const { threadId } = useParams<{ threadId: string }>();
  const [thread, setThread] = useState<Thread | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchThreadSessions = async () => {
    try {
      setLoading(true);
      const response = await fetch(`/api/threads/${threadId}/sessions`);
      if (!response.ok) {
        throw new Error("Failed to fetch thread sessions");
      }
      const data: ThreadSessionsResponse = await response.json();
      setThread(data.thread);
      setSessions(data.sessions);
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: fetchThreadSessions is not memoized
  useEffect(() => {
    fetchThreadSessions();
  }, [threadId]);

  if (loading) {
    return <div className="text-center py-8">Loading...</div>;
  }

  if (error) {
    return <div className="text-center py-8 text-red-600">Error: {error}</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold text-gray-800">
          Thread Sessions
        </h2>
        <Link to="/" className="text-blue-600 hover:text-blue-800 text-sm">
          ← Back to Threads
        </Link>
      </div>

      {thread && (
        <div className="bg-white rounded-lg shadow p-4 mb-6">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-sm text-gray-600">Thread</p>
              <p className="font-medium">{thread.thread_ts}</p>
            </div>
            <div>
              <p className="text-sm text-gray-600">Channel</p>
              <p className="font-medium">{thread.channel_id}</p>
            </div>
          </div>
          <a
            href={buildSlackThreadUrl(thread) || "#"}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-blue-600 hover:text-blue-800 text-sm mt-3"
          >
            Open in Slack ↗
          </a>
        </div>
      )}

      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b border-gray-200">
          <h3 className="text-lg font-semibold text-gray-800">
            Sessions ({sessions.length})
          </h3>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Session ID
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Initial Prompt
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Started At
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Ended At
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {sessions.map((session) => (
                <tr key={session.session_id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                    {session.session_id}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {session.initial_prompt ? (
                      <span
                        className="cursor-help block max-w-xs truncate"
                        title={session.initial_prompt}
                      >
                        {truncatePrompt(session.initial_prompt, 50)}
                      </span>
                    ) : (
                      <span className="italic text-gray-400">No prompt</span>
                    )}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex px-2 text-xs leading-5 font-semibold rounded-full ${getSessionStatusColor(
                        session.status,
                        "table",
                      )}`}
                    >
                      {session.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDateTime(session.started_at, {
                      format: "medium",
                      relative: true,
                    })}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDateTime(session.ended_at, {
                      format: "medium",
                      relative: true,
                      fallback: "-",
                    })}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {sessions.length === 0 && (
            <div className="text-center py-8 text-gray-500">
              No sessions found for this thread.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default ThreadSessionsPage;
