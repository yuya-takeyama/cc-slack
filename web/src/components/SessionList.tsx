import { useEffect, useState } from "react";
import { formatDateRange, formatDuration } from "../utils/dateFormatter";
import {
  getSessionStatusColor,
  truncateSessionId,
  truncatePrompt,
} from "../utils/sessionUtils";

interface Session {
  session_id: string;
  thread_ts: string;
  status: "active" | "completed" | "failed" | "unknown";
  started_at: string;
  ended_at?: string;
  initial_prompt?: string;
}

interface SessionsResponse {
  sessions: Session[];
}

function SessionList() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchSessions = async () => {
    try {
      const response = await fetch("/api/sessions");
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: SessionsResponse = await response.json();
      setSessions(data.sessions || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: fetchSessions should only run on mount
  useEffect(() => {
    fetchSessions();
  }, []);

  if (loading) {
    return (
      <div className="bg-white shadow rounded-lg p-6">
        <p className="text-gray-500">Loading sessions...</p>
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
      <h2 className="text-xl font-semibold text-gray-900 mb-4">Sessions</h2>
      <div className="space-y-4">
        {sessions.length === 0 ? (
          <div className="bg-white shadow rounded-lg p-6">
            <p className="text-gray-500">No sessions found</p>
          </div>
        ) : (
          sessions.map((session) => (
            <div
              key={session.session_id}
              className="bg-white shadow rounded-lg p-6"
            >
              <div className="flex justify-between items-start">
                <div className="flex-1">
                  <p className="text-sm font-medium text-gray-900">
                    ID: {truncateSessionId(session.session_id)}
                  </p>
                  <p className="text-sm text-gray-500">
                    Thread: {session.thread_ts}
                  </p>
                  <p className="text-sm text-gray-500">
                    {formatDateRange(session.started_at, session.ended_at, {
                      format: "medium",
                    })}
                  </p>
                  <p className="text-sm text-gray-400">
                    Duration:{" "}
                    {formatDuration(session.started_at, session.ended_at)}
                  </p>
                  {(session.initial_prompt || session.initial_prompt === null) && (
                    <div className="mt-2">
                      <p className="text-sm font-medium text-gray-700">
                        Initial Prompt:
                      </p>
                      {session.initial_prompt ? (
                        <p
                          className="text-sm text-gray-600 mt-1 cursor-help"
                          title={session.initial_prompt}
                        >
                          {truncatePrompt(session.initial_prompt)}
                        </p>
                      ) : (
                        <p className="text-sm text-gray-400 italic mt-1">
                          No prompt available
                        </p>
                      )}
                    </div>
                  )}
                </div>
                <span
                  className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getSessionStatusColor(session.status)}`}
                >
                  {session.status}
                </span>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

export default SessionList;
