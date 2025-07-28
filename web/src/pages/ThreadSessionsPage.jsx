import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { formatDateTime } from "../utils/dateFormatter";

function ThreadSessionsPage() {
  const { threadId } = useParams();
  const [thread, setThread] = useState(null);
  const [sessions, setSessions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchThreadSessions = async () => {
      try {
        const response = await fetch(`/api/threads/${threadId}/sessions`);
        if (!response.ok) {
          throw new Error("Failed to fetch thread sessions");
        }
        const data = await response.json();
        setThread(data.thread);
        setSessions(data.sessions);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    fetchThreadSessions();
  }, [threadId]);


  const getStatusColor = (status) => {
    switch (status) {
      case "active":
        return "bg-green-100 text-green-800";
      case "completed":
        return "bg-blue-100 text-blue-800";
      case "failed":
        return "bg-red-100 text-red-800";
      default:
        return "bg-gray-100 text-gray-800";
    }
  };

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
            href={`https://${thread.workspace_subdomain}.slack.com/archives/${thread.channel_id}/p${thread.thread_ts.replace(".", "")}`}
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
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`inline-flex px-2 text-xs leading-5 font-semibold rounded-full ${getStatusColor(
                        session.status,
                      )}`}
                    >
                      {session.status}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDateTime(session.started_at, { format: "medium", relative: true })}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                    {formatDateTime(session.ended_at, { format: "medium", relative: true, fallback: "-" })}
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
