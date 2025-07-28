import { useEffect, useState, useCallback, useRef } from "react";

interface Status {
  running: boolean;
  pid?: number;
  uptime?: string;
  started?: string;
}

interface RestartResponse {
  success: boolean;
  error?: string;
  pid?: number;
  duration?: string;
}

function ManagerPage() {
  const [status, setStatus] = useState<Status | null>(null);
  const [loading, setLoading] = useState(true);
  const [restarting, setRestarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastRestartTime, setLastRestartTime] = useState<Date | null>(null);
  const pollIntervalRef = useRef<number | null>(null);
  const previousPidRef = useRef<number | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const response = await fetch("/api/manager/status");
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: Status = await response.json();
      setStatus(data);
      setError(null);
      return data;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to fetch status");
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  const pollStatusUntilChanged = useCallback(async () => {
    const startTime = Date.now();
    const maxPollingTime = 30000; // 30 seconds max
    const pollInterval = 500; // 500ms between polls

    return new Promise<boolean>((resolve) => {
      const poll = async () => {
        const currentStatus = await fetchStatus();
        
        if (currentStatus && currentStatus.pid && currentStatus.pid !== previousPidRef.current) {
          // PID changed, restart complete
          resolve(true);
          return;
        }

        if (Date.now() - startTime > maxPollingTime) {
          // Timeout
          resolve(false);
          return;
        }

        // Continue polling
        pollIntervalRef.current = window.setTimeout(poll, pollInterval);
      };

      poll();
    });
  }, [fetchStatus]);

  const handleRestart = async () => {
    setRestarting(true);
    setError(null);

    try {
      // Store current PID before restart
      if (status?.pid) {
        previousPidRef.current = status.pid;
      }

      const response = await fetch("/api/manager/restart", {
        method: "POST",
      });

      const data: RestartResponse = await response.json();

      if (!response.ok || !data.success) {
        throw new Error(data.error || "Restart failed");
      }

      setLastRestartTime(new Date());
      
      // Poll for status change
      const restartDetected = await pollStatusUntilChanged();
      
      if (restartDetected) {
        // Force reload the browser
        window.location.reload();
      } else {
        setError("Restart may have failed - timeout waiting for PID change");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to restart");
    } finally {
      setRestarting(false);
    }
  };

  // Initial fetch
  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  // Auto-refresh status every 5 seconds
  useEffect(() => {
    const interval = setInterval(fetchStatus, 5000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollIntervalRef.current) {
        clearTimeout(pollIntervalRef.current);
      }
    };
  }, []);

  if (loading) {
    return (
      <div className="flex justify-center items-center h-64">
        <div className="text-gray-500">Loading...</div>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-2xl font-bold mb-6">cc-slack Manager</h2>

      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-6">
          {error}
        </div>
      )}

      <div className="bg-white shadow rounded-lg p-6 mb-6">
        <h3 className="text-lg font-semibold mb-4">Status</h3>
        
        {status ? (
          <div className="space-y-2">
            <div className="flex items-center">
              <span className="font-medium w-24">Status:</span>
              <span className={`px-2 py-1 rounded text-sm ${
                status.running 
                  ? "bg-green-100 text-green-800" 
                  : "bg-red-100 text-red-800"
              }`}>
                {status.running ? "Running" : "Stopped"}
              </span>
            </div>
            
            {status.running && status.pid && (
              <>
                <div className="flex items-center">
                  <span className="font-medium w-24">PID:</span>
                  <span className="text-gray-700">{status.pid}</span>
                </div>
                
                {status.uptime && (
                  <div className="flex items-center">
                    <span className="font-medium w-24">Uptime:</span>
                    <span className="text-gray-700">{status.uptime}</span>
                  </div>
                )}
                
                {status.started && (
                  <div className="flex items-center">
                    <span className="font-medium w-24">Started:</span>
                    <span className="text-gray-700">
                      {new Date(status.started).toLocaleString()}
                    </span>
                  </div>
                )}
              </>
            )}
          </div>
        ) : (
          <div className="text-gray-500">Unable to fetch status</div>
        )}
      </div>

      <div className="bg-white shadow rounded-lg p-6">
        <h3 className="text-lg font-semibold mb-4">Actions</h3>
        
        <div className="space-y-4">
          <button
            onClick={handleRestart}
            disabled={restarting || !status?.running}
            className={`px-6 py-2 rounded font-medium transition-colors ${
              restarting || !status?.running
                ? "bg-gray-300 text-gray-500 cursor-not-allowed"
                : "bg-blue-600 text-white hover:bg-blue-700"
            }`}
          >
            {restarting ? "Restarting..." : "Restart cc-slack"}
          </button>
          
          {lastRestartTime && (
            <div className="text-sm text-gray-600">
              Last restart: {lastRestartTime.toLocaleTimeString()}
            </div>
          )}
          
          {!status?.running && (
            <div className="text-sm text-amber-600">
              cc-slack is not running. Start it manually first.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default ManagerPage;